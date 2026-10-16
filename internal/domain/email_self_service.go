package domain

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// Self-service email identities (#30, iam-scope "登录身份"): a user without an email identity
// adds one, a user with one changes the login email. In both the user proves they own the new
// address with a six-digit code that arrives by mail, and which is a one-time token (#21) of its
// own purpose: the token row stores the SHA-256 of (purpose, user, address, code), so a code
// works only for the address it was sent to, only for the user who asked, and only once.
//
// The code is short, so guessing is limited: a user who has typed too many wrong codes is
// refused (rateLimited) for a while, and sending codes is limited per user and per client IP.

const (
	TokenBindEmail   TokenPurpose = "bindEmail"   // the code that proves a new email identity's address
	TokenChangeEmail TokenPurpose = "changeEmail" // the code that proves the new login email

	emailCodeTTL = 15 * time.Minute

	// Codes sent count towards a limit when the request was accepted (well-formed).
	emailCodeWindow       = time.Hour
	emailCodeLimitPerUser = 5
	emailCodeLimitPerIP   = 20

	// Wrong codes typed, per user.
	emailCodeFailWindow = 15 * time.Minute
	emailCodeFailLimit  = 5
)

func codeInvalid() error { return Validation(Invalid("code", CodeTokenInvalid, nil)) }

// emailCodeHash is what the token row stores for a code sent to address.
func emailCodeHash(purpose TokenPurpose, userID UserID, address, code string) string {
	return hashToken(fmt.Sprintf("%s:%d:%s:%s", purpose, userID, address, code))
}

func newEmailCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// requireMail refuses with mailNotConfigured (503) while mail does not work.
func (s *Service) requireMail(ctx context.Context) error {
	ok, err := s.MailConfigured(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return newKind(KindUnavailable, CodeMailNotConfigured)
	}
	return nil
}

// limitEmailCodes counts a code request against the user's and the IP's limits.
func (s *Service) limitEmailCodes(ctx context.Context, userID UserID, clientIP string) error {
	now := s.nowUTC()
	buckets := []struct {
		key   string
		limit int
	}{
		{fmt.Sprintf("emailcode:user:%d", userID), emailCodeLimitPerUser},
		{"emailcode:ip:" + clientIP, emailCodeLimitPerIP},
	}
	err := s.repo.InTx(ctx, func(tx Repository) error {
		if err := tx.LockRateKeys(ctx, buckets[0].key, buckets[1].key); err != nil {
			return err
		}
		for _, b := range buckets {
			n, err := tx.CountRateEvents(ctx, b.key, now.Add(-emailCodeWindow))
			if err != nil {
				return err
			}
			if n >= b.limit {
				return rateLimitedError()
			}
		}
		for _, b := range buckets {
			if err := tx.AddRateEvent(ctx, b.key, now); err != nil {
				return err
			}
		}
		return nil
	})
	s.pruneRateEvents(ctx, now.Add(-emailCodeWindow))
	return err
}

// RequestBindEmailCode mails a verification code to the address a user without an email
// identity wants to log in with. clientIP is the address of the TCP peer.
func (s *Service) RequestBindEmailCode(ctx context.Context, actor Actor, email, clientIP string) error {
	return s.requestEmailCode(ctx, actor, TokenBindEmail, email, clientIP)
}

func (s *Service) requestEmailCode(ctx context.Context, actor Actor, purpose TokenPurpose, email, clientIP string) error {
	if err := s.requireMail(ctx); err != nil {
		return err
	}
	email = normalizeLoginEmail(email)
	var c Collector
	checkLoginEmail(&c, "email", email)
	if err := c.Err(); err != nil {
		return err
	}
	if err := s.limitEmailCodes(ctx, actor.User.ID, clientIP); err != nil {
		return err
	}
	return s.repo.InTx(ctx, func(tx Repository) error {
		cur, found, err := tx.LockUserWithIdentities(ctx, actor.User.ID)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		if err := s.checkEmailTarget(ctx, tx, cur, purpose, email); err != nil {
			return err
		}
		code, err := newEmailCode()
		if err != nil {
			return err
		}
		now := s.nowUTC()
		// Asking again replaces the earlier code.
		if err := tx.UseUserAuthTokens(ctx, cur.User.ID, []TokenPurpose{purpose}, now); err != nil {
			return err
		}
		if err := tx.InsertAuthToken(ctx, AuthToken{
			TokenHash: emailCodeHash(purpose, cur.User.ID, email, code), UserID: cur.User.ID, Purpose: purpose,
			CreatedAt: now, ExpiresAt: now.Add(emailCodeTTL),
		}); err != nil {
			return err
		}
		_, err = s.enqueueMail(ctx, tx, MailRequest{
			Template: MailTemplateVerifyCode, To: email, Recipient: cur.User.ID, Data: map[string]string{"Code": code},
		})
		return err
	})
}

// checkEmailTarget checks that the user may be given this login email by this purpose.
func (s *Service) checkEmailTarget(ctx context.Context, tx Repository, cur UserWithIdentities, purpose TokenPurpose, email string) error {
	switch ident, has := cur.identity(IdentityEmail); {
	case purpose == TokenBindEmail && has:
		return Validation(Invalid("kind", CodeIdentityKindBound, nil))
	case purpose == TokenChangeEmail && !has:
		return Validation(Invalid("email", CodeNoEmailIdentity, nil))
	case purpose == TokenChangeEmail && ident.LoginName == email:
		return Validation(Invalid("email", CodeLoginEmailUnchanged, nil))
	}
	if _, held, err := tx.FindIdentity(ctx, IdentityEmail, email); err != nil {
		return err
	} else if held {
		return loginEmailTaken("email")
	}
	return nil
}

// verifyEmailCode spends the code within tx, or says why it does not work.
func (s *Service) verifyEmailCode(ctx context.Context, tx Repository, userID UserID, purpose TokenPurpose, email, code string) error {
	t, found, err := tx.LockAuthToken(ctx, emailCodeHash(purpose, userID, email, code))
	if err != nil {
		return err
	}
	if !found || t.UserID != userID || t.Purpose != purpose {
		return codeInvalid()
	}
	now := s.nowUTC()
	if !t.UsedAt.IsZero() {
		return Validation(Invalid("code", CodeTokenUsed, nil))
	}
	if !now.Before(t.ExpiresAt) {
		return Validation(Invalid("code", CodeTokenExpired, nil))
	}
	return tx.UseUserAuthTokens(ctx, userID, []TokenPurpose{purpose}, now)
}

func (s *Service) failBucket(userID UserID) string {
	return fmt.Sprintf("emailcode-fail:user:%d", userID)
}

// commitThenFail is returned from a transaction body to commit what the body wrote and then
// fail with err: a wrong guess has to be on record even though the request is refused.
type commitThenFail struct{ err error }

func (e commitThenFail) Error() string { return e.err.Error() }
func (e commitThenFail) Unwrap() error { return e.err }

// inTxKeeping runs fn like InTx, except that a commitThenFail commits and then comes out as its
// error.
func (s *Service) inTxKeeping(ctx context.Context, fn func(tx Repository) error) error {
	var after error
	err := s.repo.InTx(ctx, func(tx Repository) error {
		err := fn(tx)
		var keep commitThenFail
		if errors.As(err, &keep) {
			after = keep.err
			return nil
		}
		return err
	})
	if err != nil {
		return err
	}
	return after
}

func isWrongCode(err error) bool {
	e, ok := AsError(err)
	return ok && e.Kind == KindValidation && len(e.Errors) == 1 && e.Errors[0].Field == "code" && e.Errors[0].Code == CodeTokenInvalid
}

// verifyGuessedCode is verifyEmailCode behind the guess limit. The limit is read and a wrong code
// is recorded inside the caller's transaction (which must hold the user's row lock and must not
// have written anything yet), so parallel guesses are counted one by one: a user who typed too
// many wrong codes lately is refused, even with the right code, and the refusal that records a
// wrong code still commits (inTxKeeping).
func (s *Service) verifyGuessedCode(ctx context.Context, tx Repository, userID UserID, purpose TokenPurpose, email, code string) error {
	bucket := s.failBucket(userID)
	n, err := tx.CountRateEvents(ctx, bucket, s.nowUTC().Add(-emailCodeFailWindow))
	if err != nil {
		return err
	}
	if n >= emailCodeFailLimit {
		return rateLimitedError()
	}
	err = s.verifyEmailCode(ctx, tx, userID, purpose, email, code)
	if isWrongCode(err) {
		if aerr := tx.AddRateEvent(ctx, bucket, s.nowUTC()); aerr != nil {
			return aerr
		}
		return commitThenFail{err}
	}
	return err
}

// BindEmailInput is the confirmation of a new email identity: the address, the code mailed to
// it and the password to log in with.
type BindEmailInput struct {
	Email    string
	Code     string
	Password string
}

// BindOwnEmailIdentity gives the actor an email identity after they proved the address with
// the code. The password they choose is final: no forced change.
func (s *Service) BindOwnEmailIdentity(ctx context.Context, actor Actor, in BindEmailInput) (UserWithIdentities, error) {
	if err := s.requireMail(ctx); err != nil {
		return UserWithIdentities{}, err
	}
	email := normalizeLoginEmail(in.Email)
	var c Collector
	checkLoginEmail(&c, "email", email)
	c.Required("code", in.Code)
	checkNewPassword(&c, in.Password)
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		return UserWithIdentities{}, err
	}
	u, err := s.bindIdentity(ctx, actor.User.ID, bindSpec{
		kind: IdentityEmail, loginName: email, hash: hash, via: BoundViaSelf, by: AuditActorOf(actor),
		before: func(tx Repository) error {
			return s.verifyGuessedCode(ctx, tx, actor.User.ID, TokenBindEmail, email, in.Code)
		},
	})
	return u, err
}

// RequestChangeLoginEmailCode mails a verification code to the address a user with an email
// identity wants as their new login email.
func (s *Service) RequestChangeLoginEmailCode(ctx context.Context, actor Actor, email, clientIP string) error {
	return s.requestEmailCode(ctx, actor, TokenChangeEmail, email, clientIP)
}

// ChangeOwnLoginEmail replaces the login email of the actor's email identity once the code sent
// to the new address is confirmed. The contact email follows it unless the directory owns that;
// the old address (and the contact email, when it is another one) is told.
func (s *Service) ChangeOwnLoginEmail(ctx context.Context, actor Actor, email, code string) (UserWithIdentities, error) {
	if err := s.requireMail(ctx); err != nil {
		return UserWithIdentities{}, err
	}
	email = normalizeLoginEmail(email)
	var c Collector
	checkLoginEmail(&c, "email", email)
	c.Required("code", code)
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}
	var result UserWithIdentities
	err := s.inTxKeeping(ctx, func(tx Repository) error {
		cur, found, err := tx.LockUserWithIdentities(ctx, actor.User.ID)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		if err := s.checkEmailTarget(ctx, tx, cur, TokenChangeEmail, email); err != nil {
			return err
		}
		if err := s.verifyGuessedCode(ctx, tx, cur.User.ID, TokenChangeEmail, email, code); err != nil {
			return err
		}
		old, _ := cur.identity(IdentityEmail)
		contactBefore := cur.User.Email
		if err := tx.RenameEmailLogin(ctx, cur.User.ID, email); errors.Is(err, ErrIdentityTaken) {
			return loginEmailTaken("email")
		} else if err != nil {
			return err
		}
		if !cur.hasIdentity(IdentityLDAP) {
			cur.User.Email = email
			if err := tx.UpdateUser(ctx, cur.User, s.nowUTC()); err != nil {
				return mapUsernameTaken(err)
			}
		}
		// A link mailed to the old address must not work any more.
		if err := tx.UseUserAuthTokens(ctx, cur.User.ID, passwordTokenPurposes, s.nowUTC()); err != nil {
			return err
		}
		for i := range cur.Identities {
			if cur.Identities[i].Kind == IdentityEmail {
				cur.Identities[i].LoginName = email
			}
		}
		if err := s.recordAudit(ctx, tx, AuditActorOf(actor), loginEmailChangedAudit(cur.User.ID, old.LoginName, email)); err != nil {
			return err
		}
		data := map[string]string{"OldEmail": old.LoginName, "NewEmail": email}
		if err := s.notifySecurity(ctx, tx, cur.User.ID, old.LoginName, MailTemplateLoginEmailChanged, data); err != nil {
			return err
		}
		if contactBefore != old.LoginName {
			if err := s.notifySecurity(ctx, tx, cur.User.ID, contactBefore, MailTemplateLoginEmailChanged, data); err != nil {
				return err
			}
		}
		result = cur
		return nil
	})
	return result, err
}

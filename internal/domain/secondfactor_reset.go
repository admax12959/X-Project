package domain

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
)

// Resetting a second factor (#23, ADR-0006): the way back in for a user who lost the
// authenticator and the recovery codes. An administrator does it from user management; the
// command line does it for an administrator who is locked out (cli.go).

// resetSecondFactor removes the user's second factor and everything attached to it (pending
// authenticators, recovery codes, the failed-attempt counter), records who did it and tells
// the contact email when mail works. Call inside InTx. The user's open sessions stay: they were
// opened with a second factor; the user is not logged out by the loss of a device.
func (s *Service) resetSecondFactor(ctx context.Context, tx Repository, by AuditActor, u User) error {
	if err := tx.DeleteAuthenticatorsAndRecoveryCodes(ctx, u.ID); err != nil {
		return err
	}
	if err := tx.PutSecondFactorLockout(ctx, u.ID, SecondFactorLockout{}); err != nil {
		return err
	}
	if err := s.recordAudit(ctx, tx, by, secondFactorResetAudit(u.ID)); err != nil {
		return err
	}
	return s.notifySecurity(ctx, tx, u.ID, u.Email, MailTemplateSecondFactorReset, nil)
}

// ResetSecondFactor removes a user's second factor. Administrators only (user.manage). The user
// is told by mail when mail works and the reset is audited with the administrator as operator.
// A user who is required to have a second factor sets it up again at their next request.
//
// Errors: 403 forbidden, 404 for an unknown user or one without a second factor.
func (s *Service) ResetSecondFactor(ctx context.Context, actor Actor, id UserID) (UserWithIdentities, error) {
	if err := requirePermission(actor, PermUserManage); err != nil {
		return UserWithIdentities{}, err
	}
	var result UserWithIdentities
	err := s.repo.InTx(ctx, func(tx Repository) error {
		cur, found, err := tx.LockUserWithIdentities(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		if err := requireEnabledSecondFactor(ctx, tx, id); err != nil {
			return err
		}
		if err := s.resetSecondFactor(ctx, tx, AuditActorOf(actor), cur.User); err != nil {
			return err
		}
		cur.SecondFactorEnabled = false
		result = cur
		return nil
	})
	return result, err
}

// ---------------------------------------------------------------- the command line

// CLIResetInput is a request of the command line: which user (username or login email) and
// what to reset.
type CLIResetInput struct {
	User         string
	SecondFactor bool
	Password     bool
}

// CLIResetResult says what the command line did. Password is the new temporary password, set
// only when one was made; it goes to the operator's terminal and nowhere else.
type CLIResetResult struct {
	User              string // the username
	SecondFactorReset bool   // false: the user had none
	Password          string
}

// Errors of the command line, shown to the operator.
var (
	ErrCLIUserNotFound = errors.New("no such user")
	ErrCLINoPassword   = errors.New("the user has no email login, so there is no password to reset (the directory owns it)")
	ErrCLINothingAsked = errors.New("nothing to do: ask for --second-factor, --password or both")
)

// cliPasswordLength is the length of a generated temporary password: 24 characters of an
// alphabet without look-alikes is about 120 bits.
const cliPasswordLength = 24

const cliPasswordAlphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newTemporaryPassword() (string, error) {
	out := make([]byte, cliPasswordLength)
	max := big.NewInt(int64(len(cliPasswordAlphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = cliPasswordAlphabet[n.Int64()]
	}
	return string(out), nil
}

// ResetUserByCLI is the container's emergency entry for an administrator who is locked out
// (ADR-0006). There is no authentication: whoever can run the command in the container can
// reach the database anyway. Both resets are audited with the command line as the operator.
//
// A second-factor reset works as the administrator's (the user is told by mail when mail
// works). A password reset gives the user a random temporary password that the user must change
// at the next login, spends the user's invitation and recovery tokens and ends every session.
func (s *Service) ResetUserByCLI(ctx context.Context, in CLIResetInput) (CLIResetResult, error) {
	if !in.SecondFactor && !in.Password {
		return CLIResetResult{}, ErrCLINothingAsked
	}
	var temp, hash string
	if in.Password {
		var err error
		if temp, err = newTemporaryPassword(); err != nil {
			return CLIResetResult{}, err
		}
		if hash, err = hashPassword(temp); err != nil {
			return CLIResetResult{}, err
		}
	}
	var res CLIResetResult
	err := s.repo.InTx(ctx, func(tx Repository) error {
		u, found, err := s.findUserForCLI(ctx, tx, in.User)
		if err != nil {
			return err
		}
		if !found {
			return ErrCLIUserNotFound
		}
		cur, found, err := tx.LockUserWithIdentities(ctx, u.ID)
		if err != nil {
			return err
		}
		if !found {
			return ErrCLIUserNotFound
		}
		res = CLIResetResult{User: cur.User.Username}
		if !in.Password { // only the second factor was asked for: no credential changes
			reset, err := s.resetSecondFactorByCLI(ctx, tx, cur.User)
			res.SecondFactorReset = reset
			return err
		}
		if _, ok := cur.identity(IdentityEmail); !ok {
			return ErrCLINoPassword
		}
		return s.changeCredential(ctx, tx, credentialChange{
			User:        cur.User.ID,
			Revoke:      RevokeAll,
			SpendTokens: passwordTokenPurposes,
			Apply: func(l lockedCredential) error {
				if !l.HasEmail {
					// Unbound since it was read: there is no password to set.
					return ErrCLINoPassword
				}
				if in.SecondFactor {
					reset, err := s.resetSecondFactorByCLI(ctx, tx, cur.User)
					if err != nil {
						return err
					}
					res.SecondFactorReset = reset
				}
				if err := tx.SetTemporaryPassword(ctx, cur.User.ID, hash); err != nil {
					return err
				}
				res.Password = temp
				return nil
			},
			Audit:   &credentialAudit{By: AuditActorCLI, Event: passwordResetAudit(cur.User.ID)},
			Notices: []securityNotice{{To: cur.User.Email, Template: MailTemplatePasswordChangedByOperator}},
		})
	})
	if err != nil {
		return CLIResetResult{}, err
	}
	return res, nil
}

// resetSecondFactorByCLI resets the second factor of u when there is one, and says whether it did.
func (s *Service) resetSecondFactorByCLI(ctx context.Context, tx Repository, u User) (bool, error) {
	on, err := hasEnabledTOTP(ctx, tx, u.ID)
	if err != nil || !on {
		return false, err
	}
	if err := s.resetSecondFactor(ctx, tx, AuditActorCLI, u); err != nil {
		return false, err
	}
	return true, nil
}

// findUserForCLI finds a user by username, else by the login email of an email identity.
func (s *Service) findUserForCLI(ctx context.Context, tx Repository, name string) (User, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return User{}, false, nil
	}
	if u, ok, err := tx.FindUserByUsername(ctx, name); err != nil || ok {
		return u, ok, err
	}
	ident, ok, err := tx.FindIdentity(ctx, IdentityEmail, normalizeLoginEmail(name))
	if err != nil || !ok {
		return User{}, false, err
	}
	return tx.GetUser(ctx, ident.UserID)
}

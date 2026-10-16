package domain

import (
	"context"
	"errors"
	"log/slog"
	"net/mail"
	"strings"
)

// A credential change (GLOSSARY.md) is one action that replaces what proves who a user is.
// changeCredential is the one place that knows what such an action costs around the change
// itself, so that a call site cannot forget a step or take the locks in another order
// (CODING_STANDARDS.md, "Credential changes"): the locks, the change, the end of what the old
// credential opened, the audit entry and the notice, in that order, in one transaction.

// RevokePolicy says which of the user's sessions a credential change ends. Every caller states
// it; there is no default, so that a new caller has to decide.
type RevokePolicy int

const (
	revokeUnset RevokePolicy = iota // the zero value: refused, a caller that forgot to choose

	// RevokeNone leaves sessions and pending login challenges alone, as the second-factor
	// resets do: the user is not logged out by the loss of a device. No caller needs it yet
	// (the second-factor changes migrate in a later ticket).
	RevokeNone
	// RevokeOthers ends the pending login challenges and every session but the one in
	// credentialChange.KeepSession: the session that made the change stays.
	RevokeOthers
	// RevokeAll ends the pending login challenges and every session.
	RevokeAll
)

func (p RevokePolicy) String() string {
	switch p {
	case RevokeNone:
		return "none"
	case RevokeOthers:
		return "others"
	case RevokeAll:
		return "all"
	}
	return "unset"
}

// lockedCredential is what the locks returned, handed to the change unjudged: each caller
// refuses a user without an email identity, or a token it cannot honour, with its own error and
// in its own order.
type lockedCredential struct {
	Identity   LoginIdentity
	HasEmail   bool      // false: the user has no email identity (Identity is empty)
	Token      AuthToken // the locked token; empty unless credentialChange.Token is set
	TokenFound bool
}

type credentialAudit struct {
	By    AuditActor
	Event auditEvent
}

// securityNotice is a notice for notifySecurity.
type securityNotice struct {
	To       string
	Template MailTemplate
	Data     map[string]string
}

// credentialChange is one credential change for changeCredential.
type credentialChange struct {
	User UserID
	// Token is the hash of the one-time token that authorises the change, locked after the
	// identity. Empty: the change is not made with a token.
	Token string
	// Apply makes the change, with the locks held. It refuses it by returning an error, and
	// then nothing after it runs.
	Apply func(locked lockedCredential) error

	Revoke RevokePolicy // required
	// KeepSession is the hash of the session that stays under RevokeOthers; required with it.
	KeepSession string
	// SpendTokens are the purposes of the user's unused tokens that the change spends, before
	// the challenges and sessions end. A caller that does not spend tokens leaves it empty.
	SpendTokens []TokenPurpose

	// Audit and Notice are skipped when nil.
	Audit  *credentialAudit
	Notice *securityNotice
}

// changeCredential runs ch inside tx (call it within InTx). The order is fixed: the user's row
// and then the row of the email identity (LockEmailIdentity), the token, Apply, the tokens
// spent, the pending login challenges, the sessions (challenges before sessions: a login
// completing its second step holds the row of its challenge, so the delete waits for its session
// and the delete of the sessions that follows then sees it; see completeSecondFactorLogin),
// the audit entry, the notice.
func (s *Service) changeCredential(ctx context.Context, tx Repository, ch credentialChange) error {
	if ch.Apply == nil {
		return errors.New("credential change: no change given")
	}
	switch ch.Revoke {
	case RevokeNone, RevokeAll:
	case RevokeOthers:
		// DeleteOtherUserSessions with no session to keep would end every one of them.
		if ch.KeepSession == "" {
			return errors.New("credential change: RevokeOthers needs the session to keep")
		}
	default:
		return errors.New("credential change: no revoke policy given")
	}

	var locked lockedCredential
	var err error
	if locked.Identity, locked.HasEmail, err = tx.LockEmailIdentity(ctx, ch.User); err != nil {
		return err
	}
	if ch.Token != "" {
		if locked.Token, locked.TokenFound, err = tx.LockAuthToken(ctx, ch.Token); err != nil {
			return err
		}
	}
	if err := ch.Apply(locked); err != nil {
		return err
	}
	if len(ch.SpendTokens) > 0 {
		if err := tx.UseUserAuthTokens(ctx, ch.User, ch.SpendTokens, s.nowUTC()); err != nil {
			return err
		}
	}
	if ch.Revoke != RevokeNone {
		if err := tx.DeleteUserLoginChallenges(ctx, ch.User); err != nil {
			return err
		}
		if ch.Revoke == RevokeAll {
			err = tx.DeleteUserSessions(ctx, ch.User)
		} else {
			err = tx.DeleteOtherUserSessions(ctx, ch.User, ch.KeepSession)
		}
		if err != nil {
			return err
		}
	}
	if ch.Audit != nil {
		if err := s.recordAudit(ctx, tx, ch.Audit.By, ch.Audit.Event); err != nil {
			return err
		}
	}
	if n := ch.Notice; n != nil {
		return s.notifySecurity(ctx, tx, ch.User, n.To, n.Template, n.Data)
	}
	return nil
}

// notifySecurity queues a security notice about a login identity change to an address of the
// user, when mail works and the address is deliverable. A notice that cannot be sent never
// stops the change it is about, unless the transaction itself is lost (IsTxAborting).
func (s *Service) notifySecurity(ctx context.Context, tx Repository, userID UserID, to string, tpl MailTemplate, data map[string]string) error {
	_, usable, err := s.readUsableSMTP(ctx, tx)
	if err != nil {
		return err
	}
	to = strings.TrimSpace(to)
	if a, perr := mail.ParseAddress(to); !usable || perr != nil || a.Address != to || strings.ContainsAny(to, "\r\n") {
		return nil
	}
	if _, err := s.enqueueMail(ctx, tx, MailRequest{Template: tpl, To: to, Recipient: userID, Data: data}); err != nil {
		// Best effort, as documented: a statement that fails inside the transaction does not abort
		// it, so the change it is about still commits. An error that ends the transaction is not
		// swallowed: the change it is about is gone with it and must not be reported as done.
		if IsTxAborting(tx, err) {
			return err
		}
		slog.Warn("mail: a security notice could not be queued", "user", userID, "template", tpl, "err", err)
	}
	return nil
}

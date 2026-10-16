package domain

import (
	"context"

	"golang.org/x/crypto/bcrypt"
)

// ChangePassword is a user changing the password of their own email identity, after
// proving they know the current one. It is also how an administrator-chosen password is
// replaced at the first login: until then the session can do nothing else.
//
// sessionToken is the session that asks: it is freed from the must-change restriction and
// stays; every other session of the user ends, so that nobody holding a restricted session made
// with the temporary password gains access when the legitimate user changes it.
func (s *Service) ChangePassword(ctx context.Context, actor Actor, sessionToken, current, next string) error {
	var c Collector
	if current == "" {
		c.Add("currentPassword", CodeRequired, nil)
	}
	checkNewPasswordField(&c, "newPassword", next)
	if err := c.Err(); err != nil {
		return err
	}
	return s.repo.InTx(ctx, func(tx Repository) error {
		ident, found, err := tx.FindIdentityByUser(ctx, actor.User.ID, IdentityEmail)
		if err != nil {
			return err
		}
		if !found { // a directory-only user has no password here
			return newKind(KindNotFound, CodeNotFound)
		}
		if bcrypt.CompareHashAndPassword([]byte(ident.PasswordHash), []byte(current)) != nil {
			return Validation(Invalid("currentPassword", CodeInvalidCredentials, nil))
		}
		if current == next {
			return Validation(Invalid("newPassword", CodePasswordUnchanged, nil))
		}
		hash, err := hashPassword(next)
		if err != nil {
			return err
		}
		if err := tx.ChangePassword(ctx, actor.User.ID, hash); err != nil {
			return err
		}
		keep := hashToken(sessionToken)
		if err := tx.ClearSessionMustChangePassword(ctx, keep); err != nil {
			return err
		}
		return tx.DeleteOtherUserSessions(ctx, actor.User.ID, keep)
	})
}

package domain

import (
	"context"

	"golang.org/x/crypto/bcrypt"
)

// ChangePassword is a user changing the password of their own email identity, after
// proving they know the current one. It is also how an administrator-chosen password is
// replaced at the first login: until then the session can do nothing else.
func (s *Service) ChangePassword(ctx context.Context, actor Actor, current, next string) error {
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
		return tx.ChangePassword(ctx, actor.User.ID, hash)
	})
}

package domain

import (
	"context"
	"strings"
)

// Interface languages a user can choose. The preference is saved on the user so it
// follows them to other devices.
const (
	LanguageZh = "zh"
	LanguageEn = "en"
)

// GetMe returns the actor's own profile and login identities.
func (s *Service) GetMe(ctx context.Context, actor Actor) (UserWithIdentities, error) {
	me, found, err := s.repo.GetUserWithIdentities(ctx, actor.User.ID)
	if err != nil {
		return UserWithIdentities{}, err
	}
	if !found {
		return UserWithIdentities{}, newKind(KindNotFound, CodeNotFound)
	}
	return me, nil
}

// UpdateMeInput is a user's edit of their own settings. A nil field is left unchanged.
//
// Display name and email are editable only for users without an LDAP identity (the
// directory owns them otherwise); phone and language are editable by everyone.
type UpdateMeInput struct {
	DisplayName *string
	Email       *string
	Phone       *string
	Language    *string
}

// UpdateMe edits the actor's own settings.
func (s *Service) UpdateMe(ctx context.Context, actor Actor, in UpdateMeInput) (UserWithIdentities, error) {
	var result UserWithIdentities
	err := s.repo.InTx(ctx, func(tx Repository) error {
		cur, found, err := tx.LockUserWithIdentities(ctx, actor.User.ID)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		var c Collector
		u := applyProfileEdit(&c, cur.User, cur.hasIdentity(IdentityLDAP), profileEdit{
			DisplayName: in.DisplayName, Email: in.Email, Phone: in.Phone,
		})
		if in.Language != nil {
			lang := strings.TrimSpace(*in.Language)
			if lang != LanguageZh && lang != LanguageEn {
				c.Add("language", CodeInvalid, map[string]any{"allowed": []string{LanguageZh, LanguageEn}})
			}
			u.Language = lang
		}
		if err := c.Err(); err != nil {
			return err
		}
		if err := tx.UpdateUser(ctx, u, s.nowUTC()); err != nil {
			return mapUsernameTaken(err)
		}
		result = UserWithIdentities{User: u, Identities: cur.Identities}
		return nil
	})
	return result, err
}

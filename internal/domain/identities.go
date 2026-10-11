package domain

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// Binding and unbinding login identities (ADR-0002). There is no automatic merging:
// an identity is added to a user only by that user, after authenticating with it, or
// by an administrator.

// BindIdentityInput is a request to attach a login identity to a user.
//
//   - Kind "ldap": LoginName is the directory login name. A user binding it for
//     themselves must also give the directory Password (they authenticate with the
//     new identity once). An administrator gives only LoginName; the directory is asked
//     whether the login name exists.
//   - Kind "local": the login name is always the user's username. Password is the new
//     local password.
type BindIdentityInput struct {
	Kind      string
	LoginName string
	Password  string
}

type bindSpec struct {
	kind      IdentityKind
	loginName string
	hash      string // local only
	via       BoundVia
	// entry, when non-nil, is the directory entry the user's display name, email and
	// department are synced from right away (the directory owns them once bound).
	entry *DirectoryEntry
}

func (s *Service) checkBindableKind(c *Collector, kind string) {
	allowed := s.LoginMethods()
	if !slices.Contains(allowed, kind) {
		c.Add("kind", CodeInvalid, map[string]any{"allowed": allowed})
	}
}

// BindOwnIdentity attaches a login identity to the actor after they authenticated with it.
func (s *Service) BindOwnIdentity(ctx context.Context, actor Actor, in BindIdentityInput) (UserWithIdentities, error) {
	var c Collector
	s.checkBindableKind(&c, in.Kind)
	in.LoginName = strings.TrimSpace(in.LoginName)
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}
	if in.Kind == string(IdentityLocal) {
		return s.bindLocalIdentity(ctx, actor.User.ID, in.Password, BoundViaSelf)
	}
	c.Required("loginName", in.LoginName)
	if in.Password == "" {
		c.Add("password", CodeRequired, nil)
	}
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}

	// The directory is a network call: asked before the transaction.
	res := s.directory.Authenticate(ctx, in.LoginName, in.Password)
	switch res.Outcome {
	case DirectoryOK:
	case DirectoryUnreachable:
		return UserWithIdentities{}, ldapUnreachable()
	default: // bad password, not found, disabled: deliberately one answer. 422, not 401: the session is fine.
		return UserWithIdentities{}, Validation(Invalid("password", CodeInvalidCredentials, nil))
	}
	entry := res.Entry
	login := strings.TrimSpace(entry.Login)
	if login == "" {
		login = in.LoginName
	}
	return s.bindIdentity(ctx, actor.User.ID, bindSpec{kind: IdentityLDAP, loginName: login, via: BoundViaSelf, entry: &entry})
}

// bindLocalIdentity gives the user a local password identity. Its login name is the
// user's username (so a rename keeps them in step), the password is the one given:
// there is no existing credential to authenticate with, the user is already logged in.
func (s *Service) bindLocalIdentity(ctx context.Context, userID UserID, password string, via BoundVia) (UserWithIdentities, error) {
	var c Collector
	checkNewPassword(&c, password)
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return UserWithIdentities{}, err
	}
	// loginName stays empty: bindIdentity uses the username it reads under the lock.
	return s.bindIdentity(ctx, userID, bindSpec{kind: IdentityLocal, hash: hash, via: via})
}

// bindIdentity adds the identity inside one transaction, with the user row locked so
// that two concurrent binds cannot both pass the checks.
func (s *Service) bindIdentity(ctx context.Context, userID UserID, sp bindSpec) (UserWithIdentities, error) {
	var result UserWithIdentities
	err := s.repo.InTx(ctx, func(tx Repository) error {
		cur, found, err := tx.LockUserWithIdentities(ctx, userID)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		if cur.hasIdentity(sp.kind) {
			return Validation(Invalid("kind", CodeIdentityKindBound, nil))
		}
		if sp.kind == IdentityLocal {
			sp.loginName = cur.User.Username
		}
		if _, held, err := tx.FindIdentity(ctx, sp.kind, sp.loginName); err != nil {
			return err
		} else if held {
			return Validation(Invalid("loginName", CodeIdentityTaken, nil))
		}
		ident := LoginIdentity{
			UserID: userID, Kind: sp.kind, LoginName: sp.loginName,
			PasswordHash: sp.hash, BoundAt: s.nowUTC(), BoundVia: sp.via,
		}
		if err := tx.CreateIdentity(ctx, ident); errors.Is(err, ErrUsernameTaken) { // lost a race for the login name
			return Validation(Invalid("loginName", CodeIdentityTaken, nil))
		} else if err != nil {
			return err
		}
		cur.Identities = append(cur.Identities, ident)
		if sp.entry != nil {
			if err := s.syncFromDirectory(ctx, tx, &cur.User, *sp.entry); err != nil {
				return err
			}
		}
		result = cur
		return nil
	})
	return result, err
}

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
//   - Kind "email": LoginName is the login email. Only an administrator binds it (a user
//     verifies a new address by mail, which comes with the mail features): Password is the
//     temporary password the user must change at the first login.
type BindIdentityInput struct {
	Kind      string
	LoginName string
	Password  string
}

type bindSpec struct {
	kind      IdentityKind
	loginName string
	hash      string // email only
	// mustChange makes the user change the password at the first login: the password was
	// chosen by an administrator.
	mustChange bool
	// before runs inside the transaction, after the checks that the identity may be added and
	// before it is added; an error ends the bind (a commitThenFail commits what was recorded first).
	// A self-service email bind spends its code here.
	before func(tx Repository) error
	via    BoundVia
	by     AuditActor // who is doing the binding, for the audit log
	// entry, when non-nil, is the directory entry the user's display name, email and
	// department are synced from right away (the directory owns them once bound).
	entry *DirectoryEntry
}

// checkBindableKind checks the kind is one that may be bound: any login method for an
// administrator, only the directory for a user binding for themselves.
func (s *Service) checkBindableKind(ctx context.Context, c *Collector, kind string, byAdmin bool) error {
	allowed, err := s.LoginMethods(ctx)
	if err != nil {
		return err
	}
	if !byAdmin {
		allowed = slices.DeleteFunc(allowed, func(m string) bool { return m == string(IdentityEmail) })
	}
	if !slices.Contains(allowed, kind) {
		c.Add("kind", CodeInvalid, map[string]any{"allowed": allowed})
	}
	return nil
}

// BindOwnIdentity attaches a login identity to the actor after they authenticated with it.
func (s *Service) BindOwnIdentity(ctx context.Context, actor Actor, in BindIdentityInput) (UserWithIdentities, error) {
	var c Collector
	if err := s.checkBindableKind(ctx, &c, in.Kind, false); err != nil {
		return UserWithIdentities{}, err
	}
	in.LoginName = strings.TrimSpace(in.LoginName)
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}
	c.Required("loginName", in.LoginName)
	if in.Password == "" {
		c.Add("password", CodeRequired, nil)
	}
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}

	// The directory is a network call: asked before the transaction.
	dir, err := s.requireDirectory(ctx)
	if err != nil {
		return UserWithIdentities{}, err
	}
	res := dir.Authenticate(ctx, in.LoginName, in.Password)
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
	return s.bindIdentity(ctx, actor.User.ID, bindSpec{kind: IdentityLDAP, loginName: login, via: BoundViaSelf, by: AuditActorOf(actor), entry: &entry})
}

// UnbindOwnIdentity removes the actor's login identity of the given kind. The last
// identity cannot be removed (lastIdentity); the user and everything else stay.
func (s *Service) UnbindOwnIdentity(ctx context.Context, actor Actor, kind string) (UserWithIdentities, error) {
	var result UserWithIdentities
	err := s.repo.InTx(ctx, func(tx Repository) error {
		cur, found, err := tx.LockUserWithIdentities(ctx, actor.User.ID)
		if err != nil {
			return err
		}
		if !found || !cur.hasIdentity(IdentityKind(kind)) {
			return newKind(KindNotFound, CodeNotFound)
		}
		if len(cur.Identities) <= 1 {
			return Validation(Invalid("kind", CodeLastIdentity, nil))
		}
		removed, _ := cur.identity(IdentityKind(kind))
		var spend []TokenPurpose
		if removed.Kind == IdentityEmail {
			spend = passwordTokenPurposes // links mailed to the removed identity's address must not work any more
		}
		if err := s.changeCredential(ctx, tx, credentialChange{
			User:        cur.User.ID,
			Revoke:      RevokeNone,
			SpendTokens: spend,
			Apply: func(lockedCredential) error {
				return tx.DeleteIdentity(ctx, cur.User.ID, IdentityKind(kind))
			},
			Audit:   &credentialAudit{By: AuditActorOf(actor), Event: identityUnboundAudit(cur.User.ID, removed.Kind, removed.LoginName)},
			Notices: []securityNotice{{To: cur.User.Email, Template: MailTemplateIdentityUnbound, Data: map[string]string{"Kind": string(removed.Kind), "LoginName": removed.LoginName}}},
		}); err != nil {
			return err
		}
		cur.Identities = slices.DeleteFunc(cur.Identities, func(i LoginIdentity) bool { return i.Kind == IdentityKind(kind) })
		result = cur
		return nil
	})
	return result, err
}

// bindEmailIdentity gives the user an email identity with a temporary password chosen by
// an administrator. The administrator typed the login email, so it counts as verified.
func (s *Service) bindEmailIdentity(ctx context.Context, by AuditActor, userID UserID, loginEmail, password string) (UserWithIdentities, error) {
	var c Collector
	loginEmail = normalizeLoginEmail(loginEmail)
	checkLoginEmail(&c, "loginName", loginEmail)
	checkNewPassword(&c, password)
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return UserWithIdentities{}, err
	}
	return s.bindIdentity(ctx, userID, bindSpec{kind: IdentityEmail, loginName: loginEmail, hash: hash, via: BoundViaAdmin, by: by, mustChange: true})
}

// BindIdentityForUser is the administrator's bind. Unlike a user binding for themselves,
// the administrator does not authenticate with the identity: for "ldap" the directory is
// only asked whether the login name exists (a disabled directory account counts as
// existing, like in ADR-0004's name check; it simply cannot log in), and for "email"
// the administrator types the login email and sets a temporary password. The binding
// source is "admin".
func (s *Service) BindIdentityForUser(ctx context.Context, actor Actor, id UserID, in BindIdentityInput) (UserWithIdentities, error) {
	if err := requirePermission(actor, PermUserManage); err != nil {
		return UserWithIdentities{}, err
	}
	var c Collector
	if err := s.checkBindableKind(ctx, &c, in.Kind, true); err != nil {
		return UserWithIdentities{}, err
	}
	in.LoginName = strings.TrimSpace(in.LoginName)
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}
	if in.Kind == string(IdentityEmail) {
		return s.bindEmailIdentity(ctx, AuditActorOf(actor), id, in.LoginName, in.Password)
	}
	if !c.Required("loginName", in.LoginName) {
		return UserWithIdentities{}, c.Err()
	}

	dir, err := s.requireDirectory(ctx)
	if err != nil {
		return UserWithIdentities{}, err
	}
	res := dir.Lookup(ctx, in.LoginName)
	spec := bindSpec{kind: IdentityLDAP, loginName: in.LoginName, via: BoundViaAdmin, by: AuditActorOf(actor)}
	switch res.Outcome {
	case DirectoryOK:
		spec.entry = &res.Entry
	case DirectoryDisabled: // exists; nothing current to sync
	case DirectoryNotFound:
		return UserWithIdentities{}, Validation(Invalid("loginName", CodeLdapLoginNotFound, nil))
	default: // unreachable or anything unexpected: fail closed
		return UserWithIdentities{}, ldapUnreachable()
	}
	if login := strings.TrimSpace(res.Entry.Login); login != "" {
		spec.loginName = login
	}
	return s.bindIdentity(ctx, id, spec)
}

// bindIdentity adds the identity inside one transaction, with the user row locked so
// that two concurrent binds cannot both pass the checks.
func (s *Service) bindIdentity(ctx context.Context, userID UserID, sp bindSpec) (UserWithIdentities, error) {
	var result UserWithIdentities
	err := s.inTxKeeping(ctx, func(tx Repository) error {
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
		taken := func() error {
			if sp.kind == IdentityEmail {
				return loginEmailTaken("loginName")
			}
			return Validation(Invalid("loginName", CodeIdentityTaken, nil))
		}
		if _, held, err := tx.FindIdentity(ctx, sp.kind, sp.loginName); err != nil {
			return err
		} else if held {
			return taken()
		}
		if sp.before != nil {
			if err := sp.before(tx); err != nil {
				return err
			}
		}
		ident := LoginIdentity{
			UserID: userID, Kind: sp.kind, LoginName: sp.loginName, PasswordHash: sp.hash, PasswordSet: sp.hash != "",
			BoundAt: s.nowUTC(), BoundVia: sp.via, MustChangePassword: sp.mustChange,
		}
		// The directory's profile is taken before the identity is created, so that the notice goes
		// to the contact email as it is afterwards. Both are undone together if the bind fails.
		if sp.entry != nil {
			if err := s.syncFromDirectory(ctx, tx, &cur.User, *sp.entry); err != nil {
				return err
			}
		}
		if err := s.changeCredential(ctx, tx, credentialChange{
			User:   userID,
			Revoke: RevokeNone,
			Apply: func(lockedCredential) error {
				err := tx.CreateIdentity(ctx, ident)
				if errors.Is(err, ErrIdentityTaken) { // lost a race for the login name
					return taken()
				}
				return err
			},
			Audit:   &credentialAudit{By: sp.by, Event: identityBoundAudit(userID, sp.kind, sp.loginName, sp.via)},
			Notices: []securityNotice{{To: cur.User.Email, Template: MailTemplateIdentityBound, Data: map[string]string{"Kind": string(sp.kind), "LoginName": sp.loginName}}},
		}); err != nil {
			return err
		}
		cur.Identities = append(cur.Identities, ident)
		result = cur
		return nil
	})
	return result, err
}

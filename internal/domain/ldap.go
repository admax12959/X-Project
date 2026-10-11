package domain

import (
	"context"
	"strings"
	"time"
)

// IdentityRepository is the part of the persistence port that LDAP login needs.
// It is embedded in Repository.
type IdentityRepository interface {
	// FindUserByUsername matches the username as stored (MySQL compares case-insensitively).
	FindUserByUsername(ctx context.Context, username string) (User, bool, error)
	// UpdateUserProfile overwrites the directory-owned fields (display name, email, department).
	UpdateUserProfile(ctx context.Context, id UserID, displayName, email, department string, at time.Time) error
	// FindIdentityByUser returns the user's identity of the given kind, if any.
	FindIdentityByUser(ctx context.Context, userID UserID, kind IdentityKind) (LoginIdentity, bool, error)
}

func ldapUnreachable() error { return newKind(KindUnavailable, CodeLdapUnreachable) }

// loginLDAP implements Login for method "ldap" (ADR-0002, ADR-0004).
//
//   - The directory decides whether the account is valid, every time. When it reports
//     the account missing, disabled or the password wrong, the login fails with the
//     uniform invalidCredentials error and nothing is stored or deleted: the LDAP
//     identity is "invalid" only in the sense that it cannot log in right now, and the
//     user and any other identity are untouched.
//   - Unreachable is its own error and is never treated as "account missing".
//   - A known LDAP identity signs in to its user (whoever bound it) and overwrites the
//     directory-owned fields.
//   - An unknown LDAP identity creates a new user only if the username is free. It is
//     never merged into an existing user by username or email.
func (s *Service) loginLDAP(ctx context.Context, in LoginInput) (LoginResult, error) {
	res := s.directory.Authenticate(ctx, strings.TrimSpace(in.Username), in.Password)
	switch res.Outcome {
	case DirectoryOK:
	case DirectoryUnreachable:
		return LoginResult{}, ldapUnreachable()
	default: // bad password, not found, disabled
		return LoginResult{}, invalidCredentials()
	}
	entry := res.Entry
	login := strings.TrimSpace(entry.Login)
	if login == "" {
		login = strings.TrimSpace(in.Username)
	}

	var user User
	err := s.repo.InTx(ctx, func(tx Repository) error {
		ident, found, err := tx.FindIdentity(ctx, IdentityLDAP, login)
		if err != nil {
			return err
		}
		if found {
			u, ok, err := tx.GetUser(ctx, ident.UserID)
			if err != nil {
				return err
			}
			if !ok || u.Status != UserEnabled {
				return invalidCredentials()
			}
			name := strings.TrimSpace(entry.DisplayName)
			if name == "" {
				name = u.Username
			}
			if err := tx.UpdateUserProfile(ctx, u.ID, name, entry.Email, entry.Department, s.nowUTC()); err != nil {
				return err
			}
			u.DisplayName, u.Email, u.Department = name, entry.Email, entry.Department
			user = u
			return nil
		}

		username := strings.ToLower(login)
		if !usernamePattern.MatchString(username) {
			return Validation(Invalid("username", CodeInvalid, nil))
		}
		if _, taken, err := tx.FindUserByUsername(ctx, username); err != nil {
			return err
		} else if taken {
			return Validation(Invalid("username", CodeUsernameTakenByLocal, nil))
		}
		now := s.nowUTC()
		name := strings.TrimSpace(entry.DisplayName)
		if name == "" {
			name = username
		}
		u := User{
			Username: username, DisplayName: name, Email: entry.Email, Department: entry.Department,
			Status: UserEnabled, CreatedAt: now,
		}
		id, err := tx.CreateUser(ctx, u)
		if err != nil {
			return err
		}
		u.ID = id
		if err := tx.CreateIdentity(ctx, LoginIdentity{
			UserID: id, Kind: IdentityLDAP, LoginName: login, BoundAt: now, BoundVia: BoundViaFirstLogin,
		}); err != nil {
			return err
		}
		user = u
		return nil
	})
	if err != nil {
		return LoginResult{}, err
	}
	return s.openSession(ctx, user)
}

// CheckUsernameFreeInDirectory is the administrator-side rule of ADR-0004: before a
// local user is created, the directory is asked whether the username is already a
// login name there. An existing (even disabled) directory account blocks creation with
// usernameExistsInLdap; an unreachable directory fails the creation with ldapUnreachable
// and the check is never skipped. With no directory configured there is nothing to check.
// field is the request field to report the error on (normally "username").
func (s *Service) CheckUsernameFreeInDirectory(ctx context.Context, field, username string) error {
	if s.directory == nil {
		return nil
	}
	switch s.directory.Lookup(ctx, strings.TrimSpace(username)).Outcome {
	case DirectoryNotFound:
		return nil
	case DirectoryOK, DirectoryDisabled:
		return Validation(Invalid(field, CodeUsernameExistsInLdap, nil))
	default: // unreachable or anything unexpected: fail closed
		return ldapUnreachable()
	}
}

package domain

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

// UserWithIdentities is a user together with every login identity attached to it.
type UserWithIdentities struct {
	User       User
	Identities []LoginIdentity
}

// UserRepository is the user-management part of Repository.
type UserRepository interface {
	// ListUsers returns every user (oldest first) with its login identities.
	ListUsers(ctx context.Context) ([]UserWithIdentities, error)
}

// requireAdmin is the permission check for every user-management operation.
func requireAdmin(actor Actor) error {
	if !actor.User.IsAdmin {
		return newKind(KindForbidden, CodeForbidden)
	}
	return nil
}

// ListUsers returns all users for the administrator's user list.
func (s *Service) ListUsers(ctx context.Context, actor Actor) ([]UserWithIdentities, error) {
	if err := requireAdmin(actor); err != nil {
		return nil, err
	}
	return s.repo.ListUsers(ctx)
}

// CreateLocalUserInput is the administrator's request to create a local user.
type CreateLocalUserInput struct {
	Username    string
	DisplayName string
	Email       string
	Phone       string
	Department  string
	Password    string
}

// CreateLocalUser creates a user with a local login identity bound by the
// administrator. It is the single entry point for creating local users.
func (s *Service) CreateLocalUser(ctx context.Context, actor Actor, in CreateLocalUserInput) (UserWithIdentities, error) {
	if err := requireAdmin(actor); err != nil {
		return UserWithIdentities{}, err
	}
	var c Collector
	in.Username = strings.TrimSpace(in.Username)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.Email = strings.TrimSpace(in.Email)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Department = strings.TrimSpace(in.Department)
	checkUsername(&c, in.Username)
	checkProfile(&c, in.DisplayName, in.Email, in.Phone, in.Department)
	if in.Password == "" {
		c.Add("password", CodeRequired, nil)
	} else if utf8.RuneCountInString(in.Password) < minPasswordLength {
		c.Add("password", CodeTooShort, map[string]any{"min": minPasswordLength})
	} else if len(in.Password) > maxPasswordBytes {
		c.Add("password", CodeTooLong, map[string]any{"max": maxPasswordBytes})
	}
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		return UserWithIdentities{}, err
	}
	var created UserWithIdentities
	err = s.repo.InTx(ctx, func(tx Repository) error {
		now := s.nowUTC()
		u := User{
			Username: strings.TrimSpace(in.Username), DisplayName: in.DisplayName, Email: in.Email,
			Phone: in.Phone, Department: in.Department, Status: UserEnabled, CreatedAt: now,
		}
		id, err := tx.CreateUser(ctx, u)
		if errors.Is(err, ErrUsernameTaken) {
			return usernameTaken()
		}
		if err != nil {
			return err
		}
		u.ID = id
		ident := LoginIdentity{
			UserID: id, Kind: IdentityLocal, LoginName: u.Username,
			PasswordHash: hash, BoundAt: now, BoundVia: BoundViaAdmin,
		}
		created = UserWithIdentities{User: u, Identities: []LoginIdentity{ident}}
		err = tx.CreateIdentity(ctx, ident)
		if errors.Is(err, ErrUsernameTaken) {
			return usernameTaken()
		}
		return err
	})
	return created, err
}

// Field limits, matching the users table.
const (
	minPasswordLength = 8
	maxPasswordBytes  = 72 // bcrypt ignores or rejects anything longer
	maxUsernameLength = 32
	maxNameLength     = 200 // display name and department
	maxEmailLength    = 320
	maxPhoneLength    = 64
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// ErrUsernameTaken is returned by the Repository when a unique key on the
// username (or the local login name derived from it) is violated.
var ErrUsernameTaken = errors.New("username already taken")

func usernameTaken() error {
	return Validation(Invalid("username", CodeUsernameTaken, nil))
}

// checkUsername validates the username format (spec #2: ^[a-z][a-z0-9._-]{1,31}$).
func checkUsername(c *Collector, username string) {
	if username == "" {
		c.Add("username", CodeRequired, nil)
	} else if !usernamePattern.MatchString(username) {
		c.Add("username", CodeInvalid, nil)
	}
}

func checkLength(c *Collector, field, value string, max int) {
	if utf8.RuneCountInString(value) > max {
		c.Add(field, CodeTooLong, map[string]any{"max": max})
	}
}

// checkProfile validates display name (required), email (optional, shaped like
// an address), phone and department. Values are already trimmed.
func checkProfile(c *Collector, displayName, email, phone, department string) {
	if displayName == "" {
		c.Add("displayName", CodeRequired, nil)
	} else {
		checkLength(c, "displayName", displayName, maxNameLength)
	}
	if email != "" {
		if !emailPattern.MatchString(email) {
			c.Add("email", CodeInvalid, nil)
		} else {
			checkLength(c, "email", email, maxEmailLength)
		}
	}
	checkLength(c, "phone", phone, maxPhoneLength)
	checkLength(c, "department", department, maxNameLength)
}

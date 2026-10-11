package domain

import (
	"context"
	"strings"
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
		if err != nil {
			return err
		}
		u.ID = id
		ident := LoginIdentity{
			UserID: id, Kind: IdentityLocal, LoginName: u.Username,
			PasswordHash: hash, BoundAt: now, BoundVia: BoundViaAdmin,
		}
		created = UserWithIdentities{User: u, Identities: []LoginIdentity{ident}}
		return tx.CreateIdentity(ctx, ident)
	})
	return created, err
}

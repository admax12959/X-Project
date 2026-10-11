package domain

import (
	"context"
	"strings"
)

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
func (s *Service) CreateLocalUser(ctx context.Context, actor Actor, in CreateLocalUserInput) (User, error) {
	hash, err := hashPassword(in.Password)
	if err != nil {
		return User{}, err
	}
	var created User
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
		created = u
		return tx.CreateIdentity(ctx, LoginIdentity{
			UserID: id, Kind: IdentityLocal, LoginName: u.Username,
			PasswordHash: hash, BoundAt: now, BoundVia: BoundViaAdmin,
		})
	})
	return created, err
}

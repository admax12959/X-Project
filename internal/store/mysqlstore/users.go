package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/admax12959/X-Project/internal/domain"
)

// mapUsernameConflict turns a duplicate-key error on the username or on the local
// login name into domain.ErrUsernameTaken. Other errors pass through.
func mapUsernameConflict(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 &&
		(strings.Contains(me.Message, "uq_users_username") || strings.Contains(me.Message, "uq_identity_login")) {
		return domain.ErrUsernameTaken
	}
	return err
}

func (s *Store) LockUserWithIdentities(ctx context.Context, id domain.UserID) (domain.UserWithIdentities, bool, error) {
	u, err := scanUser(s.q.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE id = ? FOR UPDATE", id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.UserWithIdentities{}, false, nil
	}
	if err != nil {
		return domain.UserWithIdentities{}, false, err
	}
	out := domain.UserWithIdentities{User: u, Identities: []domain.LoginIdentity{}}
	rows, err := s.q.QueryContext(ctx, "SELECT user_id, kind, login_name, bound_at, bound_via FROM login_identities WHERE user_id = ? ORDER BY id", id)
	if err != nil {
		return out, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var i domain.LoginIdentity
		var kind, via string
		if err := rows.Scan(&i.UserID, &kind, &i.LoginName, &i.BoundAt, &via); err != nil {
			return out, false, err
		}
		i.Kind, i.BoundVia = domain.IdentityKind(kind), domain.BoundVia(via)
		out.Identities = append(out.Identities, i)
	}
	return out, true, rows.Err()
}

func (s *Store) UpdateUser(ctx context.Context, u domain.User, at time.Time) error {
	_, err := s.q.ExecContext(ctx,
		"UPDATE users SET username = ?, display_name = ?, email = ?, phone = ?, department = ?, language = ?, updated_at = ? WHERE id = ?",
		u.Username, u.DisplayName, u.Email, u.Phone, u.Department, u.Language, at, u.ID)
	return mapUsernameConflict(err)
}

func (s *Store) RenameLocalLogin(ctx context.Context, id domain.UserID, loginName string) error {
	_, err := s.q.ExecContext(ctx, "UPDATE login_identities SET login_name = ? WHERE user_id = ? AND kind = ?",
		loginName, id, string(domain.IdentityLocal))
	return mapUsernameConflict(err)
}

func (s *Store) SetUserStatus(ctx context.Context, id domain.UserID, status domain.UserStatus, at time.Time) error {
	_, err := s.q.ExecContext(ctx, "UPDATE users SET status = ?, updated_at = ? WHERE id = ?", string(status), at, id)
	return err
}

func (s *Store) ListUsers(ctx context.Context) ([]domain.UserWithIdentities, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT "+userCols+" FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.UserWithIdentities{}
	index := map[domain.UserID]int{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		index[u.ID] = len(out)
		out = append(out, domain.UserWithIdentities{User: u, Identities: []domain.LoginIdentity{}})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	idRows, err := s.q.QueryContext(ctx, "SELECT user_id, kind, login_name, bound_at, bound_via FROM login_identities ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer idRows.Close()
	for idRows.Next() {
		var i domain.LoginIdentity
		var kind, via string
		if err := idRows.Scan(&i.UserID, &kind, &i.LoginName, &i.BoundAt, &via); err != nil {
			return nil, err
		}
		i.Kind, i.BoundVia = domain.IdentityKind(kind), domain.BoundVia(via)
		if at, ok := index[i.UserID]; ok {
			out[at].Identities = append(out[at].Identities, i)
		}
	}
	return out, idRows.Err()
}

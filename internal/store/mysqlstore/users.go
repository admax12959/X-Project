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

// mapUsernameConflict turns a duplicate-key error on the username into
// domain.ErrUsernameTaken and one on a login identity's (kind, login name) into
// domain.ErrIdentityTaken. Other errors pass through.
func mapUsernameConflict(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		switch {
		case strings.Contains(me.Message, "uq_users_username"):
			return domain.ErrUsernameTaken
		case strings.Contains(me.Message, "uq_identity_login"):
			return domain.ErrIdentityTaken
		}
	}
	return err
}

func (s *Store) LockUserWithIdentities(ctx context.Context, id domain.UserID) (domain.UserWithIdentities, bool, error) {
	return s.userWithIdentities(ctx, id, " FOR UPDATE")
}

func (s *Store) GetUserWithIdentities(ctx context.Context, id domain.UserID) (domain.UserWithIdentities, bool, error) {
	return s.userWithIdentities(ctx, id, "")
}

func (s *Store) userWithIdentities(ctx context.Context, id domain.UserID, lock string) (domain.UserWithIdentities, bool, error) {
	u, err := scanUser(s.q.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE id = ?"+lock, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.UserWithIdentities{}, false, nil
	}
	if err != nil {
		return domain.UserWithIdentities{}, false, err
	}
	out := domain.UserWithIdentities{User: u, Identities: []domain.LoginIdentity{}}
	if err := s.q.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM user_authenticators WHERE user_id = ? AND type = 'totp' AND status = 'enabled')", id).Scan(&out.SecondFactorEnabled); err != nil {
		return out, false, err
	}
	rows, err := s.q.QueryContext(ctx, "SELECT user_id, kind, login_name, bound_at, bound_via, password_hash IS NOT NULL FROM login_identities WHERE user_id = ? ORDER BY id", id)
	if err != nil {
		return out, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var i domain.LoginIdentity
		var kind, via string
		if err := rows.Scan(&i.UserID, &kind, &i.LoginName, &i.BoundAt, &via, &i.PasswordSet); err != nil {
			return out, false, err
		}
		i.Kind, i.BoundVia = domain.IdentityKind(kind), domain.BoundVia(via)
		out.Identities = append(out.Identities, i)
	}
	return out, true, rows.Err()
}

func (s *Store) UpdateUser(ctx context.Context, u domain.User, at time.Time) error {
	_, err := s.q.ExecContext(ctx,
		"UPDATE users SET username = ?, display_name = ?, email = ?, phone = ?, department = ?, department_id = ?, language = ?, updated_at = ? WHERE id = ?",
		u.Username, u.DisplayName, u.Email, u.Phone, u.DirectoryDepartment, nullableDepartment(u.DepartmentID), u.Language, at, u.ID)
	return mapDepartmentFK(mapUsernameConflict(err))
}

func (s *Store) DeleteIdentity(ctx context.Context, id domain.UserID, kind domain.IdentityKind) error {
	_, err := s.q.ExecContext(ctx, "DELETE FROM login_identities WHERE user_id = ? AND kind = ?", id, string(kind))
	return err
}

func (s *Store) RenameEmailLogin(ctx context.Context, id domain.UserID, loginEmail string) error {
	_, err := s.q.ExecContext(ctx, "UPDATE login_identities SET login_name = ? WHERE user_id = ? AND kind = ?",
		loginEmail, id, string(domain.IdentityEmail))
	return mapUsernameConflict(err)
}

func (s *Store) ChangePassword(ctx context.Context, id domain.UserID, hash string) error {
	if _, err := s.q.ExecContext(ctx, "UPDATE login_identities SET password_hash = ?, must_change_password = FALSE WHERE user_id = ? AND kind = ?",
		hash, id, string(domain.IdentityEmail)); err != nil {
		return err
	}
	_, err := s.q.ExecContext(ctx, "UPDATE sessions SET must_change_password = FALSE WHERE user_id = ?", id)
	return err
}

func (s *Store) SetTemporaryPassword(ctx context.Context, id domain.UserID, hash string) error {
	_, err := s.q.ExecContext(ctx, "UPDATE login_identities SET password_hash = ?, must_change_password = TRUE WHERE user_id = ? AND kind = ?",
		hash, id, string(domain.IdentityEmail))
	return err
}

func (s *Store) SetUserStatus(ctx context.Context, id domain.UserID, status domain.UserStatus, at time.Time) error {
	_, err := s.q.ExecContext(ctx, "UPDATE users SET status = ?, updated_at = ? WHERE id = ?", string(status), at, id)
	return err
}

func (s *Store) SetUserAdmin(ctx context.Context, id domain.UserID, isAdmin bool, at time.Time) error {
	_, err := s.q.ExecContext(ctx, "UPDATE users SET is_admin = ?, updated_at = ? WHERE id = ?", isAdmin, at, id)
	return err
}

func (s *Store) LockEnabledAdmins(ctx context.Context) ([]domain.UserID, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT id FROM users WHERE is_admin AND status = 'enabled' ORDER BY id FOR UPDATE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []domain.UserID
	for rows.Next() {
		var id domain.UserID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) ListUsers(ctx context.Context) ([]domain.UserWithIdentities, error) {
	return s.listUsers(ctx, "", nil)
}

func (s *Store) ListUsersInDepartments(ctx context.Context, ids []domain.DepartmentID) ([]domain.UserWithIdentities, error) {
	if len(ids) == 0 {
		return []domain.UserWithIdentities{}, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return s.listUsers(ctx, " WHERE department_id IN (?"+strings.Repeat(",?", len(ids)-1)+")", args)
}

// listUsers reads users (oldest first) with their login identities; where is an optional
// condition on the users table, with its arguments.
func (s *Store) listUsers(ctx context.Context, where string, args []any) ([]domain.UserWithIdentities, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT "+userCols+" FROM users"+where+" ORDER BY id", args...)
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

	idRows, err := s.q.QueryContext(ctx, "SELECT user_id, kind, login_name, bound_at, bound_via, password_hash IS NOT NULL FROM login_identities WHERE user_id IN (SELECT id FROM users"+where+") ORDER BY id", args...)
	if err != nil {
		return nil, err
	}
	defer idRows.Close()
	for idRows.Next() {
		var i domain.LoginIdentity
		var kind, via string
		if err := idRows.Scan(&i.UserID, &kind, &i.LoginName, &i.BoundAt, &via, &i.PasswordSet); err != nil {
			return nil, err
		}
		i.Kind, i.BoundVia = domain.IdentityKind(kind), domain.BoundVia(via)
		if at, ok := index[i.UserID]; ok {
			out[at].Identities = append(out[at].Identities, i)
		}
	}
	if err := idRows.Err(); err != nil {
		return nil, err
	}

	// Who has a second factor turned on.
	authRows, err := s.q.QueryContext(ctx, "SELECT DISTINCT user_id FROM user_authenticators WHERE type = 'totp' AND status = 'enabled' AND user_id IN (SELECT id FROM users"+where+")", args...)
	if err != nil {
		return nil, err
	}
	defer authRows.Close()
	for authRows.Next() {
		var id domain.UserID
		if err := authRows.Scan(&id); err != nil {
			return nil, err
		}
		if at, ok := index[id]; ok {
			out[at].SecondFactorEnabled = true
		}
	}
	return out, authRows.Err()
}

func (s *Store) ReadUsersLocked(ctx context.Context, ids []domain.UserID) (map[domain.UserID]domain.User, error) {
	out := map[domain.UserID]domain.User{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.q.QueryContext(ctx,
		"SELECT "+userCols+" FROM users WHERE id IN (?"+strings.Repeat(",?", len(ids)-1)+") ORDER BY id FOR SHARE", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out[u.ID] = u
	}
	return out, rows.Err()
}

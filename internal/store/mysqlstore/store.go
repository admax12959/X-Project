// Package mysqlstore implements domain.Repository on MySQL 8.
// Only the domain service uses it.
package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/admax12959/X-Project/internal/domain"
)

// querier is satisfied by *sql.DB and *sql.Tx.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store is the MySQL repository.
type Store struct {
	db *sql.DB
	q  querier
}

var _ domain.Repository = (*Store)(nil)

func New(db *sql.DB) *Store { return &Store{db: db, q: db} }

// NormalizeDSN forces the connection settings the store relies on:
// parseTime, UTC, and no multi-statements.
func NormalizeDSN(dsn string) (string, error) {
	c, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("database DSN: %w", err)
	}
	c.ParseTime = true
	c.Loc = time.UTC
	c.MultiStatements = false
	if c.Params == nil {
		c.Params = map[string]string{}
	}
	c.Params["time_zone"] = "'+00:00'"
	return c.FormatDSN(), nil
}

// Open connects to MySQL, retrying until timeout (the database may still be starting).
func Open(ctx context.Context, dsn string, timeout time.Duration) (*sql.DB, error) {
	dsn, err := NormalizeDSN(dsn)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	deadline := time.Now().Add(timeout)
	for {
		err = db.PingContext(ctx)
		if err == nil {
			return db, nil
		}
		if time.Now().After(deadline) {
			db.Close()
			return nil, fmt.Errorf("database not reachable after %s: %w", timeout, err)
		}
		select {
		case <-ctx.Done():
			db.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (s *Store) InTx(ctx context.Context, fn func(tx domain.Repository) error) error {
	if _, nested := s.q.(*sql.Tx); nested {
		return fn(s)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(&Store{db: s.db, q: tx}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) LockBootstrap(ctx context.Context) (bool, error) {
	var at sql.NullTime
	err := s.q.QueryRowContext(ctx, "SELECT bootstrapped_at FROM system_state WHERE id = 1 FOR UPDATE").Scan(&at)
	return at.Valid, err
}

func (s *Store) MarkBootstrapped(ctx context.Context, at time.Time) error {
	_, err := s.q.ExecContext(ctx, "UPDATE system_state SET bootstrapped_at = ? WHERE id = 1", at)
	return err
}

// ---------------------------------------------------------------- users

const userCols = "id, username, display_name, email, phone, department, department_id, language, status, is_admin, created_at, avatar_key, avatar_hash, avatar_source"

func scanUser(row interface{ Scan(...any) error }) (domain.User, error) {
	var u domain.User
	var status string
	var dept sql.NullInt64
	var avatarKey, avatarHash, avatarSource sql.NullString
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Phone, &u.DirectoryDepartment, &dept, &u.Language, &status, &u.IsAdmin, &u.CreatedAt,
		&avatarKey, &avatarHash, &avatarSource)
	u.Status, u.DepartmentID = domain.UserStatus(status), domain.DepartmentID(dept.Int64)
	u.AvatarKey, u.AvatarHash, u.AvatarSource = avatarKey.String, avatarHash.String, domain.AvatarSource(avatarSource.String)
	return u, err
}

func (s *Store) CreateUser(ctx context.Context, u domain.User) (domain.UserID, error) {
	res, err := s.q.ExecContext(ctx,
		"INSERT INTO users (username, display_name, email, phone, department, department_id, status, is_admin, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		u.Username, u.DisplayName, u.Email, u.Phone, u.DirectoryDepartment, nullableDepartment(u.DepartmentID), string(u.Status), u.IsAdmin, u.CreatedAt, u.CreatedAt)
	if err != nil {
		return 0, mapDepartmentFK(mapUsernameConflict(err))
	}
	id, err := res.LastInsertId()
	return domain.UserID(id), err
}

func (s *Store) GetUser(ctx context.Context, id domain.UserID) (domain.User, bool, error) {
	u, err := scanUser(s.q.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, false, nil
	}
	return u, err == nil, err
}

func (s *Store) FindUserByUsername(ctx context.Context, username string) (domain.User, bool, error) {
	u, err := scanUser(s.q.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE username = ?", username))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, false, nil
	}
	return u, err == nil, err
}

func (s *Store) UpdateUserProfile(ctx context.Context, id domain.UserID, displayName, email, department string, at time.Time) error {
	_, err := s.q.ExecContext(ctx, "UPDATE users SET display_name = ?, email = ?, department = ?, updated_at = ? WHERE id = ?",
		displayName, email, department, at, id)
	return err
}

func (s *Store) FindIdentityByUser(ctx context.Context, userID domain.UserID, kind domain.IdentityKind) (domain.LoginIdentity, bool, error) {
	var i domain.LoginIdentity
	var k, via string
	var hash sql.NullString
	err := s.q.QueryRowContext(ctx,
		"SELECT id, user_id, kind, login_name, password_hash, bound_at, bound_via, must_change_password FROM login_identities WHERE user_id = ? AND kind = ?",
		userID, string(kind)).Scan(&i.ID, &i.UserID, &k, &i.LoginName, &hash, &i.BoundAt, &via, &i.MustChangePassword)
	if errors.Is(err, sql.ErrNoRows) {
		return i, false, nil
	}
	if err != nil {
		return i, false, err
	}
	i.Kind, i.BoundVia, i.PasswordHash, i.PasswordSet = domain.IdentityKind(k), domain.BoundVia(via), hash.String, hash.Valid
	return i, true, nil
}

func (s *Store) CreateIdentity(ctx context.Context, i domain.LoginIdentity) error {
	var hash sql.NullString
	if i.PasswordHash != "" {
		hash = sql.NullString{String: i.PasswordHash, Valid: true}
	}
	_, err := s.q.ExecContext(ctx,
		"INSERT INTO login_identities (user_id, kind, login_name, password_hash, bound_at, bound_via, must_change_password) VALUES (?, ?, ?, ?, ?, ?, ?)",
		i.UserID, string(i.Kind), i.LoginName, hash, i.BoundAt, string(i.BoundVia), i.MustChangePassword)
	return mapUsernameConflict(err)
}

func (s *Store) FindIdentity(ctx context.Context, kind domain.IdentityKind, loginName string) (domain.LoginIdentity, bool, error) {
	var i domain.LoginIdentity
	var k, via string
	var hash sql.NullString
	err := s.q.QueryRowContext(ctx,
		"SELECT id, user_id, kind, login_name, password_hash, bound_at, bound_via, must_change_password FROM login_identities WHERE kind = ? AND login_name = ?",
		string(kind), loginName).Scan(&i.ID, &i.UserID, &k, &i.LoginName, &hash, &i.BoundAt, &via, &i.MustChangePassword)
	if errors.Is(err, sql.ErrNoRows) {
		return i, false, nil
	}
	if err != nil {
		return i, false, err
	}
	i.Kind, i.BoundVia, i.PasswordHash, i.PasswordSet = domain.IdentityKind(k), domain.BoundVia(via), hash.String, hash.Valid
	return i, true, nil
}

// ---------------------------------------------------------------- sessions

func (s *Store) CreateSession(ctx context.Context, sess domain.Session) error {
	_, err := s.q.ExecContext(ctx, "INSERT INTO sessions (token_hash, user_id, created_at, expires_at, must_change_password) VALUES (?, ?, ?, ?, ?)",
		sess.TokenHash, sess.UserID, sess.CreatedAt, sess.ExpiresAt, sess.MustChangePassword)
	return err
}

func (s *Store) FindSession(ctx context.Context, tokenHash string) (domain.Session, bool, error) {
	var sess domain.Session
	err := s.q.QueryRowContext(ctx, "SELECT token_hash, user_id, created_at, expires_at, must_change_password FROM sessions WHERE token_hash = ?", tokenHash).
		Scan(&sess.TokenHash, &sess.UserID, &sess.CreatedAt, &sess.ExpiresAt, &sess.MustChangePassword)
	if errors.Is(err, sql.ErrNoRows) {
		return sess, false, nil
	}
	return sess, err == nil, err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.q.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", tokenHash)
	return err
}

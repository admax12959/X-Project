package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

func (s *Store) InsertAuthToken(ctx context.Context, t domain.AuthToken) error {
	_, err := s.q.ExecContext(ctx, "INSERT INTO auth_tokens (token_hash, user_id, purpose, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		t.TokenHash, t.UserID, string(t.Purpose), t.CreatedAt, t.ExpiresAt)
	return err
}

func (s *Store) LockAuthToken(ctx context.Context, tokenHash string) (domain.AuthToken, bool, error) {
	var t domain.AuthToken
	var purpose string
	var used sql.NullTime
	err := s.q.QueryRowContext(ctx, "SELECT id, token_hash, user_id, purpose, created_at, expires_at, used_at FROM auth_tokens WHERE token_hash = ? FOR UPDATE", tokenHash).
		Scan(&t.ID, &t.TokenHash, &t.UserID, &purpose, &t.CreatedAt, &t.ExpiresAt, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return t, false, nil
	}
	if err != nil {
		return t, false, err
	}
	t.Purpose = domain.TokenPurpose(purpose)
	if used.Valid {
		t.UsedAt = used.Time
	}
	return t, true, nil
}

func (s *Store) UseUserAuthTokens(ctx context.Context, userID domain.UserID, purposes []domain.TokenPurpose, at time.Time) error {
	for _, p := range purposes {
		if _, err := s.q.ExecContext(ctx, "UPDATE auth_tokens SET used_at = ? WHERE user_id = ? AND purpose = ? AND used_at IS NULL", at, userID, string(p)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID domain.UserID) error {
	_, err := s.q.ExecContext(ctx, "DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

func (s *Store) CountRateEvents(ctx context.Context, bucket string, since time.Time) (int, error) {
	var n int
	err := s.q.QueryRowContext(ctx, "SELECT COUNT(*) FROM rate_events WHERE bucket_key = ? AND occurred_at > ?", bucket, since).Scan(&n)
	return n, err
}

func (s *Store) AddRateEvent(ctx context.Context, bucket string, at time.Time) error {
	_, err := s.q.ExecContext(ctx, "INSERT INTO rate_events (bucket_key, occurred_at) VALUES (?, ?)", bucket, at)
	return err
}

func (s *Store) PruneRateEvents(ctx context.Context, before time.Time) error {
	_, err := s.q.ExecContext(ctx, "DELETE FROM rate_events WHERE occurred_at <= ?", before)
	return err
}

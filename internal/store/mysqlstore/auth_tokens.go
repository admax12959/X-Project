package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"hash/fnv"
	"sort"
	"strings"
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

// rateLockSlots is the number of rows in rate_locks (migration 20261015180000).
const rateLockSlots = 64

// LockRateKeys takes the row locks of the keys' slots, in slot order whatever the order of the
// keys, so that two requests locking overlapping keys cannot deadlock. The locks last until the
// transaction ends. Call it before the first plain read of the transaction: under REPEATABLE
// READ the read view is made by that read, and it has to be made after the lock is won.
func (s *Store) LockRateKeys(ctx context.Context, keys ...string) error {
	seen := map[int]bool{}
	var slots []int
	for _, k := range keys {
		h := fnv.New32a()
		_, _ = h.Write([]byte(k))
		slot := int(h.Sum32() % rateLockSlots)
		if !seen[slot] {
			seen[slot] = true
			slots = append(slots, slot)
		}
	}
	sort.Ints(slots)
	for _, slot := range slots {
		var got int
		if err := s.q.QueryRowContext(ctx, "SELECT slot FROM rate_locks WHERE slot = ? FOR UPDATE", slot).Scan(&got); err != nil {
			return err
		}
	}
	return nil
}

// PruneRateEvents deletes the old events without a range lock: it reads their ids (a plain
// read, no locks) and deletes them by primary key in ascending order. A DELETE ... WHERE
// occurred_at <= ? would lock the whole table and its gaps, and two requests doing that while
// the table is nearly empty deadlock on their own inserts. Call it outside any transaction that
// holds rate locks.
func (s *Store) PruneRateEvents(ctx context.Context, before time.Time) error {
	rows, err := s.q.QueryContext(ctx, "SELECT id FROM rate_events WHERE occurred_at <= ? ORDER BY id", before)
	if err != nil {
		return err
	}
	var ids []any
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	const chunk = 500
	for len(ids) > 0 {
		n := min(chunk, len(ids))
		marks := strings.TrimSuffix(strings.Repeat("?,", n), ",")
		if _, err := s.q.ExecContext(ctx, "DELETE FROM rate_events WHERE id IN ("+marks+")", ids[:n]...); err != nil {
			return err
		}
		ids = ids[n:]
	}
	return nil
}

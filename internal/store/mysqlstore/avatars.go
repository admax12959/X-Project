package mysqlstore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/admax12959/X-Project/internal/domain"
)

// SetUserAvatar locks the user's row, reads the current avatar and replaces it (an empty key
// clears it). It does nothing and reports applied=false when the user has an uploaded avatar
// and replaceUpload is false (the LDAP photo never overwrites an upload), or when the user
// does not exist. previous is the object key that was replaced (empty when there was none).
func (s *Store) SetUserAvatar(ctx context.Context, id domain.UserID, key, hash string, source domain.AvatarSource, replaceUpload bool) (previous string, applied bool, err error) {
	var cur, curSource sql.NullString
	err = s.q.QueryRowContext(ctx, "SELECT avatar_key, avatar_source FROM users WHERE id = ? FOR UPDATE", id).Scan(&cur, &curSource)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !replaceUpload && domain.AvatarSource(curSource.String) == domain.AvatarUpload {
		return "", false, nil
	}
	var k, h, src any // NULL when cleared
	if key != "" {
		k, h, src = key, hash, string(source)
	}
	if _, err := s.q.ExecContext(ctx, "UPDATE users SET avatar_key = ?, avatar_hash = ?, avatar_source = ? WHERE id = ?", k, h, src, id); err != nil {
		return "", false, err
	}
	return cur.String, true, nil
}

// AvatarKeysInUse returns the object keys of the avatars users currently have.
func (s *Store) AvatarKeysInUse(ctx context.Context) (map[string]bool, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT avatar_key FROM users WHERE avatar_key IS NOT NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out[k] = true
	}
	return out, rows.Err()
}

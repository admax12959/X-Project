package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.SystemConfigRepository = (*Store)(nil)

func (s *Store) GetConfigSection(ctx context.Context, section string) ([]byte, time.Time, bool, error) {
	var data []byte
	var at time.Time
	err := s.q.QueryRowContext(ctx, "SELECT data, updated_at FROM system_config WHERE section = ?", section).Scan(&data, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, time.Time{}, false, nil
	}
	return data, at, err == nil, err
}

func (s *Store) PutConfigSection(ctx context.Context, section string, data []byte, at time.Time) error {
	_, err := s.q.ExecContext(ctx,
		"INSERT INTO system_config (section, data, updated_at) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE data = VALUES(data), updated_at = VALUES(updated_at)",
		section, string(data), at) // string, not []byte: MySQL rejects binary into JSON
	return err
}

func (s *Store) InsertConfigSectionIfAbsent(ctx context.Context, section string, data []byte, at time.Time) (bool, error) {
	res, err := s.q.ExecContext(ctx,
		"INSERT IGNORE INTO system_config (section, data, updated_at) VALUES (?, ?, ?)", section, string(data), at)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) ListConfigSecrets(ctx context.Context, section string) (map[string]domain.StoredSecret, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT field_name, value_enc, key_version FROM system_config_secrets WHERE section = ?", section)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]domain.StoredSecret{}
	for rows.Next() {
		var field string
		var sec domain.StoredSecret
		if err := rows.Scan(&field, &sec.Enc, &sec.KeyVersion); err != nil {
			return nil, err
		}
		out[field] = sec
	}
	return out, rows.Err()
}

func (s *Store) PutConfigSecret(ctx context.Context, section, field string, sec domain.StoredSecret, at time.Time) error {
	_, err := s.q.ExecContext(ctx,
		"INSERT INTO system_config_secrets (section, field_name, value_enc, key_version, updated_at) VALUES (?, ?, ?, ?, ?) "+
			"ON DUPLICATE KEY UPDATE value_enc = VALUES(value_enc), key_version = VALUES(key_version), updated_at = VALUES(updated_at)",
		section, field, sec.Enc, sec.KeyVersion, at)
	return err
}

func (s *Store) DeleteConfigSecret(ctx context.Context, section, field string) error {
	_, err := s.q.ExecContext(ctx, "DELETE FROM system_config_secrets WHERE section = ? AND field_name = ?", section, field)
	return err
}

package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.SecondFactorRepository = (*Store)(nil)

func (s *Store) ListAuthenticators(ctx context.Context, userID domain.UserID) ([]domain.Authenticator, error) {
	rows, err := s.q.QueryContext(ctx,
		"SELECT id, user_id, type, status, secret_enc, key_version, last_used_step, created_at, enabled_at FROM user_authenticators WHERE user_id = ? ORDER BY id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Authenticator
	for rows.Next() {
		var a domain.Authenticator
		var typ, status string
		var enabledAt sql.NullTime
		if err := rows.Scan(&a.ID, &a.UserID, &typ, &status, &a.SecretEnc, &a.KeyVersion, &a.LastUsedStep, &a.CreatedAt, &enabledAt); err != nil {
			return nil, err
		}
		a.Type, a.Enabled, a.EnabledAt = domain.AuthenticatorType(typ), status == "enabled", enabledAt.Time
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) ReplacePendingAuthenticator(ctx context.Context, a domain.Authenticator) error {
	if _, err := s.q.ExecContext(ctx, "DELETE FROM user_authenticators WHERE user_id = ? AND type = ? AND status = 'pending'", a.UserID, string(a.Type)); err != nil {
		return err
	}
	_, err := s.q.ExecContext(ctx,
		"INSERT INTO user_authenticators (user_id, type, status, secret_enc, key_version, created_at) VALUES (?, ?, 'pending', ?, ?, ?)",
		a.UserID, string(a.Type), a.SecretEnc, a.KeyVersion, a.CreatedAt)
	return err
}

func (s *Store) EnableAuthenticator(ctx context.Context, id, step int64, at time.Time) (bool, error) {
	res, err := s.q.ExecContext(ctx,
		"UPDATE user_authenticators SET status = 'enabled', enabled_at = ?, last_used_step = ? WHERE id = ? AND status = 'pending'", at, step, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) ClaimAuthenticatorStep(ctx context.Context, id, step int64) (bool, error) {
	res, err := s.q.ExecContext(ctx,
		"UPDATE user_authenticators SET last_used_step = ? WHERE id = ? AND status = 'enabled' AND last_used_step < ?", step, id, step)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) GetSecondFactorLockout(ctx context.Context, userID domain.UserID) (domain.SecondFactorLockout, error) {
	var l domain.SecondFactorLockout
	var until sql.NullTime
	err := s.q.QueryRowContext(ctx, "SELECT failures, locked_until FROM second_factor_lockouts WHERE user_id = ? FOR UPDATE", userID).Scan(&l.Failures, &until)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SecondFactorLockout{}, nil
	}
	l.LockedUntil = until.Time
	return l, err
}

func (s *Store) PutSecondFactorLockout(ctx context.Context, userID domain.UserID, l domain.SecondFactorLockout) error {
	var until any // NULL when not locked
	if !l.LockedUntil.IsZero() {
		until = l.LockedUntil
	}
	_, err := s.q.ExecContext(ctx,
		"INSERT INTO second_factor_lockouts (user_id, failures, locked_until) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE failures = VALUES(failures), locked_until = VALUES(locked_until)",
		userID, l.Failures, until)
	return err
}

func (s *Store) CreateLoginChallenge(ctx context.Context, c domain.LoginChallenge) error {
	_, err := s.q.ExecContext(ctx,
		"INSERT INTO login_challenges (token_hash, user_id, must_change_password, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		c.TokenHash, c.UserID, c.MustChangePassword, c.CreatedAt, c.ExpiresAt)
	return err
}

func (s *Store) FindLoginChallenge(ctx context.Context, tokenHash string) (domain.LoginChallenge, bool, error) {
	var c domain.LoginChallenge
	err := s.q.QueryRowContext(ctx,
		"SELECT token_hash, user_id, must_change_password, created_at, expires_at FROM login_challenges WHERE token_hash = ?", tokenHash).
		Scan(&c.TokenHash, &c.UserID, &c.MustChangePassword, &c.CreatedAt, &c.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, false, nil
	}
	return c, err == nil, err
}

func (s *Store) DeleteLoginChallenge(ctx context.Context, tokenHash string) error {
	_, err := s.q.ExecContext(ctx, "DELETE FROM login_challenges WHERE token_hash = ?", tokenHash)
	return err
}

func (s *Store) DeleteExpiredLoginChallenges(ctx context.Context, at time.Time) error {
	_, err := s.q.ExecContext(ctx, "DELETE FROM login_challenges WHERE expires_at < ?", at)
	return err
}

// ---------------------------------------------------------------- recovery codes (#31)

func (s *Store) ReplaceRecoveryCodes(ctx context.Context, userID domain.UserID, hashes []string, at time.Time) error {
	if _, err := s.q.ExecContext(ctx, "DELETE FROM user_recovery_codes WHERE user_id = ?", userID); err != nil {
		return err
	}
	for _, h := range hashes {
		if _, err := s.q.ExecContext(ctx, "INSERT INTO user_recovery_codes (user_id, code_hash, created_at) VALUES (?, ?, ?)", userID, h, at); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListUnusedRecoveryCodes(ctx context.Context, userID domain.UserID) ([]domain.RecoveryCode, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT id, code_hash FROM user_recovery_codes WHERE user_id = ? AND used_at IS NULL ORDER BY id", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.RecoveryCode
	for rows.Next() {
		var c domain.RecoveryCode
		if err := rows.Scan(&c.ID, &c.Hash); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) ClaimRecoveryCode(ctx context.Context, id int64, at time.Time) (bool, error) {
	res, err := s.q.ExecContext(ctx, "UPDATE user_recovery_codes SET used_at = ? WHERE id = ? AND used_at IS NULL", at, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) DeleteAuthenticatorsAndRecoveryCodes(ctx context.Context, userID domain.UserID) error {
	if _, err := s.q.ExecContext(ctx, "DELETE FROM user_recovery_codes WHERE user_id = ?", userID); err != nil {
		return err
	}
	_, err := s.q.ExecContext(ctx, "DELETE FROM user_authenticators WHERE user_id = ?", userID)
	return err
}

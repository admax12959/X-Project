package mysqlstore

import (
	"context"
	"database/sql"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.MailOutboxRepository = (*Store)(nil)

const mailCols = "id, to_address, recipient_user_id, template, lang, subject, body_text, body_html, status, attempts, next_attempt_at, last_error_code, last_error, created_at, sent_at, failed_at"

func scanMail(rows *sql.Rows) (domain.MailEntry, error) {
	var e domain.MailEntry
	var recipient sql.NullInt64
	var sent, failed sql.NullTime
	var status string
	err := rows.Scan(&e.ID, &e.To, &recipient, &e.Template, &e.Lang, &e.Subject, &e.Text, &e.HTML, &status, &e.Attempts,
		&e.NextAttemptAt, &e.LastErrorCode, &e.LastError, &e.CreatedAt, &sent, &failed)
	e.Status = domain.MailStatus(status)
	e.Recipient = domain.UserID(recipient.Int64)
	e.SentAt, e.FailedAt = sent.Time, failed.Time
	return e, err
}

func (s *Store) InsertMail(ctx context.Context, e domain.MailEntry) (int64, error) {
	var recipient any
	if e.Recipient != 0 {
		recipient = int64(e.Recipient)
	}
	res, err := s.q.ExecContext(ctx,
		"INSERT INTO mail_outbox (to_address, recipient_user_id, template, lang, subject, body_text, body_html, status, attempts, next_attempt_at, created_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)",
		e.To, recipient, e.Template, e.Lang, e.Subject, e.Text, e.HTML, string(domain.MailPending), e.NextAttemptAt, e.CreatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ClaimDueMail picks pending mail whose time has come and that no other worker holds, and
// leases it to the caller. SKIP LOCKED keeps concurrent workers from waiting for each other.
func (s *Store) ClaimDueMail(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.MailEntry, error) {
	var out []domain.MailEntry
	err := s.InTx(ctx, func(tx domain.Repository) error {
		q := tx.(*Store).q
		rows, err := q.QueryContext(ctx,
			"SELECT "+mailCols+" FROM mail_outbox WHERE status = ? AND next_attempt_at <= ? AND (locked_until IS NULL OR locked_until <= ?) "+
				"ORDER BY id LIMIT ? FOR UPDATE SKIP LOCKED",
			string(domain.MailPending), now, now, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			e, err := scanMail(rows)
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		for _, e := range out {
			if _, err := q.ExecContext(ctx, "UPDATE mail_outbox SET locked_until = ? WHERE id = ?", now.Add(lease), e.ID); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

func (s *Store) MarkMailSent(ctx context.Context, id int64, attempts int, at time.Time) error {
	_, err := s.q.ExecContext(ctx,
		"UPDATE mail_outbox SET status = ?, attempts = ?, sent_at = ?, locked_until = NULL, body_text = '', body_html = '', last_error_code = '', last_error = '' WHERE id = ?",
		string(domain.MailSent), attempts, at, id)
	return err
}

func (s *Store) MarkMailRetry(ctx context.Context, id int64, attempts int, next time.Time, code, detail string) error {
	_, err := s.q.ExecContext(ctx,
		"UPDATE mail_outbox SET attempts = ?, next_attempt_at = ?, locked_until = NULL, last_error_code = ?, last_error = ? WHERE id = ?",
		attempts, next, code, detail, id)
	return err
}

func (s *Store) MarkMailFailed(ctx context.Context, id int64, attempts int, at time.Time, code, detail string) error {
	_, err := s.q.ExecContext(ctx,
		"UPDATE mail_outbox SET status = ?, attempts = ?, failed_at = ?, locked_until = NULL, body_text = '', body_html = '', last_error_code = ?, last_error = ? WHERE id = ?",
		string(domain.MailFailed), attempts, at, code, detail, id)
	return err
}

func (s *Store) ListMail(ctx context.Context, status domain.MailStatus, limit int) ([]domain.MailEntry, error) {
	query := "SELECT " + mailCols + " FROM mail_outbox"
	var args []any
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, string(status))
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.MailEntry
	for rows.Next() {
		e, err := scanMail(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) CountMail(ctx context.Context) (map[domain.MailStatus]int, error) {
	rows, err := s.q.QueryContext(ctx, "SELECT status, COUNT(*) FROM mail_outbox GROUP BY status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[domain.MailStatus]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[domain.MailStatus(status)] = n
	}
	return out, rows.Err()
}

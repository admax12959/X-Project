package mysqlstore

import (
	"context"
	"database/sql"
	"strings"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.AuditRepository = (*Store)(nil)

const (
	actorKindUser = "user"
	actorKindCLI  = "cli"
)

func (s *Store) AppendAudit(ctx context.Context, e domain.StoredAuditEntry) error {
	kind, actor := actorKindUser, sql.NullInt64{Int64: int64(e.ActorID), Valid: true}
	if e.ActorCLI {
		kind, actor = actorKindCLI, sql.NullInt64{}
	}
	_, err := s.q.ExecContext(ctx,
		"INSERT INTO audit_log (occurred_at, actor_kind, actor_user_id, action, target_type, target_id, params) VALUES (?, ?, ?, ?, ?, ?, ?)",
		e.At, kind, actor, string(e.Action), string(e.TargetType), e.TargetID, string(e.Params)) // string, not []byte: MySQL rejects binary into JSON
	return err
}

func (s *Store) ListAudit(ctx context.Context, f domain.AuditFilter, offset, limit int) ([]domain.StoredAuditEntry, int, error) {
	// One WHERE clause for both the count and the page.
	var where []string
	var args []any
	if f.ActorCLI {
		where = append(where, "actor_kind = ?")
		args = append(args, actorKindCLI)
	}
	if f.ActorID != 0 {
		where = append(where, "actor_user_id = ?")
		args = append(args, f.ActorID)
	}
	if f.TargetType != "" {
		where = append(where, "target_type = ?")
		args = append(args, string(f.TargetType))
	}
	if f.TargetID != "" {
		where = append(where, "target_id = ?")
		args = append(args, f.TargetID)
	}
	if f.Action != "" {
		where = append(where, "action = ?")
		args = append(args, string(f.Action))
	}
	if !f.From.IsZero() {
		where = append(where, "occurred_at >= ?")
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		where = append(where, "occurred_at <= ?")
		args = append(args, f.To)
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := s.q.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log"+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.q.QueryContext(ctx,
		"SELECT id, occurred_at, actor_kind, actor_user_id, action, target_type, target_id, params FROM audit_log"+cond+
			" ORDER BY occurred_at DESC, id DESC LIMIT ? OFFSET ?",
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []domain.StoredAuditEntry{}
	for rows.Next() {
		var e domain.StoredAuditEntry
		var kind, action, targetType string
		var actor sql.NullInt64
		if err := rows.Scan(&e.ID, &e.At, &kind, &actor, &action, &targetType, &e.TargetID, &e.Params); err != nil {
			return nil, 0, err
		}
		e.ActorCLI, e.ActorID = kind == actorKindCLI, domain.UserID(actor.Int64)
		e.Action, e.TargetType = domain.AuditAction(action), domain.AuditTargetType(targetType)
		out = append(out, e)
	}
	return out, total, rows.Err()
}

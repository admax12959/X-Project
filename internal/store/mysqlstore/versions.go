package mysqlstore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.VersionRepository = (*Store)(nil)

func (s *Store) LockLine(ctx context.Context, applicationID, lineID int64) (domain.Line, bool, error) {
	// The whole line, status included: the lifecycle rules (checkLineAcceptsVersion) read it.
	l, err := scanLine(s.q.QueryRowContext(ctx, lineSelect+" WHERE l.application_id = ? AND l.id = ? FOR UPDATE OF l", applicationID, lineID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Line{}, false, nil
	}
	return l, err == nil, err
}

func (s *Store) CreateVersion(ctx context.Context, v domain.NewVersion) (int64, error) {
	res, err := s.q.ExecContext(ctx,
		"INSERT INTO app_versions (application_id, line_id, number, ref_kind, ref_value, released_at, note, created_at, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		v.ApplicationID, v.LineID, v.Number, string(v.RefKind), v.RefValue, v.ReleasedAt, v.Note, v.CreatedAt, v.CreatedBy)
	if isDuplicate(err) {
		return 0, domain.ErrNumberTaken
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

const versionSelect = `SELECT v.id, v.application_id, v.line_id, v.number, v.ref_kind, v.ref_value, v.released_at, v.note, v.created_at,
  l.id, l.is_main, l.name, l.branch, l.created_at
FROM app_versions v JOIN app_lines l ON l.id = v.line_id`

func scanVersion(row interface{ Scan(...any) error }) (domain.Version, error) {
	var v domain.Version
	var kind string
	err := row.Scan(&v.ID, &v.ApplicationID, &v.LineID, &v.Number, &kind, &v.RefValue, &v.ReleasedAt, &v.Note, &v.CreatedAt,
		&v.Line.ID, &v.Line.IsMain, &v.Line.Name, &v.Line.Branch, &v.Line.CreatedAt)
	v.RefKind = domain.RefKind(kind)
	return v, err
}

func (s *Store) FindVersionByNumberForShare(ctx context.Context, applicationID int64, number string) (domain.Version, bool, error) {
	v, err := scanVersion(s.q.QueryRowContext(ctx, versionSelect+" WHERE v.application_id = ? AND v.number = ? FOR SHARE", applicationID, number))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Version{}, false, nil
	}
	return v, err == nil, err
}

func (s *Store) ListVersions(ctx context.Context, applicationID int64) ([]domain.Version, error) {
	rows, err := s.q.QueryContext(ctx, versionSelect+" WHERE v.application_id = ? ORDER BY v.released_at DESC, v.id DESC", applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Version{}
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

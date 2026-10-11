package mysqlstore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.LineRepository = (*Store)(nil)

// lineSelect reads a line with its fork facts and its source version (and the line
// that holds it, short form). Locking reads on it must say FOR UPDATE OF l so the
// source version's rows are not locked.
const lineSelect = `SELECT l.id, l.is_main, l.name, l.branch, l.created_at, l.status, l.reason, l.planned_end, l.note, l.forked_at,
  sv.id, sv.number, sv.released_at, sl.id, sl.is_main, sl.name, sl.branch, sl.created_at
FROM app_lines l
LEFT JOIN app_versions sv ON sv.id = l.source_version_id
LEFT JOIN app_lines sl ON sl.id = sv.line_id`

func scanLine(row interface{ Scan(...any) error }) (domain.Line, error) {
	var l domain.Line
	var status, reason, note sql.NullString
	var plannedEnd, forkedAt sql.NullTime
	var svID, slID sql.NullInt64
	var svNumber, slName, slBranch sql.NullString
	var svReleased, slCreated sql.NullTime
	var slMain sql.NullBool
	if err := row.Scan(&l.ID, &l.IsMain, &l.Name, &l.Branch, &l.CreatedAt, &status, &reason, &plannedEnd, &note, &forkedAt,
		&svID, &svNumber, &svReleased, &slID, &slMain, &slName, &slBranch, &slCreated); err != nil {
		return l, err
	}
	l.Status, l.Reason, l.Note = domain.LineStatus(status.String), reason.String, note.String
	if plannedEnd.Valid {
		l.PlannedEnd = &plannedEnd.Time
	}
	if forkedAt.Valid {
		l.ForkedAt = &forkedAt.Time
	}
	if svID.Valid {
		l.SourceVersion = &domain.SourceVersion{
			ID: svID.Int64, Number: svNumber.String, ReleasedAt: svReleased.Time,
			Line: domain.Line{ID: slID.Int64, IsMain: slMain.Bool, Name: slName.String, Branch: slBranch.String, CreatedAt: slCreated.Time},
		}
	}
	return l, nil
}

func (s *Store) CreateLine(ctx context.Context, l domain.NewLine) (int64, error) {
	var plannedEnd any
	if l.PlannedEnd != nil {
		plannedEnd = *l.PlannedEnd
	}
	res, err := s.q.ExecContext(ctx,
		"INSERT INTO app_lines (application_id, is_main, name, branch, created_at, source_version_id, reason, status, planned_end, note, forked_at) VALUES (?, FALSE, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		l.ApplicationID, l.Name, l.Branch, l.ForkedAt, l.SourceVersionID, l.Reason, string(domain.LineActive), plannedEnd, l.Note, l.ForkedAt)
	if isDuplicate(err) {
		return 0, domain.ErrBranchTaken
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) FindLineByBranch(ctx context.Context, applicationID int64, branch string) (domain.Line, bool, error) {
	l, err := scanLine(s.q.QueryRowContext(ctx, lineSelect+" WHERE l.application_id = ? AND l.branch = ? FOR SHARE OF l", applicationID, branch))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Line{}, false, nil
	}
	return l, err == nil, err
}

func (s *Store) GetVersion(ctx context.Context, applicationID, versionID int64) (domain.Version, bool, error) {
	v, err := scanVersion(s.q.QueryRowContext(ctx, versionSelect+" WHERE v.application_id = ? AND v.id = ?", applicationID, versionID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Version{}, false, nil
	}
	return v, err == nil, err
}

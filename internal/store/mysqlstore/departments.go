package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/admax12959/X-Project/internal/domain"
)

var _ domain.DepartmentRepository = (*Store)(nil)

// MySQL error numbers for a foreign key that refuses a delete (a row still refers to it)
// and one that refuses an insert or update (the referenced row does not exist).
const (
	errFKParentRowReferenced = 1451
	errFKChildRowNoParent    = 1452
)

// mapDepartmentFK turns foreign key violations on the department tree into domain errors.
func mapDepartmentFK(err error) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		switch me.Number {
		case errFKParentRowReferenced:
			return domain.ErrDepartmentInUse
		case errFKChildRowNoParent:
			return domain.ErrDepartmentMissing
		}
	}
	return err
}

// departmentSelect lists a department with its direct member and child counts.
const departmentSelect = `SELECT d.id, d.parent_id, d.name, d.source, d.external_id, d.created_at,
  (SELECT COUNT(*) FROM users u WHERE u.department_id = d.id),
  (SELECT COUNT(*) FROM departments c WHERE c.parent_id = d.id)
  FROM departments d`

func scanDepartment(row interface{ Scan(...any) error }) (domain.Department, error) {
	var d domain.Department
	var parent sql.NullInt64
	var external sql.NullString
	err := row.Scan(&d.ID, &parent, &d.Name, &d.Source, &external, &d.CreatedAt, &d.MemberCount, &d.ChildCount)
	d.ParentID, d.ExternalID = domain.DepartmentID(parent.Int64), external.String
	return d, err
}

func (s *Store) ListDepartments(ctx context.Context) ([]domain.Department, error) {
	rows, err := s.q.QueryContext(ctx, departmentSelect+" ORDER BY d.id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Department{}
	for rows.Next() {
		d, err := scanDepartment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDepartment(ctx context.Context, id domain.DepartmentID) (domain.Department, bool, error) {
	d, err := scanDepartment(s.q.QueryRowContext(ctx, departmentSelect+" WHERE d.id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Department{}, false, nil
	}
	return d, err == nil, err
}

// nullableDepartment is the column value of a department reference: 0 is NULL.
func nullableDepartment(id domain.DepartmentID) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(id), Valid: id != 0}
}

func (s *Store) CreateDepartment(ctx context.Context, d domain.Department) (domain.DepartmentID, error) {
	var external sql.NullString
	if d.ExternalID != "" {
		external = sql.NullString{String: d.ExternalID, Valid: true}
	}
	res, err := s.q.ExecContext(ctx,
		"INSERT INTO departments (parent_id, name, source, external_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		nullableDepartment(d.ParentID), d.Name, d.Source, external, d.CreatedAt, d.CreatedAt)
	if err != nil {
		return 0, mapDepartmentFK(err)
	}
	id, err := res.LastInsertId()
	return domain.DepartmentID(id), err
}

func (s *Store) UpdateDepartment(ctx context.Context, id domain.DepartmentID, name string, parent domain.DepartmentID, at time.Time) error {
	_, err := s.q.ExecContext(ctx, "UPDATE departments SET name = ?, parent_id = ?, updated_at = ? WHERE id = ?",
		name, nullableDepartment(parent), at, id)
	return mapDepartmentFK(err)
}

func (s *Store) DeleteDepartment(ctx context.Context, id domain.DepartmentID) error {
	_, err := s.q.ExecContext(ctx, "DELETE FROM departments WHERE id = ?", id)
	return mapDepartmentFK(err)
}

func (s *Store) LockDepartmentTree(ctx context.Context) error {
	var one int
	return s.q.QueryRowContext(ctx, "SELECT id FROM department_tree_lock WHERE id = 1 FOR UPDATE").Scan(&one)
}

func (s *Store) DepartmentSubtree(ctx context.Context, id domain.DepartmentID) ([]domain.DepartmentID, error) {
	rows, err := s.q.QueryContext(ctx, `WITH RECURSIVE tree (id) AS (
  SELECT id FROM departments WHERE id = ?
  UNION ALL
  SELECT d.id FROM departments d JOIN tree t ON d.parent_id = t.id
) SELECT id FROM tree ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.DepartmentID
	for rows.Next() {
		var d domain.DepartmentID
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

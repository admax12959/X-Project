package mysqlstore

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate applies every migration file not yet recorded in schema_migrations,
// in lexical (= timestamp) order. Each file is recorded by name, so a file merged
// later with an older timestamp is still applied.
//
// File rules (docs/dev.md): name YYYYMMDDHHMMSS_description.sql; statements end
// with ";" at end of line; no ";" inside a statement; "--" comment lines only.
// MySQL DDL is not transactional, so keep each file small.
func Migrate(ctx context.Context, db *sql.DB) error { return migrate(ctx, db, "") }

// MigrateBefore applies the migrations that sort before the file named stopAt, for tests that
// need the schema as it was before that file.
func MigrateBefore(ctx context.Context, db *sql.DB, stopAt string) error {
	return migrate(ctx, db, stopAt)
}

func migrate(ctx context.Context, db *sql.DB, stopAt string) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Serialise concurrent starts.
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK('xproject_migrate', 120)").Scan(&got); err != nil {
		return fmt.Errorf("migrate lock: %w", err)
	}
	if !got.Valid || got.Int64 != 1 {
		return fmt.Errorf("migrate lock: timeout")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('xproject_migrate')")

	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name VARCHAR(255) NOT NULL PRIMARY KEY,
		applied_at DATETIME(6) NOT NULL
	) ENGINE=InnoDB`); err != nil {
		return err
	}
	applied := map[string]bool{}
	rows, err := conn.QueryContext(ctx, "SELECT name FROM schema_migrations")
	if err != nil {
		return err
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return err
		}
		applied[n] = true
	}
	rows.Close()

	names, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, path := range names {
		name := strings.TrimPrefix(path, "migrations/")
		if applied[name] {
			continue
		}
		if stopAt != "" && name >= stopAt {
			break
		}
		body, err := migrationFS.ReadFile(path)
		if err != nil {
			return err
		}
		for i, stmt := range splitStatements(string(body)) {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migration %s statement %d: %w", name, i+1, err)
			}
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)", name, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func splitStatements(body string) []string {
	var out []string
	var cur strings.Builder
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		cur.WriteString(line)
		cur.WriteString("\n")
		if strings.HasSuffix(t, ";") {
			out = append(out, strings.TrimSuffix(strings.TrimSpace(cur.String()), ";"))
			cur.Reset()
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		out = append(out, s)
	}
	return out
}

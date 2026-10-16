package api

import (
	"context"
	"database/sql"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/store/mysqlstore"
)

const emailIdentityMigration = "20261015100000_email_identity.sql"

// seedLocalUsers puts users and their local identities into the schema as it was before #29.
func seedLocalUsers(t *testing.T, db *sql.DB, users [][2]string) {
	t.Helper()
	for _, u := range users {
		res, err := db.Exec("INSERT INTO users (username, display_name, email, status, created_at, updated_at) VALUES (?, ?, ?, 'enabled', NOW(6), NOW(6))", u[0], u[0], u[1])
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		if _, err := db.Exec("INSERT INTO login_identities (user_id, kind, login_name, password_hash, bound_at, bound_via) VALUES (?, 'local', ?, 'h', NOW(6), 'admin')", id, u[0]); err != nil {
			t.Fatal(err)
		}
	}
}

func loginNames(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.Query("SELECT u.username, li.login_name FROM login_identities li JOIN users u ON u.id = li.user_id WHERE li.kind = 'email'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var u, l string
		if err := rows.Scan(&u, &l); err != nil {
			t.Fatal(err)
		}
		got[u] = l
	}
	return got
}

// A contact email in the placeholder domain used to collide with another user's placeholder and
// leave the migration half applied.
func TestTheEmailIdentityMigrationSurvivesAContactEmailThatEqualsAnotherUsersPlaceholder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := sql.Open("mysql", apitest.FreshDatabase(t)+"?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := mysqlstore.MigrateBefore(ctx, db, emailIdentityMigration); err != nil {
		t.Fatal(err)
	}
	seedLocalUsers(t, db, [][2]string{
		{"a", "b@x-project.invalid"}, // equals b's placeholder
		{"b", ""},
		{"c", "shared@corp.example"}, {"d", "SHARED@corp.example"}, // two holders: both get placeholders
		{"e", "Eve@corp.example"}, // unique: its (lower-cased) address
	})

	// The ALTERs of a first, failed attempt are already in place: the file must still run.
	for _, ddl := range []string{
		"ALTER TABLE login_identities ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT FALSE",
		"ALTER TABLE sessions ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT FALSE",
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	if err := mysqlstore.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"a": "a@x-project.invalid", "b": "b@x-project.invalid",
		"c": "c@x-project.invalid", "d": "d@x-project.invalid",
		"e": "eve@corp.example",
	}
	got := loginNames(t, db)
	for u, l := range want {
		if got[u] != l {
			t.Errorf("login email of %s = %q, want %q", u, got[u], l)
		}
	}
}

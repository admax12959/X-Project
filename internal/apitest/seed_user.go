package apitest

import (
	"database/sql"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

// SeedUserClient inserts an enabled, non-administrator user with a local login
// identity straight into the database and returns a client logged in as that user.
//
// It exists so permission tests can have a plain user before the user-management
// API exists. Once a tracer API for creating users is available, prefer it.
func (e *Env) SeedUserClient(username, password string) *Client {
	e.t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		e.t.Fatal(err)
	}
	db, err := sql.Open("mysql", e.DSN)
	if err != nil {
		e.t.Fatal(err)
	}
	defer db.Close()
	res, err := db.Exec(
		"INSERT INTO users (username, display_name, status, is_admin, created_at, updated_at) VALUES (?, ?, 'enabled', FALSE, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6))",
		username, username)
	if err != nil {
		e.t.Fatalf("seed user: %v", err)
	}
	id, _ := res.LastInsertId()
	if _, err := db.Exec(
		"INSERT INTO login_identities (user_id, kind, login_name, password_hash, bound_at, bound_via) VALUES (?, 'local', ?, ?, UTC_TIMESTAMP(6), 'bootstrap')",
		id, username, string(hash)); err != nil {
		e.t.Fatalf("seed identity: %v", err)
	}
	c := e.Client()
	c.LoginLocal(username, password).ExpectStatus(http.StatusOK)
	return c
}

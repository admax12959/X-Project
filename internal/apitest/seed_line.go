package apitest

import "database/sql"

// SeedLineStatus puts the maintenance line with the given Git branch in the given
// state ("active", "frozen" or "retired") by writing the database directly.
//
// TEST ONLY, AND IT BYPASSES THE STATE MACHINE: nothing is validated, nothing is
// recorded in the change history, no status-change log entry is written. The lifecycle
// API (PUT /lines/{id}/status, #10) exists: tests of lifecycle behaviour must use it. This
// helper stays for fixtures such as the seed scenarios (a retired line that
// already holds versions).
func (e *Env) SeedLineStatus(appKey, branch, status string) {
	e.t.Helper()
	db, err := sql.Open("mysql", e.DSN)
	if err != nil {
		e.t.Fatal(err)
	}
	defer db.Close()
	res, err := db.Exec(
		"UPDATE app_lines l JOIN applications a ON a.id = l.application_id SET l.status = ? WHERE a.app_key = ? AND l.branch = ? AND NOT l.is_main",
		status, appKey, branch)
	if err != nil {
		e.t.Fatalf("seed line status: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		e.t.Fatalf("seed line status: %d maintenance lines matched %s/%s, want 1", n, appKey, branch)
	}
}

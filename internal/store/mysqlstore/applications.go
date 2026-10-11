package mysqlstore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/admax12959/X-Project/internal/domain"
)

var (
	_ domain.ApplicationRepository = (*Store)(nil)
	_ domain.HistoryRepository     = (*Store)(nil)
)

const mysqlDuplicateEntry = 1062

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == mysqlDuplicateEntry
}

func (s *Store) CreateApplication(ctx context.Context, a domain.NewApplication) (int64, error) {
	res, err := s.q.ExecContext(ctx,
		"INSERT INTO applications (app_key, name, description, platform_id, repo_path, repo_project_id, deploy_info, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		a.Key, a.Name, a.Description, a.PlatformID, a.RepoPath, a.ProjectID, a.DeployInfo, a.At, a.At)
	if isDuplicate(err) {
		return 0, domain.ErrKeyTaken
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) CreateMainline(ctx context.Context, applicationID int64, branch string, at time.Time) error {
	// The mainline has no name of its own: the UI labels it with the glossary term.
	_, err := s.q.ExecContext(ctx,
		"INSERT INTO app_lines (application_id, is_main, name, branch, created_at) VALUES (?, TRUE, '', ?, ?)",
		applicationID, branch, at)
	return err
}

func (s *Store) ListLines(ctx context.Context, applicationID int64) ([]domain.Line, error) {
	rows, err := s.q.QueryContext(ctx, lineSelect+" WHERE l.application_id = ? ORDER BY l.is_main DESC, l.id", applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Line{}
	for rows.Next() {
		l, err := scanLine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) SetMembers(ctx context.Context, applicationID int64, role domain.MemberRole, users []domain.UserID) error {
	if _, err := s.q.ExecContext(ctx, "DELETE FROM application_members WHERE application_id = ? AND role = ?", applicationID, string(role)); err != nil {
		return err
	}
	for i, u := range users {
		if _, err := s.q.ExecContext(ctx,
			"INSERT INTO application_members (application_id, role, user_id, position) VALUES (?, ?, ?, ?)",
			applicationID, string(role), u, i); err != nil {
			return err
		}
	}
	return nil
}

// appSelect joins an application with its platform and mainline branch.
const appSelect = `SELECT a.id, a.app_key, a.name, a.description, a.deploy_info, a.created_at, a.repo_path, a.repo_project_id,
  p.id, p.kind, p.base_url, p.token_tail, p.created_at, l.branch
FROM applications a
JOIN code_platforms p ON p.id = a.platform_id
JOIN app_lines l ON l.application_id = a.id AND l.is_main`

func scanApplication(row interface{ Scan(...any) error }) (domain.Application, error) {
	var a domain.Application
	var kind string
	err := row.Scan(&a.ID, &a.Key, &a.Name, &a.Description, &a.DeployInfo, &a.CreatedAt, &a.Repo.Path, &a.Repo.ProjectID,
		&a.Repo.Platform.ID, &kind, &a.Repo.Platform.BaseURL, &a.Repo.Platform.TokenTail, &a.Repo.Platform.CreatedAt, &a.MainBranch)
	a.Repo.Platform.Kind = domain.PlatformKind(kind)
	return a, err
}

// membersOf loads Owner and PE lists, keyed by application ID. A nil ids loads all.
func (s *Store) membersOf(ctx context.Context, ids []int64) (map[int64]map[domain.MemberRole][]domain.UserRef, error) {
	q := `SELECT m.application_id, m.role, u.id, u.username, u.display_name, u.status
FROM application_members m JOIN users u ON u.id = m.user_id`
	var args []any
	if ids != nil {
		if len(ids) == 0 {
			return map[int64]map[domain.MemberRole][]domain.UserRef{}, nil
		}
		q += " WHERE m.application_id IN (?" + strings.Repeat(",?", len(ids)-1) + ")"
		for _, id := range ids {
			args = append(args, id)
		}
	}
	rows, err := s.q.QueryContext(ctx, q+" ORDER BY m.application_id, m.role, m.position", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]map[domain.MemberRole][]domain.UserRef{}
	for rows.Next() {
		var appID int64
		var role, status string
		var u domain.UserRef
		if err := rows.Scan(&appID, &role, &u.ID, &u.Username, &u.DisplayName, &status); err != nil {
			return nil, err
		}
		u.Status = domain.UserStatus(status)
		if out[appID] == nil {
			out[appID] = map[domain.MemberRole][]domain.UserRef{}
		}
		out[appID][domain.MemberRole(role)] = append(out[appID][domain.MemberRole(role)], u)
	}
	return out, rows.Err()
}

func (s *Store) GetApplicationByKey(ctx context.Context, key string) (domain.Application, bool, error) {
	a, err := scanApplication(s.q.QueryRowContext(ctx, appSelect+" WHERE a.app_key = ?", key))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Application{}, false, nil
	}
	if err != nil {
		return domain.Application{}, false, err
	}
	members, err := s.membersOf(ctx, []int64{a.ID})
	if err != nil {
		return domain.Application{}, false, err
	}
	a.Owners, a.PEs = members[a.ID][domain.RoleOwner], members[a.ID][domain.RolePE]
	return a, true, nil
}

// likePattern turns user text into a literal-substring LIKE pattern (escape char '|').
func likePattern(q string) string {
	r := strings.NewReplacer("|", "||", "%", "|%", "_", "|_")
	return "%" + r.Replace(q) + "%"
}

func (s *Store) ListApplications(ctx context.Context, query string) ([]domain.ApplicationSummary, error) {
	q := appSelect
	var args []any
	if query != "" {
		q += ` WHERE (a.app_key LIKE ? ESCAPE '|' OR a.name LIKE ? ESCAPE '|' OR a.repo_path LIKE ? ESCAPE '|'
  OR EXISTS (SELECT 1 FROM application_members m JOIN users u ON u.id = m.user_id
     WHERE m.application_id = a.id AND m.role = 'owner' AND (u.display_name LIKE ? ESCAPE '|' OR u.username LIKE ? ESCAPE '|')))`
		p := likePattern(query)
		args = []any{p, p, p, p, p}
	}
	rows, err := s.q.QueryContext(ctx, q+" ORDER BY a.app_key", args...)
	if err != nil {
		return nil, err
	}
	var apps []domain.Application
	for rows.Next() {
		a, err := scanApplication(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		apps = append(apps, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ids := make([]int64, len(apps))
	for i, a := range apps {
		ids[i] = a.ID
	}
	members, err := s.membersOf(ctx, ids)
	if err != nil {
		return nil, err
	}
	counts, err := s.lineCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	latest, err := s.latestVersions(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ApplicationSummary, 0, len(apps))
	for _, a := range apps {
		out = append(out, domain.ApplicationSummary{
			Key: a.Key, Name: a.Name, Description: a.Description, Repo: a.Repo, Owners: members[a.ID][domain.RoleOwner],
			LineCount: counts[a.ID], LatestVersion: latest[a.ID],
		})
	}
	return out, nil
}

// ---------------------------------------------------------------- change history

func (s *Store) AppendChangeEvent(ctx context.Context, e domain.StoredChangeEvent) error {
	_, err := s.q.ExecContext(ctx,
		"INSERT INTO change_events (application_id, occurred_at, actor_user_id, event_type, params) VALUES (?, ?, ?, ?, ?)",
		e.ApplicationID, e.At, e.ActorID, string(e.Type), string(e.Params)) // string, not []byte: MySQL rejects binary into JSON
	return err
}

func (s *Store) ListChangeEvents(ctx context.Context, applicationID int64) ([]domain.StoredChangeEvent, error) {
	rows, err := s.q.QueryContext(ctx,
		"SELECT id, application_id, occurred_at, actor_user_id, event_type, params FROM change_events WHERE application_id = ? ORDER BY occurred_at DESC, id DESC",
		applicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.StoredChangeEvent{}
	for rows.Next() {
		var e domain.StoredChangeEvent
		var typ string
		if err := rows.Scan(&e.ID, &e.ApplicationID, &e.At, &e.ActorID, &typ, &e.Params); err != nil {
			return nil, err
		}
		e.Type = domain.EventType(typ)
		out = append(out, e)
	}
	return out, rows.Err()
}

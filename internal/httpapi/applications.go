package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/admax12959/X-Project/internal/domain"
)

// repoInputJSON is the repository binding as the client sends it.
type repoInputJSON struct {
	PlatformID string `json:"platformId"`
	Path       string `json:"path"`
}

func (r repoInputJSON) toDomain() domain.RepoInput {
	return domain.RepoInput{PlatformID: r.PlatformID, Path: r.Path}
}

// repoJSON is a verified repository binding.
type repoJSON struct {
	PlatformID string `json:"platformId"`
	Kind       string `json:"kind"`
	BaseURL    string `json:"baseUrl"`
	Path       string `json:"path"`
	ProjectID  string `json:"projectId"`
}

func toRepoJSON(b domain.RepoBinding) repoJSON {
	return repoJSON{
		PlatformID: strconv.FormatInt(b.Platform.ID, 10), Kind: string(b.Platform.Kind), BaseURL: b.Platform.BaseURL,
		Path: b.Path, ProjectID: b.ProjectID,
	}
}

type memberJSON struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
}

func toMembersJSON(us []domain.UserRef) []memberJSON {
	out := make([]memberJSON, 0, len(us))
	for _, u := range us {
		out = append(out, memberJSON{ID: strconv.FormatInt(int64(u.ID), 10), Username: u.Username, DisplayName: u.DisplayName, Status: string(u.Status)})
	}
	return out
}

type applicationJSON struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Repo        repoJSON     `json:"repo"`
	MainBranch  string       `json:"mainBranch"`
	Owners      []memberJSON `json:"owners"`
	PEs         []memberJSON `json:"pes"`
	DeployInfo  string       `json:"deployInfo"`
	CreatedAt   string       `json:"createdAt"`
}

func toApplicationJSON(a domain.Application) applicationJSON {
	return applicationJSON{
		Key: a.Key, Name: a.Name, Description: a.Description, Repo: toRepoJSON(a.Repo), MainBranch: a.MainBranch,
		Owners: toMembersJSON(a.Owners), PEs: toMembersJSON(a.PEs), DeployInfo: a.DeployInfo,
		CreatedAt: formatTime(a.CreatedAt),
	}
}

type applicationSummaryJSON struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Repo        repoJSON     `json:"repo"`
	Owners      []memberJSON `json:"owners"`
	LineCount   int          `json:"lineCount"`
	// LatestVersion is null for an application without versions.
	LatestVersion *homeVersionJSON `json:"latestVersion"`
}

// homeVersionJSON is the short version form of the home page: which version, and on which line.
type homeVersionJSON struct {
	ID         string   `json:"id"`
	Number     string   `json:"number"`
	ReleasedAt string   `json:"releasedAt"`
	Line       lineJSON `json:"line"`
}

// listApplications serves GET /api/applications?q=<search>&mine=<true|false>.
func (s *server) listApplications(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	apps, err := s.svc.ListApplications(r.Context(), actorOf(r), domain.ApplicationListQuery{Query: query.Get("q"), Mine: query.Get("mine")})
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := listJSON[applicationSummaryJSON]{Items: make([]applicationSummaryJSON, 0, len(apps))}
	for _, a := range apps {
		item := applicationSummaryJSON{
			Key: a.Key, Name: a.Name, Description: a.Description, Repo: toRepoJSON(a.Repo), Owners: toMembersJSON(a.Owners),
			LineCount: a.LineCount,
		}
		if v := a.LatestVersion; v != nil {
			item.LatestVersion = &homeVersionJSON{
				ID: strconv.FormatInt(v.ID, 10), Number: v.Number, ReleasedAt: formatDate(v.ReleasedAt),
				Line: shortLineJSON(v.Line),
			}
		}
		out.Items = append(out.Items, item)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) createApplication(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key         string        `json:"key"`
		Name        string        `json:"name"`
		Description string        `json:"description"`
		Repo        repoInputJSON `json:"repo"`
		MainBranch  string        `json:"mainBranch"`
		Owners      []string      `json:"owners"`
		PEs         []string      `json:"pes"`
	}
	if !decode(w, r, &body) {
		return
	}
	a, err := s.svc.CreateApplication(r.Context(), actorOf(r), domain.CreateApplicationInput{
		Key: body.Key, Name: body.Name, Description: body.Description, Repo: body.Repo.toDomain(),
		MainBranch: body.MainBranch, Owners: body.Owners, PEs: body.PEs,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toApplicationJSON(a))
}

func (s *server) getApplication(w http.ResponseWriter, r *http.Request) {
	a, err := s.svc.GetApplication(r.Context(), actorOf(r), r.PathValue("key"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toApplicationJSON(a))
}

// updateApplication serves PATCH /api/applications/{key}: basic info, Owners only.
// Unknown fields (key, repo...) are ignored: they have no edit path.
func (s *server) updateApplication(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		MainBranch  *string `json:"mainBranch"`
	}
	if !decode(w, r, &body) {
		return
	}
	a, err := s.svc.UpdateApplication(r.Context(), actorOf(r), r.PathValue("key"), domain.UpdateApplicationInput{
		Name: body.Name, Description: body.Description, MainBranch: body.MainBranch,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toApplicationJSON(a))
}

// listAssignableUsers serves GET /api/assignable-users: enabled users for the Owner/PE pickers.
func (s *server) listAssignableUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.ListAssignableUsers(r.Context(), actorOf(r), r.URL.Query().Get("departmentId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	type userOption struct {
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	}
	out := listJSON[userOption]{Items: make([]userOption, 0, len(users))}
	for _, u := range users {
		out.Items = append(out.Items, userOption{ID: strconv.FormatInt(int64(u.ID), 10), Username: u.Username, DisplayName: u.DisplayName})
	}
	writeJSON(w, http.StatusOK, out)
}

// deleteApplication serves DELETE /api/applications/{key} with {"confirmKey": "<key>"}.
// A missing body counts as an empty confirmation (the domain answers 403 / 404 / 422).
func (s *server) deleteApplication(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ConfirmKey string `json:"confirmKey"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeCode(w, http.StatusBadRequest, domain.CodeBadRequest)
		return
	}
	if err := s.svc.DeleteApplication(r.Context(), actorOf(r), r.PathValue("key"), body.ConfirmKey); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// updateMembers serves PUT /api/applications/{key}/members: administrators and Owners.
func (s *server) updateMembers(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Owners *[]string `json:"owners"`
		PEs    *[]string `json:"pes"`
	}
	if !decode(w, r, &body) {
		return
	}
	a, err := s.svc.UpdateMembers(r.Context(), actorOf(r), r.PathValue("key"), domain.UpdateMembersInput{Owners: body.Owners, PEs: body.PEs})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toApplicationJSON(a))
}

// updateDeployInfo serves PUT /api/applications/{key}/deploy-info: Owners and PEs.
func (s *server) updateDeployInfo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeployInfo *string `json:"deployInfo"`
	}
	if !decode(w, r, &body) {
		return
	}
	a, err := s.svc.UpdateDeployInfo(r.Context(), actorOf(r), r.PathValue("key"), body.DeployInfo)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toApplicationJSON(a))
}

// lineJSON: the mainline has no status (and no name of its own).
type lineJSON struct {
	ID     string `json:"id"`
	IsMain bool   `json:"isMain"`
	Name   string `json:"name"`
	Branch string `json:"branch"`
}

func (s *server) listLines(w http.ResponseWriter, r *http.Request) {
	lines, err := s.svc.ListLines(r.Context(), actorOf(r), r.PathValue("key"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := listJSON[lineDetailJSON]{Items: make([]lineDetailJSON, 0, len(lines))}
	for _, l := range lines {
		out.Items = append(out.Items, toLineDetailJSON(l))
	}
	writeJSON(w, http.StatusOK, out)
}

// changeEventJSON: type + structured params; the frontend renders the sentence
// in the UI language. Params may nest and contain enriched user references.
type changeEventJSON struct {
	ID     string         `json:"id"`
	At     string         `json:"at"`
	Actor  memberRefJSON  `json:"actor"`
	Type   string         `json:"type"`
	Params map[string]any `json:"params"`
}

func (s *server) listHistory(w http.ResponseWriter, r *http.Request) {
	events, err := s.svc.ListChangeEvents(r.Context(), actorOf(r), r.PathValue("key"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := listJSON[changeEventJSON]{Items: make([]changeEventJSON, 0, len(events))}
	for _, e := range events {
		out.Items = append(out.Items, changeEventJSON{
			ID: strconv.FormatInt(e.ID, 10), At: formatTime(e.At), Type: string(e.Type), Params: e.Params,
			Actor: toMemberRefJSON(e.Actor),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) verifyRepo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo repoInputJSON `json:"repo"`
	}
	if !decode(w, r, &body) {
		return
	}
	b, err := s.svc.VerifyRepo(r.Context(), actorOf(r), body.Repo.toDomain())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Repo repoJSON `json:"repo"`
	}{toRepoJSON(b)})
}

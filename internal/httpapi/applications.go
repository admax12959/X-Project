package httpapi

import (
	"net/http"
	"strconv"
	"time"

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
		CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

type applicationSummaryJSON struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Repo        repoJSON     `json:"repo"`
	Owners      []memberJSON `json:"owners"`
}

// listApplications serves GET /api/applications?q=<search>.
func (s *server) listApplications(w http.ResponseWriter, r *http.Request) {
	apps, err := s.svc.ListApplications(r.Context(), actorOf(r), r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := listJSON[applicationSummaryJSON]{Items: make([]applicationSummaryJSON, 0, len(apps))}
	for _, a := range apps {
		out.Items = append(out.Items, applicationSummaryJSON{
			Key: a.Key, Name: a.Name, Description: a.Description, Repo: toRepoJSON(a.Repo), Owners: toMembersJSON(a.Owners),
		})
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

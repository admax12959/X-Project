package httpapi

import (
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

package httpapi

import (
	"net/http"
	"strconv"

	"github.com/admax12959/X-Project/internal/domain"
)

// codePlatformJSON never carries the token; tokenTail is its last 4 characters ("" if too short).
type codePlatformJSON struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	BaseURL   string `json:"baseUrl"`
	TokenTail string `json:"tokenTail"`
	CreatedAt string `json:"createdAt"`
}

func toCodePlatformJSON(p domain.CodePlatform) codePlatformJSON {
	return codePlatformJSON{
		ID: strconv.FormatInt(p.ID, 10), Kind: string(p.Kind), BaseURL: p.BaseURL,
		TokenTail: p.TokenTail, CreatedAt: formatTime(p.CreatedAt),
	}
}

func (s *server) listCodePlatforms(w http.ResponseWriter, r *http.Request) {
	ps, err := s.svc.ListCodePlatforms(r.Context(), actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := listJSON[codePlatformJSON]{Items: make([]codePlatformJSON, 0, len(ps))}
	for _, p := range ps {
		out.Items = append(out.Items, toCodePlatformJSON(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) createCodePlatform(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind    string `json:"kind"`
		BaseURL string `json:"baseUrl"`
		Token   string `json:"token"`
	}
	if !decode(w, r, &body) {
		return
	}
	p, err := s.svc.CreateCodePlatform(r.Context(), actorOf(r), domain.CodePlatformInput{Kind: body.Kind, BaseURL: body.BaseURL, Token: body.Token})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCodePlatformJSON(p))
}

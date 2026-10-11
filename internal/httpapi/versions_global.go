package httpapi

import "net/http"

// globalVersionJSON is a version plus the application it belongs to.
type globalVersionJSON struct {
	versionJSON
	Application struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"application"`
}

// listAllVersions serves GET /api/versions: every application's versions, latest
// release date first.
func (s *server) listAllVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := s.svc.ListAllVersions(r.Context(), actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := struct {
		Items []globalVersionJSON `json:"items"`
		Total int                 `json:"total"`
	}{Items: make([]globalVersionJSON, 0, len(versions)), Total: len(versions)}
	for _, v := range versions {
		g := globalVersionJSON{versionJSON: toVersionJSON(v.Version)}
		g.Application.Key, g.Application.Name = v.AppKey, v.AppName
		out.Items = append(out.Items, g)
	}
	writeJSON(w, http.StatusOK, out)
}

package httpapi

import (
	"net/http"

	"github.com/admax12959/X-Project/internal/domain"
)

// globalVersionJSON is a version plus the application it belongs to.
type globalVersionJSON struct {
	versionJSON
	Application struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"application"`
}

// listAllVersions serves GET /api/versions?app=&q=&page=&pageSize=: one page of
// every application's versions, latest release date first. `total` counts all
// matches, not just this page.
func (s *server) listAllVersions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, err := s.svc.ListAllVersions(r.Context(), actorOf(r), domain.GlobalVersionQuery{
		GlobalVersionFilter: domain.GlobalVersionFilter{AppKey: query.Get("app"), Search: query.Get("q")},
		Page:                query.Get("page"), PageSize: query.Get("pageSize"),
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := struct {
		Items    []globalVersionJSON `json:"items"`
		Total    int                 `json:"total"`
		Page     int                 `json:"page"`
		PageSize int                 `json:"pageSize"`
	}{Items: make([]globalVersionJSON, 0, len(page.Items)), Total: page.Total, Page: page.Page, PageSize: page.PageSize}
	for _, v := range page.Items {
		g := globalVersionJSON{versionJSON: toVersionJSON(v.Version)}
		g.Application.Key, g.Application.Name = v.AppKey, v.AppName
		out.Items = append(out.Items, g)
	}
	writeJSON(w, http.StatusOK, out)
}

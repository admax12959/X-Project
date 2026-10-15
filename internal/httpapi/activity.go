package httpapi

import (
	"net/http"
	"strconv"
)

// activityJSON is a change event (see changeEventJSON) plus the application it belongs to.
type activityJSON struct {
	changeEventJSON
	Application struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"application"`
}

// listRecentActivity serves GET /api/recent-activity?limit=<1-50, default 8>.
func (s *server) listRecentActivity(w http.ResponseWriter, r *http.Request) {
	events, err := s.svc.ListRecentActivity(r.Context(), actorOf(r), r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := listJSON[activityJSON]{Items: make([]activityJSON, 0, len(events))}
	for _, e := range events {
		item := activityJSON{changeEventJSON: changeEventJSON{
			ID: strconv.FormatInt(e.ID, 10), At: formatTime(e.At), Type: string(e.Type), Params: e.Params,
			Actor: toMemberRefJSON(e.Actor),
		}}
		item.Application.Key, item.Application.Name = e.AppKey, e.AppName
		out.Items = append(out.Items, item)
	}
	writeJSON(w, http.StatusOK, out)
}

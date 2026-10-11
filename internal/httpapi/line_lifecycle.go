package httpapi

import "net/http"

// setLineStatus serves PUT /api/applications/{key}/lines/{lineId}/status.
func (s *server) setLineStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &body) {
		return
	}
	l, err := s.svc.SetLineStatus(r.Context(), actorOf(r), r.PathValue("key"), r.PathValue("lineId"), body.Status)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLineDetailJSON(l))
}

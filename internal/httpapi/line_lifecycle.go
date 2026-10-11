package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// statusChangeJSON is one entry of a line's status log: when, who, from and to.
type statusChangeJSON struct {
	From  string        `json:"from"`
	To    string        `json:"to"`
	At    string        `json:"at"`
	Actor memberRefJSON `json:"actor"`
}

// lineWithLogJSON is a line plus its status log, oldest change first (never null; empty
// for the mainline and for a line that never changed). Returned by GET /lines/{id},
// PUT /lines/{id}/status and PATCH /lines/{id}.
type lineWithLogJSON struct {
	lineDetailJSON
	StatusChanges []statusChangeJSON `json:"statusChanges"`
}

func toLineWithLogJSON(d domain.LineDetail) lineWithLogJSON {
	out := lineWithLogJSON{lineDetailJSON: toLineDetailJSON(d.Line), StatusChanges: make([]statusChangeJSON, 0, len(d.StatusChanges))}
	for _, c := range d.StatusChanges {
		out.StatusChanges = append(out.StatusChanges, statusChangeJSON{
			From: string(c.From), To: string(c.To), At: c.At.UTC().Format(time.RFC3339Nano), Actor: memberRefJSON{ID: strconv.FormatInt(int64(c.Actor.ID), 10), Username: c.Actor.Username, DisplayName: c.Actor.DisplayName},
		})
	}
	return out
}

// getLine serves GET /api/applications/{key}/lines/{lineId}.
func (s *server) getLine(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.GetLine(r.Context(), actorOf(r), r.PathValue("key"), r.PathValue("lineId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLineWithLogJSON(d))
}

// updateLine serves PATCH /api/applications/{key}/lines/{lineId}. Only name, note and
// plannedEnd exist in the body type; any other field is ignored. An empty note or
// plannedEnd clears it, an absent one leaves it.
func (s *server) updateLine(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       *string `json:"name"`
		Note       *string `json:"note"`
		PlannedEnd *string `json:"plannedEnd"`
	}
	if !decode(w, r, &body) {
		return
	}
	d, err := s.svc.UpdateLine(r.Context(), actorOf(r), r.PathValue("key"), r.PathValue("lineId"), domain.UpdateLineInput{
		Name: body.Name, Note: body.Note, PlannedEnd: body.PlannedEnd,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLineWithLogJSON(d))
}

// setLineStatus serves PUT /api/applications/{key}/lines/{lineId}/status.
func (s *server) setLineStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &body) {
		return
	}
	d, err := s.svc.SetLineStatus(r.Context(), actorOf(r), r.PathValue("key"), r.PathValue("lineId"), body.Status)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLineWithLogJSON(d))
}

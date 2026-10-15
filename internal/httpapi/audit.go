package httpapi

import (
	"net/http"
	"strconv"

	"github.com/admax12959/X-Project/internal/domain"
)

// auditActorJSON is the operator of an audit entry: kind "user" with the user's
// reference, or kind "cli" (the command line in the container) without one.
type auditActorJSON struct {
	Kind        string `json:"kind"`
	ID          string `json:"id,omitempty"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

// auditTargetJSON is the object of an audit entry: a user (id = user ID, with the user's
// current name) or an application (id = application key).
type auditTargetJSON struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

type auditEntryJSON struct {
	ID     string          `json:"id"`
	At     string          `json:"at"`
	Action string          `json:"action"`
	Actor  auditActorJSON  `json:"actor"`
	Target auditTargetJSON `json:"target"`
	Params map[string]any  `json:"params"`
}

// listAudit serves GET /api/audit-log?actor=<user id|cli>&targetType=<user|application>&targetId=&action=&from=&to=&page=&pageSize=
// (from / to: RFC 3339, inclusive). Newest first; `total` counts all matches.
func (s *server) listAudit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, err := s.svc.ListAudit(r.Context(), actorOf(r), domain.AuditQuery{
		Actor: q.Get("actor"), TargetType: q.Get("targetType"), TargetID: q.Get("targetId"), Action: q.Get("action"),
		From: q.Get("from"), To: q.Get("to"), Page: q.Get("page"), PageSize: q.Get("pageSize"),
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := struct {
		Items    []auditEntryJSON `json:"items"`
		Total    int              `json:"total"`
		Page     int              `json:"page"`
		PageSize int              `json:"pageSize"`
	}{Items: make([]auditEntryJSON, 0, len(page.Items)), Total: page.Total, Page: page.Page, PageSize: page.PageSize}
	for _, e := range page.Items {
		item := auditEntryJSON{
			ID: strconv.FormatInt(e.ID, 10), At: formatTime(e.At), Action: string(e.Action), Params: e.Params,
			Actor:  auditActorJSON{Kind: "cli"},
			Target: auditTargetJSON{Type: string(e.Target.Type), ID: e.Target.ID},
		}
		if !e.Actor.CLI {
			ref := toMemberRefJSON(e.Actor.User)
			item.Actor = auditActorJSON{Kind: "user", ID: ref.ID, Username: ref.Username, DisplayName: ref.DisplayName}
		}
		if u := e.Target.User; u != nil {
			item.Target.Username, item.Target.DisplayName = u.Username, u.DisplayName
		}
		out.Items = append(out.Items, item)
	}
	writeJSON(w, http.StatusOK, out)
}

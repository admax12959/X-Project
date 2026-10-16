package httpapi

import "net/http"

// roleJSON is a built-in role with the permission points it holds. Descriptions are
// rendered by the frontend from the permission key. Roles are read-only: there is no
// write route.
type roleJSON struct {
	ID          string   `json:"id"`
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

func (s *server) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := s.svc.BuiltInRoles(actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := listJSON[roleJSON]{Items: make([]roleJSON, 0, len(roles))}
	for _, role := range roles {
		item := roleJSON{ID: string(role.ID), Scope: string(role.Scope), Permissions: make([]string, 0, len(role.Permissions))}
		for _, p := range role.Permissions {
			item.Permissions = append(item.Permissions, string(p))
		}
		out.Items = append(out.Items, item)
	}
	writeJSON(w, http.StatusOK, out)
}

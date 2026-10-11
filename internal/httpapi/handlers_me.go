package httpapi

import (
	"net/http"

	"github.com/admax12959/X-Project/internal/domain"
)

// meRoutes are the routes of the logged-in user's own settings (personal settings page).
func (s *server) meRoutes() {
	s.route("PATCH /api/me", s.updateMe)
}

func (s *server) updateMe(w http.ResponseWriter, r *http.Request) {
	// Pointers distinguish "not sent" (unchanged) from "sent empty".
	var body struct {
		Language *string `json:"language"`
	}
	if !decode(w, r, &body) {
		return
	}
	u, err := s.svc.UpdateMe(r.Context(), actorOf(r), domain.UpdateMeInput{Language: body.Language})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserJSON(u))
}

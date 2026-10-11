package httpapi

import (
	"net/http"

	"github.com/admax12959/X-Project/internal/domain"
)

// meRoutes are the routes of the logged-in user's own settings (personal settings page).
func (s *server) meRoutes() {
	s.route("GET /api/me", s.getMe)
	s.route("PATCH /api/me", s.updateMe)
	s.route("POST /api/me/identities", s.bindOwnIdentity)
}

func (s *server) getMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.svc.GetMe(r.Context(), actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserJSON(u))
}

func (s *server) updateMe(w http.ResponseWriter, r *http.Request) {
	// Pointers distinguish "not sent" (unchanged) from "sent empty".
	var body struct {
		DisplayName *string `json:"displayName"`
		Email       *string `json:"email"`
		Phone       *string `json:"phone"`
		Department  *string `json:"department"`
		Language    *string `json:"language"`
	}
	if !decode(w, r, &body) {
		return
	}
	u, err := s.svc.UpdateMe(r.Context(), actorOf(r), domain.UpdateMeInput{
		DisplayName: body.DisplayName, Email: body.Email, Phone: body.Phone, Department: body.Department, Language: body.Language,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserJSON(u))
}

// bindIdentityBody is the request body of both "bind an identity" endpoints.
type bindIdentityBody struct {
	Kind      string `json:"kind"`
	LoginName string `json:"loginName"`
	Password  string `json:"password"`
}

func (b bindIdentityBody) input() domain.BindIdentityInput {
	return domain.BindIdentityInput{Kind: b.Kind, LoginName: b.LoginName, Password: b.Password}
}

func (s *server) bindOwnIdentity(w http.ResponseWriter, r *http.Request) {
	var body bindIdentityBody
	if !decode(w, r, &body) {
		return
	}
	u, err := s.svc.BindOwnIdentity(r.Context(), actorOf(r), body.input())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAdminUserJSON(u))
}

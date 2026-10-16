package httpapi

import (
	"net/http"

	"github.com/admax12959/X-Project/internal/domain"
)

// meRoutes are the routes of the logged-in user's own settings (personal settings page).
func (s *server) meRoutes() {
	s.route("GET /api/me", s.getMe)
	s.route("PATCH /api/me", s.updateMe)
	s.route("PUT /api/me/password", s.changePassword)
	s.route("POST /api/me/identities", s.bindOwnIdentity)
	s.route("DELETE /api/me/identities/{kind}", s.unbindOwnIdentity)
	s.route("POST /api/me/email-identity/code", s.requestBindEmailCode)
	s.route("POST /api/me/email-identity", s.bindOwnEmailIdentity)
	s.route("POST /api/me/login-email/code", s.requestChangeLoginEmailCode)
	s.route("PUT /api/me/login-email", s.changeOwnLoginEmail)
	s.route("POST /api/users/{id}/identities", s.adminBindIdentity)
}

func (s *server) unbindOwnIdentity(w http.ResponseWriter, r *http.Request) {
	u, err := s.svc.UnbindOwnIdentity(r.Context(), actorOf(r), r.PathValue("kind"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserJSON(u))
}

func (s *server) changePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decode(w, r, &body) {
		return
	}
	if err := s.svc.ChangePassword(r.Context(), actorOf(r), sessionToken(r), body.CurrentPassword, body.NewPassword); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
		Language    *string `json:"language"`
	}
	if !decode(w, r, &body) {
		return
	}
	u, err := s.svc.UpdateMe(r.Context(), actorOf(r), domain.UpdateMeInput{
		DisplayName: body.DisplayName, Email: body.Email, Phone: body.Phone, Language: body.Language,
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

// adminBindIdentity: an administrator binds an identity to any user (user management).
func (s *server) adminBindIdentity(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDParam(w, r)
	if !ok {
		return
	}
	var body bindIdentityBody
	if !decode(w, r, &body) {
		return
	}
	u, err := s.svc.BindIdentityForUser(r.Context(), actorOf(r), id, body.input())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAdminUserJSON(u))
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

// Self-service email identity (#30). The code is mailed to the new address; confirming it
// needs the code and, for a new identity, the password to log in with.

func (s *server) requestBindEmailCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &body) {
		return
	}
	if err := s.svc.RequestBindEmailCode(r.Context(), actorOf(r), body.Email, domain.ClientIP(r.RemoteAddr)); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *server) bindOwnEmailIdentity(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	u, err := s.svc.BindOwnEmailIdentity(r.Context(), actorOf(r), domain.BindEmailInput{Email: body.Email, Code: body.Code, Password: body.Password})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAdminUserJSON(u))
}

func (s *server) requestChangeLoginEmailCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &body) {
		return
	}
	if err := s.svc.RequestChangeLoginEmailCode(r.Context(), actorOf(r), body.Email, domain.ClientIP(r.RemoteAddr)); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *server) changeOwnLoginEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !decode(w, r, &body) {
		return
	}
	u, err := s.svc.ChangeOwnLoginEmail(r.Context(), actorOf(r), body.Email, body.Code)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserJSON(u))
}

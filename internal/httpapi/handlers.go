package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// ---------------------------------------------------------------- JSON shapes
// IDs are serialized as strings so JavaScript never loses precision.

type userJSON struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	// DirectoryDepartment is the department text synced from LDAP (read-only).
	DirectoryDepartment string `json:"directoryDepartment"`
	Language            string `json:"language"` // "zh", "en", or "" when never chosen
	IsAdmin             bool   `json:"isAdmin"`
}

func toUserJSON(u domain.User) userJSON {
	return userJSON{ID: strconv.FormatInt(int64(u.ID), 10), Username: u.Username, DisplayName: u.DisplayName, Email: u.Email, DirectoryDepartment: u.DirectoryDepartment, Language: u.Language, IsAdmin: u.IsAdmin}
}

type sessionJSON struct {
	User userJSON `json:"user"`
	// MustChangePassword: the password was chosen by an administrator. Until the user
	// changes it (PUT /api/me/password) every other endpoint answers 403 passwordChangeRequired.
	MustChangePassword bool `json:"mustChangePassword"`
}

// memberRefJSON is a user inside another record (Owner/PE lists, event and status-change actors).
type memberRefJSON struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

func toMemberRefJSON(u domain.UserRef) memberRefJSON {
	return memberRefJSON{ID: strconv.FormatInt(int64(u.ID), 10), Username: u.Username, DisplayName: u.DisplayName}
}

// formatDate is a calendar date on the wire (YYYY-MM-DD); formatTime is an instant (RFC 3339, UTC).
func formatDate(t time.Time) string { return t.Format(domain.DateLayout) }
func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// listJSON is the envelope for every collection response.
type listJSON[T any] struct {
	Items []T `json:"items"`
}

// ---------------------------------------------------------------- sessions

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Method   string `json:"method"`
		Username string `json:"username"` // method "ldap"
		Email    string `json:"email"`    // method "email"
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	res, err := s.svc.Login(r.Context(), domain.LoginInput{Method: body.Method, Username: body.Username, Email: body.Email, Password: body.Password})
	if err != nil {
		writeError(w, r, err)
		return
	}
	s.setSessionCookie(w, res.Token, res.ExpiresAt)
	writeJSON(w, http.StatusOK, sessionJSON{User: toUserJSON(res.User), MustChangePassword: res.MustChangePassword})
}

func (s *server) loginMethods(w http.ResponseWriter, r *http.Request) {
	methods, err := s.svc.LoginMethods(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listJSON[string]{Items: methods})
}

func (s *server) currentSession(w http.ResponseWriter, r *http.Request) {
	a := actorOf(r)
	writeJSON(w, http.StatusOK, sessionJSON{User: toUserJSON(a.User), MustChangePassword: a.MustChangePassword})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.Logout(r.Context(), sessionToken(r)); err != nil {
		writeError(w, r, err)
		return
	}
	s.setSessionCookie(w, "", time.Time{})
	w.WriteHeader(http.StatusNoContent)
}

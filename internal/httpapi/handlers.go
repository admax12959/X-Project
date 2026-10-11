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
	Department  string `json:"department"`
	Language    string `json:"language"` // "zh", "en", or "" when never chosen
	IsAdmin     bool   `json:"isAdmin"`
}

func toUserJSON(u domain.User) userJSON {
	return userJSON{ID: strconv.FormatInt(int64(u.ID), 10), Username: u.Username, DisplayName: u.DisplayName, Email: u.Email, Department: u.Department, Language: u.Language, IsAdmin: u.IsAdmin}
}

type sessionJSON struct {
	User userJSON `json:"user"`
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
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	res, err := s.svc.Login(r.Context(), domain.LoginInput{Method: body.Method, Username: body.Username, Password: body.Password})
	if err != nil {
		writeError(w, r, err)
		return
	}
	s.setSessionCookie(w, res.Token, res.ExpiresAt)
	writeJSON(w, http.StatusOK, sessionJSON{User: toUserJSON(res.User)})
}

func (s *server) loginMethods(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, listJSON[string]{Items: s.svc.LoginMethods()})
}

func (s *server) currentSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sessionJSON{User: toUserJSON(actorOf(r).User)})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.Logout(r.Context(), sessionToken(r)); err != nil {
		writeError(w, r, err)
		return
	}
	s.setSessionCookie(w, "", time.Time{})
	w.WriteHeader(http.StatusNoContent)
}

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
	IsAdmin     bool   `json:"isAdmin"`
}

func toUserJSON(u domain.User) userJSON {
	return userJSON{ID: strconv.FormatInt(int64(u.ID), 10), Username: u.Username, DisplayName: u.DisplayName, Email: u.Email, Department: u.Department, IsAdmin: u.IsAdmin}
}

type sessionJSON struct {
	User userJSON `json:"user"`
}

type applicationJSON struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

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

// ---------------------------------------------------------------- applications

func (s *server) listApplications(w http.ResponseWriter, r *http.Request) {
	apps, err := s.svc.ListApplications(r.Context(), actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := listJSON[applicationJSON]{Items: make([]applicationJSON, 0, len(apps))}
	for _, a := range apps {
		out.Items = append(out.Items, applicationJSON{Key: a.Key, Name: a.Name, Description: a.Description})
	}
	writeJSON(w, http.StatusOK, out)
}

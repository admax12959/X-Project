package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/admax12959/X-Project/internal/domain"
)

// adminUserJSON is the administrator view of a user.
type adminUserJSON struct {
	ID                  string `json:"id"`
	Username            string `json:"username"`
	DisplayName         string `json:"displayName"`
	Email               string `json:"email"`
	Phone               string `json:"phone"`
	DirectoryDepartment string `json:"directoryDepartment"`
	// DepartmentID is the department in the organization (null = none), not the directory department.
	DepartmentID *string        `json:"departmentId"`
	Language     string         `json:"language"`
	Status       string         `json:"status"`
	IsAdmin      bool           `json:"isAdmin"`
	Identities   []identityJSON `json:"identities"`
	AvatarURL    *string        `json:"avatarUrl"`
}

type identityJSON struct {
	Kind      string `json:"kind"`
	LoginName string `json:"loginName"`
	BoundAt   string `json:"boundAt"`
	BoundVia  string `json:"boundVia"`
}

func toAdminUserJSON(v domain.UserWithIdentities) adminUserJSON {
	u := v.User
	out := adminUserJSON{
		ID: strconv.FormatInt(int64(u.ID), 10), Username: u.Username, DisplayName: u.DisplayName,
		Email: u.Email, Phone: u.Phone, DirectoryDepartment: u.DirectoryDepartment, Language: u.Language, Status: string(u.Status), IsAdmin: u.IsAdmin, AvatarURL: avatarURL(u.ID, u.AvatarHash),
		Identities: make([]identityJSON, 0, len(v.Identities)),
	}
	if u.DepartmentID != 0 {
		d := formatID(int64(u.DepartmentID))
		out.DepartmentID = &d
	}
	for _, i := range v.Identities {
		out.Identities = append(out.Identities, identityJSON{
			Kind: string(i.Kind), LoginName: i.LoginName, BoundAt: formatTime(i.BoundAt), BoundVia: string(i.BoundVia),
		})
	}
	return out
}

func (s *server) userRoutes() {
	s.route("GET /api/users", s.listUsers)
	s.route("POST /api/users", s.createUser)
	s.route("PATCH /api/users/{id}", s.updateUser)
	s.route("POST /api/users/{id}/disable", s.statusHandler(s.svc.DisableUser))
	s.route("POST /api/users/{id}/enable", s.statusHandler(s.svc.EnableUser))
	// Deliberately no DELETE: users can only be disabled.
}

func (s *server) statusHandler(change func(context.Context, domain.Actor, domain.UserID) (domain.UserWithIdentities, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := userIDParam(w, r)
		if !ok {
			return
		}
		u, err := change(r.Context(), actorOf(r), id)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toAdminUserJSON(u))
	}
}

// userIDParam reads {id}; a malformed id is a user that does not exist.
func userIDParam(w http.ResponseWriter, r *http.Request) (domain.UserID, bool) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || n <= 0 {
		writeCode(w, http.StatusNotFound, domain.CodeNotFound)
		return 0, false
	}
	return domain.UserID(n), true
}

func (s *server) updateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDParam(w, r)
	if !ok {
		return
	}
	// Pointers distinguish "not sent" (unchanged) from "sent empty" (cleared).
	var body struct {
		Username     *string         `json:"username"`
		DisplayName  *string         `json:"displayName"`
		Email        *string         `json:"email"`
		LoginEmail   *string         `json:"loginEmail"`
		Phone        *string         `json:"phone"`
		IsAdmin      *bool           `json:"isAdmin"`
		DepartmentID json.RawMessage `json:"departmentId"` // absent = unchanged, null = none
	}
	if !decode(w, r, &body) {
		return
	}
	dept, ok := refString(w, body.DepartmentID)
	if !ok {
		return
	}
	u, err := s.svc.UpdateUser(r.Context(), actorOf(r), id, domain.UpdateUserInput{
		Username: body.Username, DisplayName: body.DisplayName, Email: body.Email, LoginEmail: body.LoginEmail,
		Phone: body.Phone, Department: dept, IsAdmin: body.IsAdmin,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserJSON(u))
}

func (s *server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.ListUsers(r.Context(), actorOf(r), r.URL.Query().Get("departmentId"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := listJSON[adminUserJSON]{Items: make([]adminUserJSON, 0, len(users))}
	for _, u := range users {
		out.Items = append(out.Items, toAdminUserJSON(u))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username     string `json:"username"`
		DisplayName  string `json:"displayName"`
		LoginEmail   string `json:"loginEmail"`
		Phone        string `json:"phone"`
		DepartmentID string `json:"departmentId"`
		Password     string `json:"password"` // temporary: the user must change it at the first login
	}
	if !decode(w, r, &body) {
		return
	}
	u, err := s.svc.CreateEmailUser(r.Context(), actorOf(r), domain.CreateEmailUserInput{
		Username: body.Username, DisplayName: body.DisplayName, LoginEmail: body.LoginEmail,
		Phone: body.Phone, DepartmentID: body.DepartmentID, Password: body.Password,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAdminUserJSON(u))
}

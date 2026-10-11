package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// adminUserJSON is the administrator view of a user.
type adminUserJSON struct {
	ID          string         `json:"id"`
	Username    string         `json:"username"`
	DisplayName string         `json:"displayName"`
	Email       string         `json:"email"`
	Phone       string         `json:"phone"`
	Department  string         `json:"department"`
	Status      string         `json:"status"`
	IsAdmin     bool           `json:"isAdmin"`
	Identities  []identityJSON `json:"identities"`
}

type identityJSON struct {
	Kind      string    `json:"kind"`
	LoginName string    `json:"loginName"`
	BoundAt   time.Time `json:"boundAt"`
	BoundVia  string    `json:"boundVia"`
}

func toAdminUserJSON(v domain.UserWithIdentities) adminUserJSON {
	u := v.User
	out := adminUserJSON{
		ID: strconv.FormatInt(int64(u.ID), 10), Username: u.Username, DisplayName: u.DisplayName,
		Email: u.Email, Phone: u.Phone, Department: u.Department, Status: string(u.Status), IsAdmin: u.IsAdmin,
		Identities: make([]identityJSON, 0, len(v.Identities)),
	}
	for _, i := range v.Identities {
		out.Identities = append(out.Identities, identityJSON{
			Kind: string(i.Kind), LoginName: i.LoginName, BoundAt: i.BoundAt, BoundVia: string(i.BoundVia),
		})
	}
	return out
}

func (s *server) userRoutes() {
	s.route("GET /api/users", s.listUsers)
	s.route("POST /api/users", s.createUser)
}

func (s *server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.ListUsers(r.Context(), actorOf(r))
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
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		Department  string `json:"department"`
		Password    string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	u, err := s.svc.CreateLocalUser(r.Context(), actorOf(r), domain.CreateLocalUserInput{
		Username: body.Username, DisplayName: body.DisplayName, Email: body.Email,
		Phone: body.Phone, Department: body.Department, Password: body.Password,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAdminUserJSON(u))
}

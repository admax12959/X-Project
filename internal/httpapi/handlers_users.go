package httpapi

import (
	"net/http"
	"strconv"

	"github.com/admax12959/X-Project/internal/domain"
)

// adminUserJSON is the administrator view of a user.
type adminUserJSON struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Department  string `json:"department"`
	Status      string `json:"status"`
	IsAdmin     bool   `json:"isAdmin"`
}

func toAdminUserJSON(u domain.User) adminUserJSON {
	return adminUserJSON{
		ID: strconv.FormatInt(int64(u.ID), 10), Username: u.Username, DisplayName: u.DisplayName,
		Email: u.Email, Phone: u.Phone, Department: u.Department, Status: string(u.Status), IsAdmin: u.IsAdmin,
	}
}

func (s *server) userRoutes() {
	s.route("POST /api/users", s.createUser)
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

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/admax12959/X-Project/internal/domain"
)

// departmentJSON is a node of the department tree, sent as a flat list: parentId is null for
// a top-level department. memberCount / childCount are direct members and direct child
// departments.
type departmentJSON struct {
	ID          string  `json:"id"`
	ParentID    *string `json:"parentId"`
	Name        string  `json:"name"`
	Source      string  `json:"source"`
	MemberCount int     `json:"memberCount"`
	ChildCount  int     `json:"childCount"`
}

func formatID(id int64) string { return strconv.FormatInt(id, 10) }

func toDepartmentJSON(d domain.Department) departmentJSON {
	out := departmentJSON{
		ID: formatID(int64(d.ID)), Name: d.Name, Source: d.Source, MemberCount: d.MemberCount, ChildCount: d.ChildCount,
	}
	if d.ParentID != 0 {
		p := formatID(int64(d.ParentID))
		out.ParentID = &p
	}
	return out
}

func (s *server) departmentRoutes() {
	s.route("GET /api/departments", s.listDepartments)
	s.route("POST /api/departments", s.createDepartment)
	s.route("PATCH /api/departments/{id}", s.updateDepartment)
	s.route("DELETE /api/departments/{id}", s.deleteDepartment)
}

func (s *server) listDepartments(w http.ResponseWriter, r *http.Request) {
	items, err := s.svc.ListDepartments(r.Context(), actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := listJSON[departmentJSON]{Items: make([]departmentJSON, 0, len(items))}
	for _, d := range items {
		out.Items = append(out.Items, toDepartmentJSON(d))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) createDepartment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string          `json:"name"`
		ParentID json.RawMessage `json:"parentId"` // absent, null or an ID: all mean what they say
	}
	if !decode(w, r, &body) {
		return
	}
	parent, ok := refString(w, body.ParentID)
	if !ok {
		return
	}
	d, err := s.svc.CreateDepartment(r.Context(), actorOf(r), domain.DepartmentInput{Name: body.Name, ParentID: parent.ID})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toDepartmentJSON(d))
}

func (s *server) updateDepartment(w http.ResponseWriter, r *http.Request) {
	id, ok := departmentIDParam(w, r)
	if !ok {
		return
	}
	var body struct {
		Name     *string         `json:"name"`
		ParentID json.RawMessage `json:"parentId"` // absent = unchanged, null = top level
	}
	if !decode(w, r, &body) {
		return
	}
	parent, ok := refString(w, body.ParentID)
	if !ok {
		return
	}
	d, err := s.svc.UpdateDepartment(r.Context(), actorOf(r), id, domain.UpdateDepartmentInput{Name: body.Name, Parent: parent})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toDepartmentJSON(d))
}

func (s *server) deleteDepartment(w http.ResponseWriter, r *http.Request) {
	id, ok := departmentIDParam(w, r)
	if !ok {
		return
	}
	if err := s.svc.DeleteDepartment(r.Context(), actorOf(r), id); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// departmentIDParam reads {id}; a malformed id is a department that does not exist.
func departmentIDParam(w http.ResponseWriter, r *http.Request) (domain.DepartmentID, bool) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || n <= 0 {
		writeCode(w, http.StatusNotFound, domain.CodeNotFound)
		return 0, false
	}
	return domain.DepartmentID(n), true
}

// refString reads an optional reference from a body field kept as raw JSON: absent is not
// set, null is set to "" (none), a string is that ID. Any other JSON type is a bad request.
func refString(w http.ResponseWriter, raw json.RawMessage) (domain.OptionalRef, bool) {
	if len(raw) == 0 {
		return domain.OptionalRef{}, true
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return domain.OptionalRef{Set: true}, true
	}
	var id string
	if err := json.Unmarshal(raw, &id); err != nil {
		writeCode(w, http.StatusBadRequest, domain.CodeBadRequest)
		return domain.OptionalRef{}, false
	}
	return domain.OptionalRef{Set: true, ID: id}, true
}

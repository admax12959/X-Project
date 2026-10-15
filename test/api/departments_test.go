package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// deptBody mirrors an item of GET /api/departments.
type deptBody struct {
	ID          string  `json:"id"`
	ParentID    *string `json:"parentId"`
	Name        string  `json:"name"`
	Source      string  `json:"source"`
	MemberCount int     `json:"memberCount"`
}

func listDepartments(t *testing.T, c *apitest.Client) []deptBody {
	t.Helper()
	var l struct {
		Items []deptBody `json:"items"`
	}
	c.Get("/api/departments").ExpectStatus(http.StatusOK).JSON(&l)
	return l.Items
}

func findDept(t *testing.T, items []deptBody, id string) deptBody {
	t.Helper()
	for _, d := range items {
		if d.ID == id {
			return d
		}
	}
	t.Fatalf("department %s not in %+v", id, items)
	return deptBody{}
}

// createDept creates a department; parent "" = a top-level one.
func createDept(t *testing.T, c *apitest.Client, name, parent string) deptBody {
	t.Helper()
	body := map[string]any{"name": name}
	if parent != "" {
		body["parentId"] = parent
	}
	var d deptBody
	c.Post("/api/departments", body).ExpectStatus(http.StatusCreated).JSON(&d)
	return d
}

func patchDept(c *apitest.Client, id string, fields map[string]any) *apitest.Response {
	return c.Do(http.MethodPatch, "/api/departments/"+id, fields)
}

func deleteDept(c *apitest.Client, id string) *apitest.Response {
	return c.Do(http.MethodDelete, "/api/departments/"+id, nil)
}

func TestAdministratorBuildsADepartmentTreeEveryoneCanRead(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	if got := listDepartments(t, admin); len(got) != 0 {
		t.Fatalf("a new system has departments: %+v", got)
	}
	tech := createDept(t, admin, "  Technology ", "")
	payments := createDept(t, admin, "Payments", tech.ID)
	createDept(t, admin, "Risk", "")
	if tech.Name != "Technology" || tech.ParentID != nil || tech.Source != "local" || tech.MemberCount != 0 {
		t.Fatalf("tech = %+v", tech)
	}
	if payments.ParentID == nil || *payments.ParentID != tech.ID {
		t.Fatalf("payments = %+v, want parent %s", payments, tech.ID)
	}

	// The tree is readable by every logged-in user (Owners filter the picker by it), not by strangers.
	bob := plainUserClient(t, env, "bob")
	items := listDepartments(t, bob)
	if len(items) != 3 {
		t.Fatalf("bob sees %+v", items)
	}
	if got := findDept(t, items, payments.ID); got.Name != "Payments" || *got.ParentID != tech.ID {
		t.Fatalf("payments as bob sees it = %+v", got)
	}
	env.Client().Get("/api/departments").ExpectStatus(http.StatusUnauthorized)
}

func TestOnlyAdministratorsMaintainDepartments(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	tech := createDept(t, admin, "Technology", "")
	bob := plainUserClient(t, env, "bob")

	bob.Post("/api/departments", map[string]any{"name": "Mine"}).ExpectStatus(http.StatusForbidden)
	patchDept(bob, tech.ID, map[string]any{"name": "Mine"}).ExpectStatus(http.StatusForbidden)
	deleteDept(bob, tech.ID).ExpectStatus(http.StatusForbidden)
	env.Client().Post("/api/departments", map[string]any{"name": "Mine"}).ExpectStatus(http.StatusUnauthorized)
	if got := listDepartments(t, admin); len(got) != 1 || got[0].Name != "Technology" {
		t.Fatalf("departments after refused writes = %+v", got)
	}
}

func TestDepartmentNameAndParentAreValidated(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	tech := createDept(t, admin, "Technology", "")

	admin.Post("/api/departments", map[string]any{"name": "  "}).ExpectError(http.StatusUnprocessableEntity, "name", "required")
	e := admin.Post("/api/departments", map[string]any{"name": strings.Repeat("x", 201)}).
		ExpectError(http.StatusUnprocessableEntity, "name", "tooLong")
	if e.Params["max"] != float64(200) {
		t.Fatalf("params = %v", e.Params)
	}
	admin.Post("/api/departments", map[string]any{"name": "A", "parentId": "9999"}).ExpectError(http.StatusUnprocessableEntity, "parentId", "invalid")
	admin.Post("/api/departments", map[string]any{"name": "A", "parentId": "abc"}).ExpectError(http.StatusUnprocessableEntity, "parentId", "invalid")

	patchDept(admin, tech.ID, map[string]any{"name": ""}).ExpectError(http.StatusUnprocessableEntity, "name", "required")
	patchDept(admin, tech.ID, map[string]any{"parentId": "9999"}).ExpectError(http.StatusUnprocessableEntity, "parentId", "invalid")
	patchDept(admin, "9999", map[string]any{"name": "X"}).ExpectStatus(http.StatusNotFound)
	patchDept(admin, "abc", map[string]any{"name": "X"}).ExpectStatus(http.StatusNotFound)
	if got := listDepartments(t, admin); len(got) != 1 || got[0].Name != "Technology" {
		t.Fatalf("departments after refused writes = %+v", got)
	}
}

func TestAdministratorRenamesAndMovesDepartments(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	tech := createDept(t, admin, "Technology", "")
	ops := createDept(t, admin, "Operations", "")
	payments := createDept(t, admin, "Payments", tech.ID)

	var got deptBody
	patchDept(admin, payments.ID, map[string]any{"name": " Payments Platform "}).ExpectStatus(http.StatusOK).JSON(&got)
	if got.Name != "Payments Platform" || *got.ParentID != tech.ID {
		t.Fatalf("after rename = %+v", got)
	}

	// Move under another department, then to the top level (null), then leave the parent alone.
	patchDept(admin, payments.ID, map[string]any{"parentId": ops.ID}).ExpectStatus(http.StatusOK).JSON(&got)
	if got.Name != "Payments Platform" || got.ParentID == nil || *got.ParentID != ops.ID {
		t.Fatalf("after move = %+v", got)
	}
	patchDept(admin, payments.ID, map[string]any{"parentId": nil}).ExpectStatus(http.StatusOK).JSON(&got)
	if got.ParentID != nil {
		t.Fatalf("after move to the top = %+v", got)
	}
	patchDept(admin, payments.ID, map[string]any{"parentId": ops.ID, "name": "Pay"}).ExpectStatus(http.StatusOK).JSON(&got)
	if got.Name != "Pay" || *got.ParentID != ops.ID {
		t.Fatalf("after rename and move = %+v", got)
	}
	patchDept(admin, payments.ID, map[string]any{}).ExpectStatus(http.StatusOK)
	if again := findDept(t, listDepartments(t, admin), payments.ID); again.Name != "Pay" || *again.ParentID != ops.ID {
		t.Fatalf("after an empty edit = %+v", again)
	}
}

func TestADepartmentCannotMoveIntoItselfOrItsDescendants(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	a := createDept(t, admin, "A", "")
	b := createDept(t, admin, "B", a.ID)
	c := createDept(t, admin, "C", b.ID)
	other := createDept(t, admin, "Other", "")

	patchDept(admin, a.ID, map[string]any{"parentId": a.ID}).ExpectError(http.StatusUnprocessableEntity, "parentId", "departmentCycle")
	patchDept(admin, a.ID, map[string]any{"parentId": b.ID}).ExpectError(http.StatusUnprocessableEntity, "parentId", "departmentCycle")
	patchDept(admin, a.ID, map[string]any{"parentId": c.ID}).ExpectError(http.StatusUnprocessableEntity, "parentId", "departmentCycle")
	patchDept(admin, b.ID, map[string]any{"parentId": c.ID}).ExpectError(http.StatusUnprocessableEntity, "parentId", "departmentCycle")

	items := listDepartments(t, admin)
	if got := findDept(t, items, a.ID); got.ParentID != nil {
		t.Fatalf("A = %+v, want still at the top", got)
	}
	if got := findDept(t, items, c.ID); *got.ParentID != b.ID {
		t.Fatalf("C = %+v", got)
	}
	// A whole subtree can move elsewhere; its members follow it implicitly.
	patchDept(admin, a.ID, map[string]any{"parentId": other.ID}).ExpectStatus(http.StatusOK)
	if got := findDept(t, listDepartments(t, admin), c.ID); *got.ParentID != b.ID {
		t.Fatalf("C after moving A = %+v", got)
	}
}

func TestADepartmentWithChildrenCannotBeDeleted(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	tech := createDept(t, admin, "Technology", "")
	payments := createDept(t, admin, "Payments", tech.ID)

	e := deleteDept(admin, tech.ID).ExpectError(http.StatusUnprocessableEntity, "", "departmentNotEmpty")
	if e.Params["children"] != float64(1) || e.Params["members"] != float64(0) {
		t.Fatalf("params = %v", e.Params)
	}
	if len(listDepartments(t, admin)) != 2 {
		t.Fatal("the department was deleted")
	}

	deleteDept(admin, payments.ID).ExpectStatus(http.StatusNoContent)
	deleteDept(admin, tech.ID).ExpectStatus(http.StatusNoContent)
	if got := listDepartments(t, admin); len(got) != 0 {
		t.Fatalf("departments = %+v", got)
	}
	deleteDept(admin, tech.ID).ExpectStatus(http.StatusNotFound)
}

func TestDepartmentsOfTheSameNameMayExistInDifferentPlaces(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	a := createDept(t, admin, "A", "")
	b := createDept(t, admin, "B", "")
	createDept(t, admin, "Support", a.ID)
	createDept(t, admin, "Support", b.ID)
	if got := listDepartments(t, admin); len(got) != 4 {
		t.Fatalf("departments = %+v", got)
	}
}

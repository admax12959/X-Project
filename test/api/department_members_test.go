package api

import (
	"net/http"
	"slices"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// deptOfUser reads the organization department of a user from the administrator's user list.
type userDeptBody struct {
	Username     string  `json:"username"`
	DepartmentID *string `json:"departmentId"`
}

func listUserDepts(t *testing.T, c *apitest.Client, query string) map[string]*string {
	t.Helper()
	var l struct {
		Items []userDeptBody `json:"items"`
	}
	c.Get("/api/users" + query).ExpectStatus(http.StatusOK).JSON(&l)
	out := map[string]*string{}
	for _, u := range l.Items {
		out[u.Username] = u.DepartmentID
	}
	return out
}

func usernames(m map[string]*string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func TestAdministratorPutsAUserIntoADepartmentAndTakesThemOut(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	tech := createDept(t, admin, "Technology", "")
	risk := createDept(t, admin, "Risk", "")
	jane := createUser(t, admin, map[string]any{"username": "jdoe"})
	if got := listUserDepts(t, admin, "")["jdoe"]; got != nil {
		t.Fatalf("a new user is in department %s", *got)
	}

	var edited userDeptBody
	patchUser(admin, jane.ID, map[string]any{"departmentId": tech.ID}).ExpectStatus(http.StatusOK).JSON(&edited)
	if edited.DepartmentID == nil || *edited.DepartmentID != tech.ID {
		t.Fatalf("edited = %+v", edited)
	}
	if got := findDept(t, listDepartments(t, admin), tech.ID); got.MemberCount != 1 {
		t.Fatalf("tech = %+v, want one member", got)
	}
	// Moving between departments, then leaving it alone, then taking the user out.
	patchUser(admin, jane.ID, map[string]any{"departmentId": risk.ID}).ExpectStatus(http.StatusOK)
	patchUser(admin, jane.ID, map[string]any{"phone": "555-0100"}).ExpectStatus(http.StatusOK)
	if got := listUserDepts(t, admin, "")["jdoe"]; got == nil || *got != risk.ID {
		t.Fatalf("department = %v, want %s", got, risk.ID)
	}
	patchUser(admin, jane.ID, map[string]any{"departmentId": nil}).ExpectStatus(http.StatusOK).JSON(&edited)
	if edited.DepartmentID != nil {
		t.Fatalf("edited = %+v", edited)
	}

	// The user sees their own department in the profile; they cannot choose it themselves.
	patchUser(admin, jane.ID, map[string]any{"departmentId": tech.ID}).ExpectStatus(http.StatusOK)
	janeC := env.Client()
	janeC.LoginAs("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)
	var me userDeptBody
	janeC.Get("/api/me").ExpectStatus(http.StatusOK).JSON(&me)
	if me.DepartmentID == nil || *me.DepartmentID != tech.ID {
		t.Fatalf("me = %+v", me)
	}
	patchMe(janeC, map[string]any{"departmentId": risk.ID}).ExpectStatus(http.StatusOK)
	janeC.Get("/api/me").ExpectStatus(http.StatusOK).JSON(&me)
	if *me.DepartmentID != tech.ID {
		t.Fatalf("a user changed their own department: %+v", me)
	}
	patchUser(janeC, jane.ID, map[string]any{"departmentId": risk.ID}).ExpectStatus(http.StatusForbidden)
}

func TestTheDepartmentOfAUserIsValidatedAndLDAPUsersCanHaveOne(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	tech := createDept(t, admin, "Technology", "")
	jane := createUser(t, admin, map[string]any{"username": "jdoe"})

	patchUser(admin, jane.ID, map[string]any{"departmentId": "9999"}).ExpectError(http.StatusUnprocessableEntity, "departmentId", "invalid")
	patchUser(admin, jane.ID, map[string]any{"departmentId": "abc"}).ExpectError(http.StatusUnprocessableEntity, "departmentId", "invalid")
	admin.Post("/api/users", newUser(map[string]any{"username": "kim", "departmentId": "9999"})).
		ExpectError(http.StatusUnprocessableEntity, "departmentId", "invalid")

	// Directory-owned fields are read-only for an LDAP user, the department in the organization is not.
	addEmma(env)
	emma := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
	patchUser(admin, emma.ID, map[string]any{"departmentId": tech.ID}).ExpectStatus(http.StatusOK)
	if got := listUserDepts(t, admin, "")["elarsen"]; got == nil || *got != tech.ID {
		t.Fatalf("emma's department = %v", got)
	}
	// An LDAP login syncs the directory department and leaves the organization alone.
	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)
	if got := listUserDepts(t, admin, "")["elarsen"]; got == nil || *got != tech.ID {
		t.Fatalf("emma's department after login = %v", got)
	}

	// A user can be created straight into a department.
	var kim userDeptBody
	admin.Post("/api/users", newUser(map[string]any{"username": "kim", "departmentId": tech.ID})).ExpectStatus(http.StatusCreated).JSON(&kim)
	if kim.DepartmentID == nil || *kim.DepartmentID != tech.ID {
		t.Fatalf("kim = %+v", kim)
	}
}

func TestADepartmentWithMembersCannotBeDeletedUntilTheyLeave(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	tech := createDept(t, admin, "Technology", "")
	jane := createUser(t, admin, map[string]any{"username": "jdoe"})
	patchUser(admin, jane.ID, map[string]any{"departmentId": tech.ID}).ExpectStatus(http.StatusOK)

	e := deleteDept(admin, tech.ID).ExpectError(http.StatusUnprocessableEntity, "", "departmentNotEmpty")
	if e.Params["members"] != float64(1) || e.Params["children"] != float64(0) {
		t.Fatalf("params = %v", e.Params)
	}
	// A disabled user is still a member.
	admin.Post("/api/users/"+jane.ID+"/disable", nil).ExpectStatus(http.StatusOK)
	deleteDept(admin, tech.ID).ExpectError(http.StatusUnprocessableEntity, "", "departmentNotEmpty")

	patchUser(admin, jane.ID, map[string]any{"departmentId": nil}).ExpectStatus(http.StatusOK)
	deleteDept(admin, tech.ID).ExpectStatus(http.StatusNoContent)
}

// A fixture: Technology > Payments > Cards, Technology > Platform, Risk; one user in each of
// Payments, Cards and Risk, one in no department.
type orgFixture struct {
	env                         *apitest.Env
	admin                       *apitest.Client
	tech, payments, cards, risk deptBody
	platform                    deptBody
}

func newOrgFixture(t *testing.T) orgFixture {
	t.Helper()
	env := apitest.New(t)
	admin := env.AdminClient()
	f := orgFixture{env: env, admin: admin}
	f.tech = createDept(t, admin, "Technology", "")
	f.payments = createDept(t, admin, "Payments", f.tech.ID)
	f.cards = createDept(t, admin, "Cards", f.payments.ID)
	f.platform = createDept(t, admin, "Platform", f.tech.ID)
	f.risk = createDept(t, admin, "Risk", "")
	for name, dept := range map[string]string{"pat": f.payments.ID, "carl": f.cards.ID, "rita": f.risk.ID, "nora": ""} {
		u := createUser(t, admin, map[string]any{"username": name, "displayName": name})
		if dept != "" {
			patchUser(admin, u.ID, map[string]any{"departmentId": dept}).ExpectStatus(http.StatusOK)
		}
	}
	return f
}

func TestUserListFiltersByDepartmentIncludingSubDepartments(t *testing.T) {
	t.Parallel()
	f := newOrgFixture(t)

	want := func(dept deptBody, names ...string) {
		t.Helper()
		if got := usernames(listUserDepts(t, f.admin, "?departmentId="+dept.ID)); !slices.Equal(got, names) {
			t.Errorf("users in %s = %v, want %v", dept.Name, got, names)
		}
	}
	want(f.tech, "carl", "pat")
	want(f.payments, "carl", "pat")
	want(f.cards, "carl")
	want(f.platform)
	want(f.risk, "rita")
	if all := usernames(listUserDepts(t, f.admin, "")); len(all) != 5 {
		t.Fatalf("unfiltered = %v", all)
	}
	f.admin.Get("/api/users?departmentId=9999").ExpectError(http.StatusUnprocessableEntity, "departmentId", "invalid")
	f.admin.Get("/api/users?departmentId=abc").ExpectError(http.StatusUnprocessableEntity, "departmentId", "invalid")

	// Moving a department moves the filter result with it.
	patchDept(f.admin, f.payments.ID, map[string]any{"parentId": f.risk.ID}).ExpectStatus(http.StatusOK)
	want(f.tech)
	want(f.risk, "carl", "pat", "rita")
}

func TestOwnersFilterThePickerByDepartment(t *testing.T) {
	t.Parallel()
	f := newOrgFixture(t)
	owner := plainUserClient(t, f.env, "olga") // not an administrator

	assignable := func(query string) []string {
		t.Helper()
		var l struct {
			Items []map[string]any `json:"items"`
		}
		owner.Get("/api/assignable-users" + query).ExpectStatus(http.StatusOK).JSON(&l)
		var names []string
		for _, it := range l.Items {
			if len(it) != 3 {
				t.Fatalf("item = %v, want id, username and displayName only", it)
			}
			names = append(names, it["username"].(string))
		}
		slices.Sort(names)
		return names
	}
	if got := assignable("?departmentId=" + f.tech.ID); !slices.Equal(got, []string{"carl", "pat"}) {
		t.Errorf("technology = %v", got)
	}
	if got := assignable("?departmentId=" + f.risk.ID); !slices.Equal(got, []string{"rita"}) {
		t.Errorf("risk = %v", got)
	}
	// Disabled users are still left out.
	var users []userBody = listUsers(t, f.admin)
	f.admin.Post("/api/users/"+findUser(t, users, "carl").ID+"/disable", nil).ExpectStatus(http.StatusOK)
	if got := assignable("?departmentId=" + f.tech.ID); !slices.Equal(got, []string{"pat"}) {
		t.Errorf("technology after disabling carl = %v", got)
	}
	if got := assignable(""); len(got) != 5 { // admin, olga, pat, rita, nora
		t.Errorf("unfiltered = %v", got)
	}
	owner.Get("/api/assignable-users?departmentId=9999").ExpectError(http.StatusUnprocessableEntity, "departmentId", "invalid")
}

func TestDepartmentChangesAndMembershipAdjustmentsAreAudited(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	adminUser := findUser(t, listUsers(t, admin), apitest.AdminUsername)

	tech := createDept(t, admin, "Technology", "")
	payments := createDept(t, admin, "Payments", tech.ID)
	patchDept(admin, payments.ID, map[string]any{"name": "Payments Platform"}).ExpectStatus(http.StatusOK)
	patchDept(admin, payments.ID, map[string]any{"parentId": nil}).ExpectStatus(http.StatusOK)
	// No change, no entry.
	patchDept(admin, payments.ID, map[string]any{"name": "Payments Platform", "parentId": nil}).ExpectStatus(http.StatusOK)
	jane := createUser(t, admin, map[string]any{"username": "jdoe"})
	patchUser(admin, jane.ID, map[string]any{"departmentId": tech.ID}).ExpectStatus(http.StatusOK)
	patchUser(admin, jane.ID, map[string]any{"departmentId": tech.ID}).ExpectStatus(http.StatusOK)
	patchUser(admin, jane.ID, map[string]any{"departmentId": payments.ID}).ExpectStatus(http.StatusOK)
	patchUser(admin, jane.ID, map[string]any{"departmentId": nil}).ExpectStatus(http.StatusOK)
	deleteDept(admin, payments.ID).ExpectStatus(http.StatusNoContent)
	deleteDept(admin, tech.ID).ExpectStatus(http.StatusNoContent)
	// Refused operations leave no entry.
	deleteDept(admin, "9999").ExpectStatus(http.StatusNotFound)
	patchDept(plainUserClient(t, env, "bob"), tech.ID, map[string]any{"name": "x"}).ExpectStatus(http.StatusForbidden)

	ref := func(v any) map[string]any {
		m, _ := v.(map[string]any)
		return m
	}
	created := auditOf(t, admin, "departmentCreated") // newest first
	if len(created) != 2 {
		t.Fatalf("departmentCreated = %+v", created)
	}
	c := created[0] // Payments, under Technology
	if c.Actor.ID != adminUser.ID || c.Target.Type != "department" || c.Target.ID != payments.ID || c.Params["name"] != "Payments" ||
		ref(c.Params["parent"])["id"] != tech.ID || ref(c.Params["parent"])["name"] != "Technology" {
		t.Errorf("created Payments = %+v", c)
	}
	if created[1].Params["parent"] != nil || created[1].Params["name"] != "Technology" {
		t.Errorf("created Technology = %+v", created[1])
	}

	renamed := auditOf(t, admin, "departmentRenamed")
	if len(renamed) != 1 || renamed[0].Target.ID != payments.ID || renamed[0].Params["from"] != "Payments" || renamed[0].Params["to"] != "Payments Platform" {
		t.Errorf("renamed = %+v", renamed)
	}
	moved := auditOf(t, admin, "departmentMoved")
	if len(moved) != 1 || moved[0].Params["name"] != "Payments Platform" || ref(moved[0].Params["from"])["name"] != "Technology" || moved[0].Params["to"] != nil {
		t.Errorf("moved = %+v", moved)
	}
	deleted := auditOf(t, admin, "departmentDeleted")
	if len(deleted) != 2 || deleted[0].Target.ID != tech.ID || deleted[1].Target.ID != payments.ID || deleted[1].Params["name"] != "Payments Platform" {
		t.Errorf("deleted = %+v", deleted)
	}
	// The entries about a department stay after it is gone, and can be found by their object.
	if got := auditLog(t, admin, "targetType", "department", "targetId", payments.ID); got.Total != 4 {
		t.Errorf("entries about Payments = %d, want created, renamed, moved, deleted", got.Total)
	}

	members := auditOf(t, admin, "userDepartmentChanged")
	if len(members) != 3 {
		t.Fatalf("userDepartmentChanged = %+v", members)
	}
	// Newest first: out of Payments, Technology to Payments, none to Technology.
	if members[0].Target.Type != "user" || members[0].Target.ID != jane.ID || members[0].Target.Username != "jdoe" ||
		ref(members[0].Params["from"])["name"] != "Payments Platform" || members[0].Params["to"] != nil {
		t.Errorf("leaving = %+v", members[0])
	}
	if ref(members[1].Params["from"])["name"] != "Technology" || ref(members[1].Params["to"])["id"] != payments.ID {
		t.Errorf("moving = %+v", members[1])
	}
	if members[2].Params["from"] != nil || ref(members[2].Params["to"])["name"] != "Technology" {
		t.Errorf("joining = %+v", members[2])
	}
}

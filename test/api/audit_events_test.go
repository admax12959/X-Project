package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// One test per recorded operation: what was done, by whom, to what, with which parameters.
// (Creating a user is in audit_log_test.go.)

func TestChangingALoginEmailIsRecordedWithTheOldAndTheNewAddress(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	jane := createUser(t, admin, map[string]any{"username": "jdoe"})

	patchUser(admin, jane.ID, map[string]any{"loginEmail": " Jane.New@Example.com "}).ExpectStatus(http.StatusOK)
	// Setting the address it already has is not a change.
	patchUser(admin, jane.ID, map[string]any{"loginEmail": "jane.new@example.com"}).ExpectStatus(http.StatusOK)
	// A profile edit that leaves the login email alone is not recorded here.
	patchUser(admin, jane.ID, map[string]any{"phone": "555-0100"}).ExpectStatus(http.StatusOK)

	got := auditOf(t, admin, "loginEmailChanged")
	if len(got) != 1 {
		t.Fatalf("loginEmailChanged entries = %+v, want exactly one", got)
	}
	e := got[0]
	if e.Actor.Username != apitest.AdminUsername || e.Target.Type != "user" || e.Target.ID != jane.ID {
		t.Errorf("entry = %+v, want the administrator changing jdoe", e)
	}
	if e.Params["from"] != apitest.Email("jdoe") || e.Params["to"] != "jane.new@example.com" {
		t.Errorf("params = %+v, want from %s to jane.new@example.com", e.Params, apitest.Email("jdoe"))
	}
}

func TestBindingAndUnbindingIdentitiesIsRecordedForBothTheUserAndTheAdministrator(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addJane(env)
	jane, janeUser := localJane(t, env)
	admin := env.AdminClient()

	bindOwn(jane, bindLDAP("jane.doe", "jane-ldap-pw")).ExpectStatus(http.StatusCreated)
	unbindOwn(jane, "ldap").ExpectStatus(http.StatusOK)
	adminBind(admin, janeUser.ID, map[string]any{"kind": "ldap", "loginName": "jane.doe"}).ExpectStatus(http.StatusCreated)

	// A refused bind leaves no trace.
	bindOwn(jane, bindLDAP("jane.doe", "wrong")).ExpectStatus(http.StatusUnprocessableEntity)

	bound := auditOf(t, admin, "identityBound")
	if len(bound) != 2 {
		t.Fatalf("identityBound entries = %+v, want two", bound)
	}
	// Newest first: the administrator's bind, then the user's own.
	byAdmin, bySelf := bound[0], bound[1]
	if byAdmin.Actor.Username != apitest.AdminUsername || byAdmin.Params["via"] != "admin" || byAdmin.Params["kind"] != "ldap" || byAdmin.Params["loginName"] != "jane.doe" {
		t.Errorf("administrator's bind = %+v", byAdmin)
	}
	if bySelf.Actor.ID != janeUser.ID || bySelf.Target.ID != janeUser.ID || bySelf.Params["via"] != "self" || bySelf.Params["loginName"] != "jane.doe" {
		t.Errorf("user's own bind = %+v", bySelf)
	}

	unbound := auditOf(t, admin, "identityUnbound")
	if len(unbound) != 1 {
		t.Fatalf("identityUnbound entries = %+v, want one", unbound)
	}
	if u := unbound[0]; u.Actor.ID != janeUser.ID || u.Target.ID != janeUser.ID || u.Params["kind"] != "ldap" || u.Params["loginName"] != "jane.doe" {
		t.Errorf("unbind = %+v", u)
	}
}

func TestAdministratorBindingAnEmailIdentityIsRecordedWithoutThePassword(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	emma := userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
	admin := env.AdminClient()

	adminBind(admin, emma.ID, map[string]any{"kind": "email", "loginName": "emma.private@example.com", "password": "set-by-admin-1"}).ExpectStatus(http.StatusCreated)

	got := auditOf(t, admin, "identityBound")
	if len(got) != 1 || got[0].Params["kind"] != "email" || got[0].Params["loginName"] != "emma.private@example.com" || got[0].Target.ID != emma.ID {
		t.Fatalf("identityBound entries = %+v, want the email identity bound to Emma", got)
	}
	if _, leaked := got[0].Params["password"]; leaked {
		t.Fatalf("params contain the password: %+v", got[0].Params)
	}
}

func TestFirstLDAPLoginWritesNoAuditEntry(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)

	if p := auditLog(t, env.AdminClient()); p.Total != 0 {
		t.Fatalf("audit log after a first LDAP login = %+v, want empty", p.Items)
	}
}

func TestOwnerAndPEListChangesAreRecordedPerPersonAndSurviveTheApplication(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)
	carol := createUser(t, f.admin, map[string]any{"username": "carol", "displayName": "Carol"})

	// Add carol as PE in praman's place, and bob as a second Owner.
	f.admin.Put(f.appPath+"/members", map[string]any{
		"owners": []string{f.owner.ID, f.bob.ID}, "pes": []string{carol.ID},
	}).ExpectStatus(http.StatusOK)

	added := auditOf(t, f.admin, "appMemberAdded")
	removed := auditOf(t, f.admin, "appMemberRemoved")
	if len(added) != 2 || len(removed) != 1 {
		t.Fatalf("added = %+v, removed = %+v, want two added and one removed", added, removed)
	}
	r := removed[0]
	if r.Actor.Username != apitest.AdminUsername || r.Target.Type != "application" || r.Target.ID != "payments-core" || r.Params["role"] != "pe" {
		t.Errorf("removal = %+v", r)
	}
	if u, _ := r.Params["user"].(map[string]any); u["userId"] != f.pe.ID || u["username"] != "praman" {
		t.Errorf("removal user = %+v, want praman", r.Params["user"])
	}
	if app, _ := r.Params["application"].(map[string]any); app["key"] != "payments-core" || app["name"] == "" {
		t.Errorf("removal application = %+v", r.Params["application"])
	}
	roles := map[string]string{}
	for _, a := range added {
		u, _ := a.Params["user"].(map[string]any)
		roles[u["username"].(string)] = a.Params["role"].(string)
	}
	if len(roles) != 2 || roles["carol"] != "pe" || roles["bob"] != "owner" {
		t.Errorf("added roles = %v, want carol pe and bob owner", roles)
	}

	// Filtering by the application finds them; deleting the application keeps them.
	if byApp := auditLog(t, f.admin, "targetType", "application", "targetId", "payments-core"); byApp.Total != 3 {
		t.Fatalf("entries for payments-core = %d, want 3", byApp.Total)
	}
	f.admin.Do(http.MethodDelete, f.appPath, map[string]string{"confirmKey": "payments-core"}).ExpectStatus(http.StatusNoContent)
	if got := auditLog(t, f.admin, "targetType", "application", "targetId", "payments-core"); got.Total != 3 {
		t.Fatalf("entries for payments-core after deleting it = %d, want 3", got.Total)
	}
}

func TestAnOwnerEditingTheirOwnApplicationsListsIsRecordedWithTheOwnerAsOperator(t *testing.T) {
	t.Parallel()
	f := newMaintFixture(t)

	f.ownerC.Put(f.appPath+"/members", map[string]any{
		"owners": []string{f.owner.ID}, "pes": []string{f.pe.ID, f.bob.ID},
	}).ExpectStatus(http.StatusOK)

	got := auditOf(t, f.admin, "appMemberAdded")
	if len(got) != 1 || got[0].Actor.ID != f.owner.ID || got[0].Params["role"] != "pe" {
		t.Fatalf("appMemberAdded entries = %+v, want Emma adding bob as PE", got)
	}
}

func TestChangingTheAdministratorFlagIsRecordedAndTakesEffect(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	jane := createUser(t, admin, map[string]any{"username": "jdoe"})
	janeC := env.Client()
	janeC.LoginAs("jdoe", "initial-pw-1").ExpectStatus(http.StatusOK)
	janeC.Get("/api/users").ExpectStatus(http.StatusForbidden)

	var promoted userBody
	patchUser(admin, jane.ID, map[string]any{"isAdmin": true}).ExpectStatus(http.StatusOK).JSON(&promoted)
	if !promoted.IsAdmin {
		t.Fatalf("user after granting = %+v, want an administrator", promoted)
	}
	janeC.Get("/api/users").ExpectStatus(http.StatusOK)
	// Sending the current value is not a change.
	patchUser(admin, jane.ID, map[string]any{"isAdmin": true}).ExpectStatus(http.StatusOK)

	// She can take the flag away again, even from herself, while another administrator remains.
	patchUser(janeC, jane.ID, map[string]any{"isAdmin": false}).ExpectStatus(http.StatusOK)
	janeC.Get("/api/users").ExpectStatus(http.StatusForbidden)

	got := auditOf(t, admin, "adminFlagChanged")
	if len(got) != 2 {
		t.Fatalf("adminFlagChanged entries = %+v, want two", got)
	}
	revoked, granted := got[0], got[1]
	if granted.Actor.Username != apitest.AdminUsername || granted.Target.ID != jane.ID || granted.Params["from"] != false || granted.Params["to"] != true {
		t.Errorf("grant = %+v", granted)
	}
	if revoked.Actor.ID != jane.ID || revoked.Params["from"] != true || revoked.Params["to"] != false {
		t.Errorf("revocation = %+v", revoked)
	}
}

func TestTheLastEnabledAdministratorCannotLoseTheFlag(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	adminUser := findUser(t, listUsers(t, admin), apitest.AdminUsername)

	patchUser(admin, adminUser.ID, map[string]any{"isAdmin": false}).ExpectError(http.StatusUnprocessableEntity, "isAdmin", "lastAdmin")

	if !findUser(t, listUsers(t, admin), apitest.AdminUsername).IsAdmin {
		t.Fatal("the administrator lost the flag")
	}
	if p := auditLog(t, admin, "action", "adminFlagChanged"); p.Total != 0 {
		t.Fatalf("a refused change was recorded: %+v", p.Items)
	}
}

func TestADisabledAdministratorDoesNotCountAsAnotherAdministrator(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	adminUser := findUser(t, listUsers(t, admin), apitest.AdminUsername)
	jane := createUser(t, admin, map[string]any{"username": "jdoe"})
	patchUser(admin, jane.ID, map[string]any{"isAdmin": true}).ExpectStatus(http.StatusOK)
	admin.Post("/api/users/"+jane.ID+"/disable", nil).ExpectStatus(http.StatusOK)

	patchUser(admin, adminUser.ID, map[string]any{"isAdmin": false}).ExpectError(http.StatusUnprocessableEntity, "isAdmin", "lastAdmin")
}

func TestOnlyAdministratorsMayChangeTheAdministratorFlag(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	jane := plainUserClient(t, env, "jdoe")
	me := getMe(t, jane)

	patchUser(jane, me.ID, map[string]any{"isAdmin": true}).ExpectStatus(http.StatusForbidden)
}

func TestAnOperationThatFailsLeavesNoAuditEntry(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	createUser(t, admin, map[string]any{"username": "jdoe"})
	before := auditLog(t, admin).Total

	admin.Post("/api/users", newUser(map[string]any{"username": "other", "loginEmail": apitest.Email("jdoe")})).
		ExpectError(http.StatusUnprocessableEntity, "loginEmail", "loginEmailTaken")

	if got := auditLog(t, admin).Total; got != before {
		t.Fatalf("audit entries = %d after a refused creation, want %d", got, before)
	}
}

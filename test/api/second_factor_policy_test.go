package api

import (
	"net/http"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// The enforcement policy (#23, ADR-0006): administrators always need a second factor; an
// administrator can require it of everyone. A required user who has none can do nothing but
// set it up.

const securityPolicyPath = "/api/system-config/security"

type securityPolicyBody struct {
	ForceSecondFactor bool   `json:"forceSecondFactor"`
	UpdatedAt         string `json:"updatedAt"`
}

func forceAll(t *testing.T, admin *apitest.Client, on bool) {
	t.Helper()
	admin.Put(securityPolicyPath, map[string]bool{"forceSecondFactor": on}).ExpectStatus(http.StatusOK)
}

func TestThePolicyIsOffByDefaultAndOnlyAdministratorsSeeAndChangeIt(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	var p securityPolicyBody
	admin.Get(securityPolicyPath).ExpectStatus(http.StatusOK).JSON(&p)
	if p.ForceSecondFactor {
		t.Fatalf("policy = %+v, want off by default", p)
	}
	var on securityPolicyBody
	admin.Put(securityPolicyPath, map[string]bool{"forceSecondFactor": true}).ExpectStatus(http.StatusOK).JSON(&on)
	if !on.ForceSecondFactor || on.UpdatedAt == "" {
		t.Fatalf("saved policy = %+v, want on with a time", on)
	}
	admin.Get(securityPolicyPath).ExpectStatus(http.StatusOK).JSON(&p)
	if !p.ForceSecondFactor {
		t.Fatalf("policy = %+v, want it to stay on", p)
	}

	forceAll(t, admin, false)
	tina := plainUserClient(t, env, "tina")
	tina.Get(securityPolicyPath).ExpectError(http.StatusForbidden, "", "forbidden")
	tina.Put(securityPolicyPath, map[string]bool{"forceSecondFactor": true}).ExpectError(http.StatusForbidden, "", "forbidden")
	env.Client().Get(securityPolicyPath).ExpectError(http.StatusUnauthorized, "", "unauthenticated")
}

func TestChangingThePolicyIsAuditedAndAnUnchangedSaveIsNot(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	forceAll(t, admin, true)
	forceAll(t, admin, true)
	forceAll(t, admin, false)

	var security []auditEntryBody
	for _, e := range auditOf(t, admin, "configChanged") {
		if e.Target.ID == "security" {
			security = append(security, e)
		}
	}
	if len(security) != 2 {
		t.Fatalf("security entries = %+v, want two (on, off) and none for the unchanged save", security)
	}
	e := security[0]
	changes, _ := e.Params["changes"].([]any)
	if e.Actor.Username != apitest.AdminUsername || e.Target.Type != "systemConfig" || e.Params["section"] != "security" || len(changes) != 1 || changes[0] != "forceSecondFactor" {
		t.Errorf("entry = %+v, want admin changing security.forceSecondFactor", e)
	}
}

func TestWhileThePolicyIsOnAUserWithASecondFactorCannotTurnItOff(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	secret, _, tina := tinaWithCodes(t, env)
	admin := env.AdminClient()

	forceAll(t, admin, true)
	env.Clock.Advance(totpStep)
	disable(tina, nowCode(t, env, secret)).ExpectError(http.StatusForbidden, "", "secondFactorEnforced")
	if !secondFactorOf(t, tina).Enabled {
		t.Fatal("a refused disable turned the second factor off")
	}

	forceAll(t, admin, false)
	disable(tina, nowCode(t, env, secret)).ExpectStatus(http.StatusOK)
}

func TestAnAdministratorCannotTurnTheSecondFactorOffWhateverThePolicySays(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	admin := env.AdminClient() // has a second factor
	env.Clock.Advance(totpStep)

	disable(admin, apitest.TOTPCode(env.AdminTOTPSecret(), env.Clock.Now())).ExpectError(http.StatusForbidden, "", "secondFactorEnforced")
	if !secondFactorOf(t, admin).Enabled {
		t.Fatal("the administrator's second factor was turned off")
	}

	// A user who is made an administrator is bound by the same rule.
	secret, _, tina := tinaWithCodes(t, env)
	u := findUser(t, listUsers(t, admin), "tina")
	admin.Do(http.MethodPatch, "/api/users/"+u.ID, map[string]any{"isAdmin": true}).ExpectStatus(http.StatusOK)
	env.Clock.Advance(totpStep)
	disable(tina, nowCode(t, env, secret)).ExpectError(http.StatusForbidden, "", "secondFactorEnforced")
}

// ---------------------------------------------------------------- the gate

// setupOnly are the calls a required user without a second factor may still make.
func requireSetupOnly(t *testing.T, c *apitest.Client) {
	t.Helper()
	for _, path := range []string{"/api/applications", "/api/me", "/api/users", "/api/code-platforms", "/api/audit-log", securityPolicyPath} {
		c.Get(path).ExpectError(http.StatusForbidden, "", "secondFactorSetupRequired")
	}
	c.Do(http.MethodPatch, "/api/me", map[string]string{"language": "en"}).ExpectError(http.StatusForbidden, "", "secondFactorSetupRequired")
	c.Post(recoveryCodesPath, map[string]string{"code": "123456"}).ExpectError(http.StatusForbidden, "", "secondFactorSetupRequired")
	c.Post(disablePath, map[string]string{"code": "123456"}).ExpectError(http.StatusForbidden, "", "secondFactorSetupRequired")
	c.Get("/api/no-such-route").ExpectError(http.StatusForbidden, "", "secondFactorSetupRequired")
}

type sessionWithGate struct {
	User struct {
		Username string `json:"username"`
	} `json:"user"`
	SecondFactorSetupRequired bool `json:"secondFactorSetupRequired"`
}

func TestAForcedUserWithoutASecondFactorCanOnlyEnterTheSetupFlow(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	tina := plainUserClient(t, env, "tina")
	forceAll(t, env.AdminClient(), true)

	requireSetupOnly(t, tina)

	// What the setup flow needs stays open: who she is, the status, starting and confirming.
	var s sessionWithGate
	tina.Get("/api/session").ExpectStatus(http.StatusOK).JSON(&s)
	if s.User.Username != "tina" || !s.SecondFactorSetupRequired {
		t.Fatalf("session = %+v, want tina with secondFactorSetupRequired", s)
	}
	if secondFactorOf(t, tina).Enabled {
		t.Fatal("the status says enabled")
	}
	_, codes := enableWithCodes(t, tina, "initial-pw-1")
	if len(codes) != 10 {
		t.Fatalf("recovery codes = %v", codes)
	}

	// Done: everything is open again, and the session no longer says setup is needed.
	tina.Get("/api/applications").ExpectStatus(http.StatusOK)
	tina.Get("/api/me").ExpectStatus(http.StatusOK)
	tina.Get("/api/session").ExpectStatus(http.StatusOK).JSON(&s)
	if s.SecondFactorSetupRequired {
		t.Fatalf("session = %+v, want setup no longer required", s)
	}
}

func TestTheLoginAnswerSaysThatSetupIsRequired(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	plainUserClient(t, env, "tina")
	forceAll(t, env.AdminClient(), true)

	var s sessionWithGate
	c := env.Client()
	c.LoginAs("tina", "initial-pw-1").ExpectStatus(http.StatusOK).JSON(&s)
	if !s.SecondFactorSetupRequired {
		t.Fatalf("login = %+v, want secondFactorSetupRequired", s)
	}
	requireSetupOnly(t, c)
}

func TestAnAdministratorWithoutASecondFactorIsGatedWithoutAnyPolicy(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	c := env.Client()
	var s sessionWithGate
	c.LoginEmail(apitest.AdminEmail, apitest.AdminPassword).ExpectStatus(http.StatusOK).JSON(&s)
	if !s.SecondFactorSetupRequired {
		t.Fatalf("login = %+v, want secondFactorSetupRequired", s)
	}
	requireSetupOnly(t, c)
	enableWithCodes(t, c, apitest.AdminPassword)
	c.Get("/api/users").ExpectStatus(http.StatusOK)
}

func TestWithThePolicyOffAUserWithoutASecondFactorIsNotGated(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	tina := plainUserClient(t, env, "tina")
	forceAll(t, env.AdminClient(), true)
	forceAll(t, env.AdminClient(), false)
	tina.Get("/api/applications").ExpectStatus(http.StatusOK)
}

func TestTurningThePolicyOnGatesSessionsThatAreAlreadyOpen(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	tina := plainUserClient(t, env, "tina")
	tina.Get("/api/applications").ExpectStatus(http.StatusOK)
	forceAll(t, env.AdminClient(), true)
	tina.Get("/api/applications").ExpectError(http.StatusForbidden, "", "secondFactorSetupRequired")
}

func TestAPasswordChangeAndASetupRequirementCanBothApply(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	admin := env.AdminClient()
	createUserRaw(t, admin, map[string]any{"username": "tina", "displayName": "tina"}) // password chosen by the admin: must change it
	forceAll(t, admin, true)

	tina := env.Client()
	var s struct {
		MustChangePassword        bool `json:"mustChangePassword"`
		SecondFactorSetupRequired bool `json:"secondFactorSetupRequired"`
	}
	tina.LoginAs("tina", "initial-pw-1").ExpectStatus(http.StatusOK).JSON(&s)
	if !s.MustChangePassword || !s.SecondFactorSetupRequired {
		t.Fatalf("login = %+v, want both", s)
	}
	tina.Get("/api/applications").ExpectError(http.StatusForbidden, "", "passwordChangeRequired")
	changePassword(tina, "initial-pw-1", "a-new-password-1").ExpectStatus(http.StatusNoContent)
	tina.Get("/api/applications").ExpectError(http.StatusForbidden, "", "secondFactorSetupRequired")
	enableWithCodes(t, tina, "a-new-password-1")
	tina.Get("/api/applications").ExpectStatus(http.StatusOK)
}

// A setup-restricted session of another holder of the password must not turn into a full one when
// the user enrols: enrolling ends every other session of the user.
func TestEnrollingTheSecondFactorEndsTheOtherSessionsOfTheUser(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	tina := plainUserClient(t, env, "tina")
	forceAll(t, env.AdminClient(), true)
	thief := env.Client()
	thief.LoginAs("tina", "initial-pw-1").ExpectStatus(http.StatusOK)
	thief.Get("/api/applications").ExpectError(http.StatusForbidden, "", "secondFactorSetupRequired")

	enableWithCodes(t, tina, "initial-pw-1")

	tina.Get("/api/applications").ExpectStatus(http.StatusOK)
	thief.Get("/api/session").ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	thief.Get("/api/applications").ExpectError(http.StatusUnauthorized, "", "unauthenticated")
}

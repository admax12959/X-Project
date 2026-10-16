package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// An administrator resets a user's second factor (#23, ADR-0006): the way back in for a user
// who lost the authenticator and the recovery codes.

func resetSecondFactor(c *apitest.Client, userID string) *apitest.Response {
	return c.Post("/api/users/"+userID+"/second-factor/reset", nil)
}

func TestTheUserListSaysWhoHasASecondFactor(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	tinaWithCodes(t, env)
	plainUserClient(t, env, "bob")
	users := listUsers(t, env.AdminClient())
	if !findUser(t, users, "tina").SecondFactorEnabled || findUser(t, users, "bob").SecondFactorEnabled {
		t.Fatalf("users = %+v, want tina with a second factor and bob without", users)
	}
	if !findUser(t, users, apitest.AdminUsername).SecondFactorEnabled {
		t.Fatal("the administrator has one too")
	}
}

func TestAnAdministratorResetsASecondFactorAndTheUserLogsInWithThePasswordAlone(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	_, _, tina := tinaWithCodes(t, env)
	admin := env.AdminClient()
	tinaID := findUser(t, listUsers(t, admin), "tina").ID

	// A locked second step is cleared too.
	for i := 0; i < 5; i++ {
		c, _ := firstFactor(t, env)
		c.Post("/api/session/second-factor", map[string]string{"code": "000000"})
	}

	var u userBody
	resetSecondFactor(admin, tinaID).ExpectStatus(http.StatusOK).JSON(&u)
	if u.Username != "tina" || u.SecondFactorEnabled {
		t.Fatalf("answer = %+v, want tina without a second factor", u)
	}
	if st := recoveryStatusOf(t, tina); st.Enabled || len(st.Authenticators) != 0 || st.RecoveryCodesRemaining != 0 {
		t.Fatalf("status = %+v, want everything gone", st)
	}
	var s loginState
	env.Client().LoginAs("tina", "initial-pw-1").ExpectStatus(http.StatusOK).JSON(&s)
	if s.User.Username != "tina" {
		t.Fatalf("login = %+v, want a session without asking for a code", s)
	}
	// She can set it up again from scratch.
	env.Clock.Advance(totpStep)
	enableWithCodes(t, tina, "initial-pw-1")
}

func TestOnlyAnAdministratorCanResetAndOnlyWhereThereIsSomethingToReset(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	_, _, tina := tinaWithCodes(t, env)
	bob := plainUserClient(t, env, "bob")
	admin := env.AdminClient()
	users := listUsers(t, admin)
	tinaID, bobID := findUser(t, users, "tina").ID, findUser(t, users, "bob").ID

	resetSecondFactor(bob, tinaID).ExpectError(http.StatusForbidden, "", "forbidden")
	resetSecondFactor(tina, tinaID).ExpectError(http.StatusForbidden, "", "forbidden") // not even for herself
	resetSecondFactor(env.Client(), tinaID).ExpectError(http.StatusUnauthorized, "", "unauthenticated")
	if !secondFactorOf(t, tina).Enabled {
		t.Fatal("a refused reset removed the second factor")
	}

	resetSecondFactor(admin, bobID).ExpectError(http.StatusNotFound, "", "notFound")
	resetSecondFactor(admin, "999999").ExpectError(http.StatusNotFound, "", "notFound")
	if got := auditOf(t, admin, "secondFactorReset"); len(got) != 0 {
		t.Fatalf("audit = %+v, want nothing for refused or empty resets", got)
	}
}

func TestAResetUserWhoIsRequiredToHaveASecondFactorMustSetItUpAgain(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	_, _, tina := tinaWithCodes(t, env)
	admin := env.AdminClient()
	forceAll(t, admin, true)
	resetSecondFactor(admin, findUser(t, listUsers(t, admin), "tina").ID).ExpectStatus(http.StatusOK)

	tina.Get("/api/applications").ExpectError(http.StatusForbidden, "", "secondFactorSetupRequired")
	enableWithCodes(t, tina, "initial-pw-1")
	tina.Get("/api/applications").ExpectStatus(http.StatusOK)
}

func TestAnAdministratorCanResetTheirOwnAndMustSetItUpAgain(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	admin := env.AdminClient()
	adminID := findUser(t, listUsers(t, admin), apitest.AdminUsername).ID
	resetSecondFactor(admin, adminID).ExpectStatus(http.StatusOK)
	admin.Get("/api/users").ExpectError(http.StatusForbidden, "", "secondFactorSetupRequired")
}

func TestResettingIsAuditedWithTheOperatorAndMailsTheUserWithoutAnySecret(t *testing.T) {
	t.Parallel()
	env, tina := tinaOnMail(t)
	addr := apitest.Email("tina")
	secret, codes := enableWithCodes(t, tina, "initial-pw-1")
	eventually(t, "the enable notice", func() bool { return len(mailsTo(env, addr)) == 1 })
	admin := env.AdminClient()

	resetSecondFactor(admin, findUser(t, listUsers(t, admin), "tina").ID).ExpectStatus(http.StatusOK)

	got := auditOf(t, admin, "secondFactorReset")
	if len(got) != 1 || got[0].Actor.Kind != "user" || got[0].Actor.Username != apitest.AdminUsername || got[0].Target.Username != "tina" {
		t.Fatalf("audit = %+v, want the administrator resetting tina", got)
	}
	if raw := string(mustJSON(t, got[0])); strings.Contains(raw, secret) || recoveryCodeShape.FindString(raw) != "" {
		t.Errorf("audit entry leaks a secret: %s", raw)
	}
	eventually(t, "the reset notice", func() bool { return len(mailsTo(env, addr)) == 2 })
	subjects := mailsTo(env, addr)
	if subjects[0] == subjects[1] {
		t.Errorf("subjects = %q, want a reset notice different from the enable notice", subjects)
	}
	m := env.Mailer.SentTo(addr)[1].Message
	if m.Text == "" || m.HTML == "" || strings.Contains(m.Text+m.HTML, secret) || strings.Contains(m.Text, codes[0]) {
		t.Errorf("reset notice = %+v, want both parts and no secret", m)
	}
}

func TestResettingWithoutSMTPSendsNoMail(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	_, _, _ = tinaWithCodes(t, env)
	admin := env.AdminClient()
	resetSecondFactor(admin, findUser(t, listUsers(t, admin), "tina").ID).ExpectStatus(http.StatusOK)
	env.SettleBackground()
	if got := env.Mailer.Sent(); len(got) != 0 {
		t.Fatalf("mails = %d, want none", len(got))
	}
}

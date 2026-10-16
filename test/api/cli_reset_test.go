package api

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
)

// The command line in the container (#23, ADR-0006): the way back in when the only
// administrator is locked out. It acts as "the command line", not as a user.

var temporaryPasswordLine = regexp.MustCompile(`(?m)^Temporary password for \S+: (\S+)$`)

func TestTheCommandLineResetsTheSoleAdministratorsSecondFactorAndPassword(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	admin := env.AdminClient() // has a second factor; this session is about to be ended
	env.Clock.Advance(totpStep)

	res := env.CLI("reset-user", "--user", apitest.AdminUsername, "--second-factor", "--password")
	if res.Exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", res.Exit, res.Stderr)
	}
	m := temporaryPasswordLine.FindStringSubmatch(res.Stdout)
	if m == nil {
		t.Fatalf("stdout = %q, want a \"Temporary password for admin: ...\" line", res.Stdout)
	}
	temp := m[1]

	admin.Get("/api/session").ExpectError(http.StatusUnauthorized, "", "unauthenticated") // sessions ended
	env.Client().LoginEmail(apitest.AdminEmail, apitest.AdminPassword).ExpectError(http.StatusUnauthorized, "", "invalidCredentials")

	// The second factor is gone, so the temporary password alone logs in; it has to be changed,
	// and an administrator then has to set a second factor up again.
	c := env.Client()
	var s struct {
		MustChangePassword        bool `json:"mustChangePassword"`
		SecondFactorSetupRequired bool `json:"secondFactorSetupRequired"`
	}
	c.LoginEmail(apitest.AdminEmail, temp).ExpectStatus(http.StatusOK).JSON(&s)
	if !s.MustChangePassword || !s.SecondFactorSetupRequired {
		t.Fatalf("login = %+v, want a password change and a second-factor setup due", s)
	}
	c.Get("/api/users").ExpectError(http.StatusForbidden, "", "passwordChangeRequired")
	changePassword(c, temp, "a-new-admin-password").ExpectStatus(http.StatusNoContent)
	c.Get("/api/users").ExpectError(http.StatusForbidden, "", "secondFactorSetupRequired")
	enableWithCodes(t, c, "a-new-admin-password")
	c.Get("/api/users").ExpectStatus(http.StatusOK)
}

func TestTheCommandLineResetIsAuditedWithTheCommandLineAsOperatorAndNeverTheSecrets(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	secret, _, _ := tinaWithCodes(t, env)
	admin := env.AdminClient()

	res := env.CLI("reset-user", "--user", "tina", "--second-factor", "--password")
	if res.Exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", res.Exit, res.Stderr)
	}
	temp := temporaryPasswordLine.FindStringSubmatch(res.Stdout)[1]

	for _, action := range []string{"secondFactorReset", "passwordReset"} {
		got := auditOf(t, admin, action)
		if len(got) != 1 || got[0].Actor.Kind != "cli" || got[0].Actor.Username != "" || got[0].Target.Username != "tina" {
			t.Fatalf("%s entries = %+v, want one by the command line on tina", action, got)
		}
		raw := string(mustJSON(t, got[0]))
		if strings.Contains(raw, temp) || strings.Contains(raw, secret) {
			t.Errorf("%s entry leaks a secret: %s", action, raw)
		}
	}
	// The filter for the operator finds them.
	if p := auditLog(t, admin, "actor", "cli"); p.Total != 2 {
		t.Errorf("entries of the command line = %d, want 2", p.Total)
	}
}

func TestTheCommandLineDoesOnlyWhatItIsAskedTo(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	frozenMidStep(env)
	_, _, tina := tinaWithCodes(t, env)
	admin := env.AdminClient()

	if res := env.CLI("reset-user", "--user", "tina", "--second-factor"); res.Exit != 0 || strings.Contains(res.Stdout, "Temporary password") {
		t.Fatalf("exit = %d, stdout = %q, want a reset without a new password", res.Exit, res.Stdout)
	}
	if secondFactorOf(t, tina).Enabled {
		t.Fatal("the second factor is still on")
	}
	tina.Get("/api/applications").ExpectStatus(http.StatusOK) // the password and the session are untouched
	env.Client().LoginAs("tina", "initial-pw-1").ExpectStatus(http.StatusOK)
	if got := auditOf(t, admin, "passwordReset"); len(got) != 0 {
		t.Fatalf("passwordReset entries = %+v, want none", got)
	}

	// Nothing to reset: said so, nothing audited, success.
	res := env.CLI("reset-user", "--user", "tina", "--second-factor")
	if res.Exit != 0 || !strings.Contains(res.Stdout, "no second factor") {
		t.Fatalf("exit = %d, stdout = %q, want it to say there was none", res.Exit, res.Stdout)
	}
	if got := auditOf(t, admin, "secondFactorReset"); len(got) != 1 {
		t.Fatalf("secondFactorReset entries = %d, want 1", len(got))
	}
}

func TestTheCommandLineRefusesWhatItCannotDo(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectStatus(http.StatusOK)

	for name, args := range map[string][]string{
		"no user":               {"reset-user", "--second-factor"},
		"nothing to do":         {"reset-user", "--user", "admin"},
		"unknown user":          {"reset-user", "--user", "nobody", "--password"},
		"a directory-only user": {"reset-user", "--user", "elarsen", "--password"},
		"unknown command":       {"frobnicate"},
		"no command":            {},
	} {
		res := env.CLI(args...)
		if res.Exit == 0 || res.Stderr == "" || strings.Contains(res.Stdout, "Temporary password") {
			t.Errorf("%s: exit = %d, stdout = %q, stderr = %q, want a refusal", name, res.Exit, res.Stdout, res.Stderr)
		}
	}
	env.Client().LoginEmail(apitest.AdminEmail, apitest.AdminPassword).ExpectStatus(http.StatusOK) // untouched
	if got := auditOf(t, env.AdminClient(), "passwordReset"); len(got) != 0 {
		t.Fatalf("passwordReset entries = %+v, want none", got)
	}
}

func TestTheCommandLineMailsTheUserWithoutAnySecret(t *testing.T) {
	t.Parallel()
	env, tina := tinaOnMail(t)
	addr := apitest.Email("tina")
	enableWithCodes(t, tina, "initial-pw-1")
	eventually(t, "the enable notice", func() bool { return len(mailsTo(env, addr)) == 1 })

	res := env.CLI("reset-user", "--user", addr, "--second-factor", "--password") // by login email too
	if res.Exit != 0 {
		t.Fatalf("exit = %d, stderr = %q", res.Exit, res.Stderr)
	}
	temp := temporaryPasswordLine.FindStringSubmatch(res.Stdout)[1]
	eventually(t, "both notices", func() bool { return len(mailsTo(env, addr)) == 3 })
	for _, m := range env.Mailer.SentTo(addr) {
		if strings.Contains(m.Message.Text+m.Message.HTML, temp) {
			t.Errorf("mail %q carries the password", m.Message.Subject)
		}
	}
}

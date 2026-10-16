package api

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/app"
)

// smtpConfigView mirrors GET /api/system-config/smtp. There is deliberately no password field,
// and no field to skip certificate verification.
type smtpConfigView struct {
	Enabled           bool   `json:"enabled"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Security          string `json:"security"`
	AllowPlaintext    bool   `json:"allowPlaintext"`
	Username          string `json:"username"`
	PasswordSet       bool   `json:"passwordSet"`
	From              string `json:"from"`
	CACert            string `json:"caCert"`
	SiteURL           string `json:"siteUrl"`
	SecretsUnreadable bool   `json:"secretsUnreadable"`
	Configured        bool   `json:"configured"`
}

const smtpConfigPath = "/api/system-config/smtp"

func getSMTPConfig(t *testing.T, c *apitest.Client) (smtpConfigView, string) {
	t.Helper()
	r := c.Get(smtpConfigPath).ExpectStatus(http.StatusOK)
	var v smtpConfigView
	r.JSON(&v)
	return v, string(r.Body)
}

// smtpForm is a complete, valid SMTP form (STARTTLS on 587 with an account); over replaces fields.
func smtpForm(over map[string]any) map[string]any {
	f := map[string]any{
		"enabled": true, "host": "smtp.corp.example", "port": 587, "security": "starttls",
		"username": "mailer", "password": "smtp-secret-1", "from": "X-Project <noreply@corp.example>",
		"siteUrl": "https://xp.corp.example",
	}
	for k, v := range over {
		f[k] = v
	}
	return f
}

func mailConfigured(t *testing.T, c *apitest.Client) bool {
	t.Helper()
	var body struct {
		Configured bool `json:"configured"`
	}
	c.Get("/api/mail/status").ExpectStatus(http.StatusOK).JSON(&body)
	return body.Configured
}

func TestMailIsNotConfiguredUntilAnAdministratorSavesAnEnabledConfiguration(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	v, _ := getSMTPConfig(t, admin)
	if v.Configured || v.Enabled || v.Host != "" || v.Port != 587 || v.Security != "starttls" || v.PasswordSet {
		t.Fatalf("default config = %+v, want unconfigured STARTTLS on 587", v)
	}
	if mailConfigured(t, admin) {
		t.Fatal("mail reported as configured before anything was saved")
	}

	bob := plainUserClient(t, env, "bob") // created first: once mail is configured, new users are invited instead
	admin.Put(smtpConfigPath, smtpForm(nil)).ExpectStatus(http.StatusOK)
	if !mailConfigured(t, admin) {
		t.Fatal("mail not reported as configured after saving an enabled configuration")
	}
	// Any logged-in user may ask (other features decide from it whether they are available).
	if !mailConfigured(t, bob) {
		t.Fatal("a plain user does not see mail as configured")
	}

	admin.Put(smtpConfigPath, smtpForm(map[string]any{"enabled": false})).ExpectStatus(http.StatusOK)
	if mailConfigured(t, admin) {
		t.Fatal("switched-off mail reported as configured")
	}
	env.Client().Get("/api/mail/status").ExpectStatus(http.StatusUnauthorized)
}

func TestSavedSMTPConfigurationIsReadBackWithoutThePasswordAndSurvivesARestart(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	ca := strings.TrimSpace(testCAPEM(t))

	admin.Put(smtpConfigPath, smtpForm(map[string]any{"caCert": ca + "\n"})).ExpectStatus(http.StatusOK)
	env.Restart()

	v, body := getSMTPConfig(t, env.AdminClient())
	if !v.Configured || !v.Enabled || v.Host != "smtp.corp.example" || v.Port != 587 || v.Security != "starttls" ||
		v.Username != "mailer" || !v.PasswordSet || v.From != "X-Project <noreply@corp.example>" ||
		v.CACert != ca || v.SiteURL != "https://xp.corp.example" {
		t.Fatalf("config = %+v", v)
	}
	if strings.Contains(body, "smtp-secret-1") || strings.Contains(strings.ToLower(body), `"password"`) {
		t.Fatalf("the response carries the password: %s", body)
	}
	for _, skip := range []string{"skipverify", "insecure", "verify"} {
		if strings.Contains(strings.ToLower(body), skip) {
			t.Fatalf("the configuration has a switch to skip certificate verification (%q): %s", skip, body)
		}
	}
}

func TestLeavingThePasswordEmptyKeepsTheStoredSMTPPasswordAndRemovingTheAccountForgetsIt(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(smtpConfigPath, smtpForm(nil)).ExpectStatus(http.StatusOK)

	admin.Put(smtpConfigPath, smtpForm(map[string]any{"password": "", "host": "smtp2.corp.example"})).ExpectStatus(http.StatusOK)
	if v, _ := getSMTPConfig(t, admin); !v.PasswordSet || v.Host != "smtp2.corp.example" {
		t.Fatalf("config = %+v, want the password kept", v)
	}

	// An account-less (anonymous relay) configuration has no password; adding an account back needs one.
	admin.Put(smtpConfigPath, smtpForm(map[string]any{"username": "", "password": ""})).ExpectStatus(http.StatusOK)
	if v, _ := getSMTPConfig(t, admin); v.PasswordSet || v.Username != "" {
		t.Fatalf("config = %+v, want no account and no password", v)
	}
	admin.Put(smtpConfigPath, smtpForm(map[string]any{"password": ""})).ExpectError(http.StatusUnprocessableEntity, "password", "required")
}

func TestSMTPFormIsValidated(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	cases := []struct {
		name        string
		over        map[string]any
		field, code string
	}{
		{"host required", map[string]any{"host": ""}, "host", "required"},
		{"host is a name, not a URL", map[string]any{"host": "smtp://smtp.corp.example"}, "host", "invalid"},
		{"port negative", map[string]any{"port": -1}, "port", "invalid"},
		{"port too large", map[string]any{"port": 70000}, "port", "invalid"},
		{"unknown security", map[string]any{"security": "ssl3"}, "security", "invalid"},
		{"plaintext must be confirmed", map[string]any{"security": "none"}, "allowPlaintext", "required"},
		{"sender required", map[string]any{"from": ""}, "from", "required"},
		{"sender must be an address", map[string]any{"from": "not an address"}, "from", "invalid"},
		{"password needs an account", map[string]any{"username": ""}, "username", "required"},
		{"account needs a password", map[string]any{"password": ""}, "password", "required"},
		{"CA must be PEM", map[string]any{"caCert": "-----BEGIN CERTIFICATE-----\nnope\n-----END CERTIFICATE-----"}, "caCert", "invalid"},
		{"CA text without certificate", map[string]any{"caCert": "hello"}, "caCert", "invalid"},
		{"site address required", map[string]any{"siteUrl": ""}, "siteUrl", "required"},
		{"site address must be http(s)", map[string]any{"siteUrl": "ftp://xp.corp.example"}, "siteUrl", "invalid"},
		{"site address without a host", map[string]any{"siteUrl": "https://"}, "siteUrl", "invalid"},
		{"site address below a path", map[string]any{"siteUrl": "https://xp.corp.example/xproject"}, "siteUrl", "invalid"},
		{"site address without a query", map[string]any{"siteUrl": "https://xp.corp.example/?a=b"}, "siteUrl", "invalid"},
		{"site address without credentials", map[string]any{"siteUrl": "https://u:p@xp.corp.example"}, "siteUrl", "invalid"},
		{"field too long", map[string]any{"username": strings.Repeat("a", 1001)}, "username", "tooLong"},
	}
	for _, tc := range cases {
		admin.Put(smtpConfigPath, smtpForm(tc.over)).ExpectError(http.StatusUnprocessableEntity, tc.field, tc.code)
	}
	if v, _ := getSMTPConfig(t, admin); v.Configured {
		t.Fatalf("a refused save changed the configuration: %+v", v)
	}
	// A switched-off configuration may be incomplete.
	admin.Put(smtpConfigPath, map[string]any{"enabled": false}).ExpectStatus(http.StatusOK)
	// Unencrypted mail is possible, but only with the explicit confirmation.
	admin.Put(smtpConfigPath, smtpForm(map[string]any{"security": "none", "allowPlaintext": true, "port": 25})).ExpectStatus(http.StatusOK)
	if v, _ := getSMTPConfig(t, admin); v.Security != "none" || !v.AllowPlaintext || v.Port != 25 {
		t.Fatalf("config = %+v", v)
	}
	// The confirmation means nothing for an encrypted connection and is not kept.
	admin.Put(smtpConfigPath, smtpForm(map[string]any{"security": "tls", "port": 465, "allowPlaintext": true})).ExpectStatus(http.StatusOK)
	if v, _ := getSMTPConfig(t, admin); v.Security != "tls" || v.AllowPlaintext {
		t.Fatalf("config = %+v, want the plaintext confirmation dropped", v)
	}
}

func TestSiteAddressIsStoredWithoutATrailingSlash(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	admin.Put(smtpConfigPath, smtpForm(map[string]any{"siteUrl": " https://corp.example/ "})).ExpectStatus(http.StatusOK)

	if v, _ := getSMTPConfig(t, admin); v.SiteURL != "https://corp.example" {
		t.Fatalf("siteUrl = %q", v.SiteURL)
	}
}

func TestOnlyAdministratorsMayReadOrChangeTheSMTPConfiguration(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	bob := plainUserClient(t, env, "bob")

	bob.Get(smtpConfigPath).ExpectError(http.StatusForbidden, "", "forbidden")
	bob.Put(smtpConfigPath, smtpForm(nil)).ExpectError(http.StatusForbidden, "", "forbidden")

	if v, _ := getSMTPConfig(t, env.AdminClient()); v.Configured {
		t.Fatalf("a plain user changed the configuration: %+v", v)
	}
}

func TestSavingSMTPConfigurationIsAuditedWithFieldNamesAndNeverValues(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	admin.Put(smtpConfigPath, smtpForm(map[string]any{"password": "audit-me-not-1"})).ExpectStatus(http.StatusOK)

	entries := auditOf(t, admin, "configChanged")
	if len(entries) != 1 {
		t.Fatalf("config audit entries = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Actor.Username != apitest.AdminUsername || e.Target.Type != "systemConfig" || e.Target.ID != "smtp" || e.Params["section"] != "smtp" {
		t.Fatalf("entry = %+v", e)
	}
	var names []string
	for _, n := range e.Params["changes"].([]any) {
		names = append(names, n.(string))
	}
	for _, want := range []string{"enabled", "host", "from", "username", "password", "siteUrl"} {
		if !slices.Contains(names, want) {
			t.Errorf("changes = %v, missing %s", names, want)
		}
	}
	raw := string(admin.Get(auditPath("action", "configChanged")).Body)
	for _, secret := range []string{"audit-me-not-1", "smtp.corp.example", "noreply@corp.example", "xp.corp.example"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("the audit log carries the value %q: %s", secret, raw)
		}
	}

	// Saving the same form again (password left empty) changes nothing and writes nothing.
	admin.Put(smtpConfigPath, smtpForm(map[string]any{"password": ""})).ExpectStatus(http.StatusOK)
	if n := len(auditOf(t, admin, "configChanged")); n != 1 {
		t.Fatalf("config audit entries = %d, want 1", n)
	}
}

func TestAMasterKeyThatChangedLeavesTheSMTPPasswordUnreadableUntilItIsEnteredAgain(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.AdminClient().Put(smtpConfigPath, smtpForm(nil)).ExpectStatus(http.StatusOK)

	env.Restart(func(c *app.Config) { c.SecretKey = "a-different-master-key" })

	admin := env.AdminClient()
	v, _ := getSMTPConfig(t, admin)
	if !v.SecretsUnreadable || !v.PasswordSet {
		t.Fatalf("config = %+v, want the password reported as set but unreadable", v)
	}
	if v.Configured {
		t.Fatalf("config = %+v, want mail reported unavailable while the stored password cannot be read", v)
	}
	admin.Put(smtpConfigPath, smtpForm(map[string]any{"password": ""})).ExpectError(http.StatusUnprocessableEntity, "password", "required")
	admin.Put(smtpConfigPath, smtpForm(map[string]any{"password": "smtp-secret-1"})).ExpectStatus(http.StatusOK)
	if v, _ := getSMTPConfig(t, admin); v.SecretsUnreadable || !v.Configured {
		t.Fatalf("config = %+v, want the secret readable again and mail available", v)
	}
}

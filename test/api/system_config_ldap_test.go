package api

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/app"
)

// ldapConfigView mirrors GET /api/system-config/ldap. There is deliberately no password field.
type ldapConfigView struct {
	Enabled           bool   `json:"enabled"`
	URL               string `json:"url"`
	StartTLS          bool   `json:"startTls"`
	SkipVerify        bool   `json:"skipVerify"`
	BindDN            string `json:"bindDn"`
	BindPasswordSet   bool   `json:"bindPasswordSet"`
	BaseDN            string `json:"baseDn"`
	UserFilter        string `json:"userFilter"`
	LoginAttr         string `json:"loginAttr"`
	NameAttr          string `json:"nameAttr"`
	EmailAttr         string `json:"emailAttr"`
	DepartmentAttr    string `json:"departmentAttr"`
	DisabledFilter    string `json:"disabledFilter"`
	TimeoutSeconds    int    `json:"timeoutSeconds"`
	SecretsUnreadable bool   `json:"secretsUnreadable"`
	Configured        bool   `json:"configured"`
}

const ldapConfigPath = "/api/system-config/ldap"

func getLDAPConfig(t *testing.T, c *apitest.Client) (ldapConfigView, string) {
	t.Helper()
	r := c.Get(ldapConfigPath).ExpectStatus(http.StatusOK)
	var v ldapConfigView
	r.JSON(&v)
	return v, string(r.Body)
}

// ldapForm is a complete, valid LDAP form; over replaces fields.
func ldapForm(over map[string]any) map[string]any {
	f := map[string]any{
		"enabled": true, "url": "ldap://ldap.corp.example:389", "baseDn": "ou=people,dc=corp,dc=example",
		"bindDn": "cn=svc,dc=corp,dc=example", "bindPassword": "svc-secret-1",
	}
	for k, v := range over {
		f[k] = v
	}
	return f
}

func loginMethods(t *testing.T, env *apitest.Env) []string {
	t.Helper()
	var body struct {
		Items []string `json:"items"`
	}
	env.Client().Get("/api/login-methods").ExpectStatus(http.StatusOK).JSON(&body)
	return body.Items
}

func TestAdminSeesTheLDAPConfigurationImportedFromTheConfigFile(t *testing.T) {
	t.Parallel()
	env := apitest.New(t, func(c *app.Config) {
		c.LDAP.URL, c.LDAP.BaseDN = "ldap://dir.corp.example:389", "ou=staff,dc=corp,dc=example"
		c.LDAP.StartTLS, c.LDAP.DepartmentAttr = true, "departmentNumber"
	})

	v, _ := getLDAPConfig(t, env.AdminClient())

	if !v.Enabled || !v.Configured || v.URL != "ldap://dir.corp.example:389" || v.BaseDN != "ou=staff,dc=corp,dc=example" ||
		!v.StartTLS || v.DepartmentAttr != "departmentNumber" {
		t.Fatalf("imported config = %+v", v)
	}
	// Defaults of the fields the file left empty.
	if v.UserFilter != "(uid=%s)" || v.LoginAttr != "uid" || v.NameAttr != "displayName" || v.EmailAttr != "mail" || v.TimeoutSeconds != 10 {
		t.Fatalf("defaults = %+v", v)
	}
	if v.BindPasswordSet {
		t.Fatal("no bind password was configured, but one is reported as set")
	}
}

func TestWithoutLDAPInTheFileThereIsNothingToImportAndLDAPIsOff(t *testing.T) {
	t.Parallel()
	env := apitest.New(t, func(c *app.Config) { c.LDAP.URL, c.LDAP.BaseDN = "", "" })

	v, _ := getLDAPConfig(t, env.AdminClient())
	if v.Enabled || v.Configured || v.URL != "" || v.UserFilter != "(uid=%s)" {
		t.Fatalf("config = %+v, want the defaults, switched off and never saved", v)
	}
	if got := loginMethods(t, env); !slices.Equal(got, []string{"email"}) {
		t.Fatalf("login methods = %v, want [email]", got)
	}
}

func TestPartialLDAPSettingsInTheConfigFileStopStartup(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*app.Config){
		"base without url":    func(c *app.Config) { c.LDAP.URL, c.LDAP.BaseDN = "", "dc=corp" },
		"bind dn without url": func(c *app.Config) { c.LDAP.URL, c.LDAP.BaseDN, c.LDAP.BindDN = "", "", "cn=svc" },
		"url without base":    func(c *app.Config) { c.LDAP.BaseDN = "" },
		"bind dn without pw":  func(c *app.Config) { c.LDAP.BindDN = "cn=svc" },
		"filter without %s":   func(c *app.Config) { c.LDAP.UserFilter = "(uid=x)" },
		"filter with two %s":  func(c *app.Config) { c.LDAP.UserFilter = "(|(uid=%s)(cn=%s))" },
		"not an ldap url":     func(c *app.Config) { c.LDAP.URL = "http://ldap.example" },
	}
	for name, mutate := range cases {
		if err := apitest.StartError(t, mutate); err == nil {
			t.Errorf("%s: startup succeeded, want an error", name)
		}
	}
}

func TestTheDatabaseRulesAfterTheImportAndTheFileIsNeverReadAgain(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(ldapConfigPath, ldapForm(map[string]any{"url": "ldap://edited.corp.example:389"})).ExpectStatus(http.StatusOK)

	env.Restart(func(c *app.Config) { c.LDAP.URL, c.LDAP.BaseDN = "ldap://changed-in-file:389", "dc=other" })

	v, _ := getLDAPConfig(t, env.AdminClient())
	if v.URL != "ldap://edited.corp.example:389" || v.BaseDN != "ou=people,dc=corp,dc=example" {
		t.Fatalf("config after restart = %+v, want the one saved in the database", v)
	}
}

func TestLDAPInTheFileIsImportedOnTheFirstStartThatHasIt(t *testing.T) {
	t.Parallel()
	env := apitest.New(t, func(c *app.Config) { c.LDAP.URL, c.LDAP.BaseDN = "", "" })
	if v, _ := getLDAPConfig(t, env.AdminClient()); v.Configured {
		t.Fatalf("config = %+v, want nothing stored yet", v)
	}

	env.Restart(func(c *app.Config) { c.LDAP.URL, c.LDAP.BaseDN = "ldap://late.corp.example:389", "dc=corp" })
	if v, _ := getLDAPConfig(t, env.AdminClient()); !v.Enabled || v.URL != "ldap://late.corp.example:389" {
		t.Fatalf("config = %+v, want the file's LDAP imported", v)
	}

	env.Restart(func(c *app.Config) { c.LDAP.URL = "ldap://later-still:389" })
	if v, _ := getLDAPConfig(t, env.AdminClient()); v.URL != "ldap://late.corp.example:389" {
		t.Fatalf("config = %+v, want the import to happen only once", v)
	}
}

func TestBindPasswordIsEncryptedAtRestUsedByTheDirectoryAndNeverReadBack(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	admin := env.AdminClient()

	admin.Put(ldapConfigPath, ldapForm(nil)).ExpectStatus(http.StatusOK)

	// Write-only: the response and a later read say "set", never the value.
	v, body := getLDAPConfig(t, admin)
	if !v.BindPasswordSet || v.BindDN != "cn=svc,dc=corp,dc=example" {
		t.Fatalf("config = %+v", v)
	}
	if strings.Contains(body, "svc-secret-1") || strings.Contains(strings.ToLower(body), `"bindpassword"`) {
		t.Fatalf("the response carries the password: %s", body)
	}
	// Round trip through encryption: the directory is built with the plaintext that was saved
	// (and survives a restart, i.e. it really came out of the database).
	env.Restart()
	userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
	cfg, ok := env.Directories.LastConfig()
	if !ok || cfg.BindPassword != "svc-secret-1" || cfg.BindDN != "cn=svc,dc=corp,dc=example" || cfg.URL != "ldap://ldap.corp.example:389" {
		t.Fatalf("directory built with %+v, want the saved configuration with the decrypted password", cfg)
	}
}

func TestLeavingThePasswordEmptyKeepsTheStoredOneAndANewOneReplacesIt(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	admin := env.AdminClient()
	admin.Put(ldapConfigPath, ldapForm(nil)).ExpectStatus(http.StatusOK)

	admin.Put(ldapConfigPath, ldapForm(map[string]any{"bindPassword": "", "userFilter": "(sAMAccountName=%s)"})).ExpectStatus(http.StatusOK)
	userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
	if cfg, _ := env.Directories.LastConfig(); cfg.BindPassword != "svc-secret-1" || cfg.UserFilter != "(sAMAccountName=%s)" {
		t.Fatalf("directory built with %+v, want the old password and the new filter", cfg)
	}

	admin.Put(ldapConfigPath, ldapForm(map[string]any{"bindPassword": "svc-secret-2"})).ExpectStatus(http.StatusOK)
	userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
	if cfg, _ := env.Directories.LastConfig(); cfg.BindPassword != "svc-secret-2" {
		t.Fatalf("directory built with %+v, want the new password", cfg)
	}
}

func TestRemovingTheServiceAccountForgetsItsPassword(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(ldapConfigPath, ldapForm(nil)).ExpectStatus(http.StatusOK)

	admin.Put(ldapConfigPath, ldapForm(map[string]any{"bindDn": "", "bindPassword": ""})).ExpectStatus(http.StatusOK)
	if v, _ := getLDAPConfig(t, admin); v.BindPasswordSet || v.BindDN != "" {
		t.Fatalf("config = %+v, want an anonymous search without a stored password", v)
	}
	// Adding an account back needs a password again.
	admin.Put(ldapConfigPath, ldapForm(map[string]any{"bindPassword": ""})).ExpectError(http.StatusUnprocessableEntity, "bindPassword", "required")
}

func TestSavedLDAPConfigurationIsInEffectAtOnceWithoutARestart(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	admin := env.AdminClient()
	if got := loginMethods(t, env); !slices.Equal(got, []string{"email", "ldap"}) {
		t.Fatalf("login methods = %v", got)
	}

	admin.Put(ldapConfigPath, ldapForm(map[string]any{"enabled": false})).ExpectStatus(http.StatusOK)
	if got := loginMethods(t, env); !slices.Equal(got, []string{"email"}) {
		t.Fatalf("login methods after switching LDAP off = %v, want [email]", got)
	}
	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectError(http.StatusUnprocessableEntity, "method", "invalid")
	env.Client().LoginEmail(apitest.AdminEmail, apitest.AdminPassword).ExpectStatus(http.StatusOK)

	admin.Put(ldapConfigPath, ldapForm(nil)).ExpectStatus(http.StatusOK)
	userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
}

func TestLDAPFormIsValidatedLikeTheAdapterNeedsIt(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	cases := []struct {
		name        string
		over        map[string]any
		field, code string
	}{
		{"url required when enabled", map[string]any{"url": ""}, "url", "required"},
		{"url must be ldap", map[string]any{"url": "https://ldap.example"}, "url", "invalid"},
		{"url needs a host", map[string]any{"url": "ldap://"}, "url", "invalid"},
		{"base dn required when enabled", map[string]any{"baseDn": ""}, "baseDn", "required"},
		{"filter needs one %s", map[string]any{"userFilter": "(uid=jdoe)"}, "userFilter", "invalid"},
		{"filter with two %s", map[string]any{"userFilter": "(|(uid=%s)(cn=%s))"}, "userFilter", "invalid"},
		{"bind dn needs a password", map[string]any{"bindPassword": ""}, "bindPassword", "required"},
		{"password needs a bind dn", map[string]any{"bindDn": ""}, "bindDn", "required"},
		{"timeout out of range", map[string]any{"timeoutSeconds": 500}, "timeoutSeconds", "invalid"},
		{"field too long", map[string]any{"baseDn": strings.Repeat("a", 1001)}, "baseDn", "tooLong"},
	}
	for _, tc := range cases {
		// The first save has no stored password, so tests about the password start from nothing.
		admin.Put(ldapConfigPath, ldapForm(tc.over)).ExpectError(http.StatusUnprocessableEntity, tc.field, tc.code)
	}
	if v, _ := getLDAPConfig(t, admin); v.URL != "ldap://directory.test:389" {
		t.Fatalf("a refused save changed the configuration: %+v", v)
	}
	// A switched-off configuration may be incomplete.
	admin.Put(ldapConfigPath, map[string]any{"enabled": false}).ExpectStatus(http.StatusOK)
}

func TestTestConnectionUsesTheUnsavedFormAndSavesNothing(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(ldapConfigPath, ldapForm(nil)).ExpectStatus(http.StatusOK)
	before := len(env.Directories.Configs())

	var res struct {
		OK   bool   `json:"ok"`
		Code string `json:"code"`
	}
	admin.Post(ldapConfigPath+"/test", ldapForm(map[string]any{"url": "ldaps://unsaved.corp.example:636", "bindPassword": "typed-not-saved"})).
		ExpectStatus(http.StatusOK).JSON(&res)
	if !res.OK {
		t.Fatalf("test = %+v, want ok", res)
	}
	configs := env.Directories.Configs()
	if len(configs) != before+1 {
		t.Fatalf("directories built = %d, want exactly one more for the test", len(configs)-before)
	}
	if got := configs[len(configs)-1]; got.URL != "ldaps://unsaved.corp.example:636" || got.BindPassword != "typed-not-saved" {
		t.Fatalf("the test connected with %+v, want the unsaved form values", got)
	}
	if v, _ := getLDAPConfig(t, admin); v.URL != "ldap://ldap.corp.example:389" {
		t.Fatalf("testing changed the saved configuration: %+v", v)
	}
	if n := len(auditOf(t, admin, "configChanged")); n != 1 {
		t.Fatalf("config audit entries = %d, want only the one from the save", n)
	}
}

func TestTestConnectionFallsBackToTheStoredPasswordWhenTheFormLeavesItEmpty(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(ldapConfigPath, ldapForm(nil)).ExpectStatus(http.StatusOK)

	admin.Post(ldapConfigPath+"/test", ldapForm(map[string]any{"bindPassword": "", "baseDn": "dc=other"})).ExpectStatus(http.StatusOK)

	if cfg, _ := env.Directories.LastConfig(); cfg.BindPassword != "svc-secret-1" || cfg.BaseDN != "dc=other" {
		t.Fatalf("the test connected with %+v, want the stored password and the typed base DN", cfg)
	}
}

func TestTestConnectionReportsAnUnreachableDirectoryAndABadForm(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	env.Directory.SetUnreachable(true)

	var res struct {
		OK   bool   `json:"ok"`
		Code string `json:"code"`
	}
	admin.Post(ldapConfigPath+"/test", ldapForm(nil)).ExpectStatus(http.StatusOK).JSON(&res)
	if res.OK || res.Code != "ldapUnreachable" {
		t.Fatalf("test = %+v, want a failure with code ldapUnreachable", res)
	}

	admin.Post(ldapConfigPath+"/test", ldapForm(map[string]any{"userFilter": "nope"})).ExpectError(http.StatusUnprocessableEntity, "userFilter", "invalid")
}

func TestOnlyAdministratorsMayReadOrChangeTheSystemConfiguration(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	bob := plainUserClient(t, env, "bob")

	bob.Get(ldapConfigPath).ExpectError(http.StatusForbidden, "", "forbidden")
	bob.Put(ldapConfigPath, ldapForm(nil)).ExpectError(http.StatusForbidden, "", "forbidden")
	bob.Post(ldapConfigPath+"/test", ldapForm(nil)).ExpectError(http.StatusForbidden, "", "forbidden")

	if v, _ := getLDAPConfig(t, env.AdminClient()); v.URL != "ldap://directory.test:389" {
		t.Fatalf("a plain user changed the configuration: %+v", v)
	}
	if n := len(auditOf(t, env.AdminClient(), "configChanged")); n != 0 {
		t.Fatalf("config audit entries = %d, want none", n)
	}
}

func TestSavingLDAPConfigurationIsAuditedWithFieldNamesAndNeverValues(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	admin.Put(ldapConfigPath, ldapForm(map[string]any{"bindPassword": "audit-me-not-1"})).ExpectStatus(http.StatusOK)

	entries := auditOf(t, admin, "configChanged")
	if len(entries) != 1 {
		t.Fatalf("config audit entries = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Actor.Kind != "user" || e.Actor.Username != apitest.AdminUsername || e.At == "" {
		t.Fatalf("operator/time = %+v", e)
	}
	if e.Target.Type != "systemConfig" || e.Target.ID != "ldap" || e.Params["section"] != "ldap" {
		t.Fatalf("target/params = %+v / %+v", e.Target, e.Params)
	}
	var names []string
	for _, n := range e.Params["changes"].([]any) {
		names = append(names, n.(string))
	}
	// The file had url + baseDn; the form changed url, baseDn, bindDn and the password.
	for _, want := range []string{"url", "baseDn", "bindDn", "bindPassword"} {
		if !slices.Contains(names, want) {
			t.Errorf("changes = %v, missing %s", names, want)
		}
	}
	raw := admin.Get(auditPath("action", "configChanged")).Body
	for _, secret := range []string{"audit-me-not-1", "ldap.corp.example", "cn=svc"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("the audit log carries the value %q: %s", secret, raw)
		}
	}
	// Filterable by the configuration section.
	if p := auditLog(t, admin, "targetType", "systemConfig", "targetId", "ldap"); p.Total != 1 {
		t.Fatalf("entries for the ldap section = %d, want 1", p.Total)
	}
}

func TestSavingWithoutChangesWritesNoAuditEntry(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(ldapConfigPath, ldapForm(nil)).ExpectStatus(http.StatusOK)

	// Same form again, password left empty: nothing changed.
	admin.Put(ldapConfigPath, ldapForm(map[string]any{"bindPassword": ""})).ExpectStatus(http.StatusOK)

	if n := len(auditOf(t, admin, "configChanged")); n != 1 {
		t.Fatalf("config audit entries = %d, want 1 (only the first save)", n)
	}
}

func TestAMasterKeyThatChangedLeavesSecretsUnreadableUntilTheyAreEnteredAgain(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	addEmma(env)
	env.AdminClient().Put(ldapConfigPath, ldapForm(nil)).ExpectStatus(http.StatusOK)

	env.Restart(func(c *app.Config) { c.SecretKey = "a-different-master-key" })

	// No decrypted garbage and no 500: the directory counts as unreachable, and the page says why.
	loginLDAP(env.Client(), "elarsen", "ldap-pw").ExpectError(http.StatusServiceUnavailable, "", "ldapUnreachable")
	admin := env.AdminClient()
	v, _ := getLDAPConfig(t, admin)
	if !v.SecretsUnreadable || !v.BindPasswordSet {
		t.Fatalf("config = %+v, want the password reported as set but unreadable", v)
	}
	// Saving without re-entering the password cannot work; entering it again fixes it.
	admin.Put(ldapConfigPath, ldapForm(map[string]any{"bindPassword": ""})).ExpectError(http.StatusUnprocessableEntity, "bindPassword", "required")
	admin.Put(ldapConfigPath, ldapForm(map[string]any{"bindPassword": "svc-secret-1"})).ExpectStatus(http.StatusOK)
	if v, _ := getLDAPConfig(t, admin); v.SecretsUnreadable {
		t.Fatalf("config = %+v, want the secret readable again", v)
	}
	userOf(t, loginLDAP(env.Client(), "elarsen", "ldap-pw"))
}

func TestASecretSealedWithAnotherKeyVersionIsUnreadable(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	env.AdminClient().Put(ldapConfigPath, ldapForm(nil)).ExpectStatus(http.StatusOK)

	env.Restart(func(c *app.Config) { c.SecretKeyVersion = 2 })
	if v, _ := getLDAPConfig(t, env.AdminClient()); !v.SecretsUnreadable {
		t.Fatalf("config = %+v, want the version-1 secret reported as unreadable under key version 2", v)
	}

	env.Restart(func(c *app.Config) { c.SecretKeyVersion = 1 })
	if v, _ := getLDAPConfig(t, env.AdminClient()); v.SecretsUnreadable {
		t.Fatalf("config = %+v, want it readable again under key version 1", v)
	}
}

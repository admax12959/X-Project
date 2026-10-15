package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/domain"
)

// testMailResult mirrors POST /api/system-config/smtp/test.
type testMailResult struct {
	OK     bool   `json:"ok"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
	To     string `json:"to"`
}

func sendTestMail(t *testing.T, c *apitest.Client, form map[string]any) testMailResult {
	t.Helper()
	var res testMailResult
	c.Post(smtpConfigPath+"/test", form).ExpectStatus(http.StatusOK).JSON(&res)
	return res
}

func TestTestMailUsesTheUnsavedFormAndGoesToTheAdministratorsContactEmail(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(smtpConfigPath, smtpForm(nil)).ExpectStatus(http.StatusOK)
	ca := strings.TrimSpace(testCAPEM(t))

	res := sendTestMail(t, admin, smtpForm(map[string]any{
		"host": "unsaved.corp.example", "port": 465, "security": "tls", "username": "typed-user", "password": "typed-not-saved",
		"from": "Typed <typed@corp.example>", "caCert": ca,
	}))

	if !res.OK || res.To != apitest.AdminEmail {
		t.Fatalf("result = %+v, want ok, sent to the contact email %s", res, apitest.AdminEmail)
	}
	sent := env.Mailer.Sent()
	if len(sent) != 1 {
		t.Fatalf("delivered messages = %d, want 1", len(sent))
	}
	cfg, msg := sent[0].Config, sent[0].Message
	if cfg.Host != "unsaved.corp.example" || cfg.Port != 465 || cfg.Security != "tls" || cfg.Username != "typed-user" ||
		cfg.Password != "typed-not-saved" || cfg.From != "Typed <typed@corp.example>" || cfg.CACert != ca {
		t.Fatalf("the test used %+v, want the unsaved form values", cfg)
	}
	if msg.To != apitest.AdminEmail || msg.From != "Typed <typed@corp.example>" || msg.Subject == "" || msg.Text == "" || msg.HTML == "" {
		t.Fatalf("message = %+v", msg)
	}
	// Nothing was saved or audited, and nothing went through the queue.
	if v, _ := getSMTPConfig(t, admin); v.Host != "smtp.corp.example" {
		t.Fatalf("testing changed the saved configuration: %+v", v)
	}
	if n := len(auditOf(t, admin, "configChanged")); n != 1 {
		t.Fatalf("config audit entries = %d, want only the one from the save", n)
	}
}

func TestTestMailWorksBeforeAnythingIsSavedAndNeedsNoSiteAddress(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)

	res := sendTestMail(t, env.AdminClient(), smtpForm(map[string]any{"siteUrl": ""}))

	if !res.OK || len(env.Mailer.Sent()) != 1 {
		t.Fatalf("result = %+v, sent = %d", res, len(env.Mailer.Sent()))
	}
}

func TestTestMailFallsBackToTheStoredPasswordWhenTheFormLeavesItEmpty(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	admin.Put(smtpConfigPath, smtpForm(nil)).ExpectStatus(http.StatusOK)

	sendTestMail(t, admin, smtpForm(map[string]any{"password": "", "host": "other.corp.example"}))

	if cfg, _ := env.Mailer.LastConfig(); cfg.Password != "smtp-secret-1" || cfg.Host != "other.corp.example" {
		t.Fatalf("the test used %+v, want the stored password and the typed host", cfg)
	}
}

func TestTestMailIsWrittenInTheAdministratorsLanguage(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	sendTestMail(t, admin, smtpForm(nil))
	patchMe(admin, map[string]any{"language": "en"}).ExpectStatus(http.StatusOK)
	sendTestMail(t, admin, smtpForm(nil))

	sent := env.Mailer.Sent()
	if len(sent) != 2 {
		t.Fatalf("delivered messages = %d, want 2", len(sent))
	}
	if !strings.Contains(sent[0].Message.Subject, "测试") || !strings.Contains(sent[0].Message.Text, "测试") {
		t.Errorf("no language chosen yet: subject %q, want Chinese (the default)", sent[0].Message.Subject)
	}
	if !strings.Contains(sent[1].Message.Subject, "Test") || strings.Contains(sent[1].Message.Text, "测试") {
		t.Errorf("language en: subject %q, want English", sent[1].Message.Subject)
	}
}

func TestAFailedTestMailShowsTheSpecificError(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	env.Mailer.FailWith(&domain.MailSendError{Code: "smtpAuthFailed", Detail: "535 5.7.8 Username and Password not accepted"})
	res := sendTestMail(t, admin, smtpForm(nil))
	if res.OK || res.Code != "smtpAuthFailed" || res.Detail != "535 5.7.8 Username and Password not accepted" {
		t.Fatalf("result = %+v, want the server's own words and code smtpAuthFailed", res)
	}

	env.Mailer.FailWith(&domain.MailSendError{Code: "smtpTlsFailed", Detail: "x509: certificate signed by unknown authority"})
	if res := sendTestMail(t, admin, smtpForm(nil)); res.OK || res.Code != "smtpTlsFailed" || !strings.Contains(res.Detail, "unknown authority") {
		t.Fatalf("result = %+v", res)
	}

	env.Mailer.FailWith(nil)
	if res := sendTestMail(t, admin, smtpForm(nil)); !res.OK {
		t.Fatalf("result = %+v, want ok once the server works", res)
	}
}

func TestATestMailFormIsValidatedAndPlaintextNeedsConfirmationHere(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()

	admin.Post(smtpConfigPath+"/test", smtpForm(map[string]any{"host": ""})).ExpectError(http.StatusUnprocessableEntity, "host", "required")
	admin.Post(smtpConfigPath+"/test", smtpForm(map[string]any{"security": "none"})).ExpectError(http.StatusUnprocessableEntity, "allowPlaintext", "required")
	if n := len(env.Mailer.Sent()); n != 0 {
		t.Fatalf("delivered messages = %d, want none for an invalid form", n)
	}
}

func TestOnlyAdministratorsMaySendATestMail(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	bob := plainUserClient(t, env, "bob")

	bob.Post(smtpConfigPath+"/test", smtpForm(nil)).ExpectError(http.StatusForbidden, "", "forbidden")

	if n := len(env.Mailer.Sent()) + len(env.Mailer.Configs()); n != 0 {
		t.Fatalf("a plain user made the application connect to a mail server (%d)", n)
	}
}

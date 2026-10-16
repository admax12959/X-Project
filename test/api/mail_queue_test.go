package api

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/admax12959/X-Project/internal/apitest"
	"github.com/admax12959/X-Project/internal/app"
	"github.com/admax12959/X-Project/internal/domain"
)

// Other features send mail by putting it into the database queue (Service.EnqueueMail); these
// tests do that through the env fixture env.EnqueueMail, because the features that use it
// (invitations, recovery, ...) are separate tickets. Delivery, retries and the outbox page are
// observed through the HTTP API and the fake SMTP side (env.Mailer).

type outboxItem struct {
	ID            string `json:"id"`
	To            string `json:"to"`
	Template      string `json:"template"`
	Subject       string `json:"subject"`
	Status        string `json:"status"`
	Attempts      int    `json:"attempts"`
	LastErrorCode string `json:"lastErrorCode"`
	LastError     string `json:"lastError"`
	CreatedAt     string `json:"createdAt"`
	NextAttemptAt string `json:"nextAttemptAt"`
	SentAt        string `json:"sentAt"`
	FailedAt      string `json:"failedAt"`
}

type outboxBody struct {
	Items  []outboxItem `json:"items"`
	Counts struct {
		Pending int `json:"pending"`
		Sent    int `json:"sent"`
		Failed  int `json:"failed"`
	} `json:"counts"`
}

func getOutbox(t *testing.T, c *apitest.Client, status string) (outboxBody, string) {
	t.Helper()
	path := "/api/mail-outbox"
	if status != "" {
		path += "?status=" + status
	}
	r := c.Get(path).ExpectStatus(http.StatusOK)
	var b outboxBody
	r.JSON(&b)
	return b, string(r.Body)
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func userIDOf(t *testing.T, u userBody) domain.UserID {
	t.Helper()
	id, err := strconv.ParseInt(u.ID, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return domain.UserID(id)
}

// mailEnv returns an env whose mail is not configured yet (and the admin client). Create the
// test's users first, then enableMail: once mail is configured, creating a user sends an
// invitation instead of setting a temporary password (#21).
func mailEnv(t *testing.T, mutate ...func(*app.Config)) (*apitest.Env, *apitest.Client) {
	t.Helper()
	env := apitest.New(t, mutate...)
	return env, env.AdminClient()
}

// enableMail saves a working SMTP configuration (site address https://xp.corp.example).
func enableMail(t *testing.T, admin *apitest.Client) {
	t.Helper()
	admin.Put(smtpConfigPath, smtpForm(nil)).ExpectStatus(http.StatusOK)
}

// configuredMail returns an env whose mail is configured (and the admin client).
func configuredMail(t *testing.T, mutate ...func(*app.Config)) (*apitest.Env, *apitest.Client) {
	t.Helper()
	env, admin := mailEnv(t, mutate...)
	enableMail(t, admin)
	return env, admin
}

func TestQueuedMailIsSentInTheBackgroundWithTheSavedConfiguration(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)

	err := env.EnqueueMail(domain.MailRequest{Template: domain.MailTemplateInvite, Recipient: userIDOf(t, bob), To: bob.Email, LinkPath: "/invite/tok-abc"})
	if err != nil {
		t.Fatal(err)
	}

	eventually(t, "the mail to be delivered", func() bool { return len(env.Mailer.SentTo(bob.Email)) == 1 })
	sent := env.Mailer.SentTo(bob.Email)[0]
	if sent.Config.Host != "smtp.corp.example" || sent.Config.Port != 587 || sent.Config.Security != "starttls" ||
		sent.Config.Username != "mailer" || sent.Config.Password != "smtp-secret-1" {
		t.Fatalf("delivered with %+v, want the saved configuration with the decrypted password", sent.Config)
	}
	if sent.Message.From != "X-Project <noreply@corp.example>" || sent.Message.Subject == "" {
		t.Fatalf("message = %+v", sent.Message)
	}
	eventually(t, "the outbox to show the mail as sent", func() bool {
		b, _ := getOutbox(t, admin, "sent")
		return len(b.Items) == 1 && b.Items[0].Attempts == 1 && b.Items[0].To == bob.Email && b.Items[0].Template == "invite" && b.Items[0].SentAt != ""
	})
}

func TestMailLinksAreBuiltFromTheConfiguredSiteAddress(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	req := domain.MailRequest{Template: domain.MailTemplateInvite, Recipient: userIDOf(t, bob), To: bob.Email, LinkPath: "/invite/tok-abc"}

	if err := env.EnqueueMail(req); err != nil {
		t.Fatal(err)
	}
	admin.Put(smtpConfigPath, smtpForm(map[string]any{"password": "", "siteUrl": "https://corp.example:8443/"})).ExpectStatus(http.StatusOK)
	if err := env.EnqueueMail(req); err != nil {
		t.Fatal(err)
	}

	eventually(t, "both mails", func() bool { return len(env.Mailer.SentTo(bob.Email)) == 2 })
	first, second := env.Mailer.SentTo(bob.Email)[0].Message, env.Mailer.SentTo(bob.Email)[1].Message
	for _, c := range []struct {
		m    domain.MailMessage
		link string
	}{{first, "https://xp.corp.example/invite/tok-abc"}, {second, "https://corp.example:8443/invite/tok-abc"}} {
		if !strings.Contains(c.m.Text, c.link) || !strings.Contains(c.m.HTML, `href="`+c.link+`"`) {
			t.Errorf("the mail does not carry the link %s:\n%s\n%s", c.link, c.m.Text, c.m.HTML)
		}
	}
}

func TestMailIsWrittenInTheRecipientsLanguageInBothFormatsForEveryTemplate(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	zh := createUser(t, admin, map[string]any{"username": "zhang", "displayName": "张三"})
	en := createUser(t, admin, map[string]any{"username": "ellen", "displayName": "Ellen"})
	fresh := createUser(t, admin, map[string]any{"username": "fresh", "displayName": "Fresh"}) // never chose a language
	enableMail(t, admin)
	for name, lang := range map[string]string{"zhang": "zh", "ellen": "en"} {
		c := env.Client()
		c.LoginAs(name, "initial-pw-1").ExpectStatus(http.StatusOK)
		patchMe(c, map[string]any{"language": lang}).ExpectStatus(http.StatusOK)
	}
	han := regexp.MustCompile(`\p{Han}`)

	for _, tpl := range domain.MailTemplates() {
		for _, c := range []struct {
			user    userBody
			chinese bool
		}{{zh, true}, {en, false}, {fresh, true}} {
			req := domain.MailRequest{Template: tpl, Recipient: userIDOf(t, c.user), To: c.user.Email, Data: map[string]string{"Code": "482913", "OldEmail": "old@example.com", "NewEmail": "new@example.com", "Kind": "ldap", "LoginName": "elarsen"}, LinkPath: "/x/tok"}
			if err := env.EnqueueMail(req); err != nil {
				t.Fatalf("%s: %v", tpl, err)
			}
		}
	}
	want := len(domain.MailTemplates()) * 3
	eventually(t, "all mails", func() bool { return len(env.Mailer.Sent()) == want })

	for _, s := range env.Mailer.Sent() {
		m := s.Message
		chinese := m.To != en.Email
		if m.Subject == "" || m.Text == "" || !strings.Contains(m.HTML, "<html") {
			t.Errorf("to %s: incomplete message %+v", m.To, m)
		}
		if han.MatchString(m.Subject+m.Text+m.HTML) != chinese {
			t.Errorf("to %s: wrong language (want Chinese=%v): %q", m.To, chinese, m.Subject)
		}
		if strings.Contains(m.Subject, "{{") || strings.Contains(m.Text, "{{") || strings.Contains(m.HTML, "{{") {
			t.Errorf("to %s: unrendered template in %q", m.To, m.Subject)
		}
	}
}

func TestHTMLMailEscapesTheRecipientsNameAndTextMailDoesNot(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	evil := createUser(t, admin, map[string]any{"username": "evil", "displayName": `<b>Eve</b> & "Co"`})
	enableMail(t, admin)

	if err := env.EnqueueMail(domain.MailRequest{Template: domain.MailTemplateVerifyCode, Recipient: userIDOf(t, evil), To: evil.Email, Data: map[string]string{"Code": "111222"}}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the mail", func() bool { return len(env.Mailer.Sent()) == 1 })
	m := env.Mailer.Sent()[0].Message
	if strings.Contains(m.HTML, "<b>Eve</b>") || !strings.Contains(m.HTML, "&lt;b&gt;Eve&lt;/b&gt;") {
		t.Errorf("HTML is not escaped: %s", m.HTML)
	}
	if !strings.Contains(m.Text, `<b>Eve</b> & "Co"`) || !strings.Contains(m.Text, "111222") || !strings.Contains(m.HTML, "111222") {
		t.Errorf("plain text = %s", m.Text)
	}
}

func TestFailedDeliveriesAreRetriedAndAFinalFailureShowsInTheOutbox(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t) // the test environment retries after 25ms, three times: four attempts in all
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	env.Mailer.FailWith(&domain.MailSendError{Code: "smtpAuthFailed", Detail: "535 5.7.8 Username and Password not accepted"})

	if err := env.EnqueueMail(domain.MailRequest{Template: domain.MailTemplateInvite, Recipient: userIDOf(t, bob), To: bob.Email, LinkPath: "/invite/secret-token-1"}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the mail to fail for good", func() bool { b, _ := getOutbox(t, admin, "failed"); return len(b.Items) == 1 })
	b, body := getOutbox(t, admin, "failed")
	it := b.Items[0]
	if it.Attempts != 4 || it.Status != "failed" || it.To != bob.Email || it.Template != "invite" || it.Subject == "" ||
		it.LastErrorCode != "smtpAuthFailed" || it.LastError != "535 5.7.8 Username and Password not accepted" || it.FailedAt == "" {
		t.Fatalf("outbox item = %+v, want a final failure after 4 attempts with the server's error", it)
	}
	if b.Counts.Failed != 1 || b.Counts.Pending != 0 || b.Counts.Sent != 0 {
		t.Fatalf("counts = %+v", b.Counts)
	}
	// The outbox shows what happened, never what was in the mail (links carry one-time tokens).
	if strings.Contains(body, "secret-token-1") {
		t.Fatalf("the outbox carries the content of the mail: %s", body)
	}
	// Giving up means giving up: no further attempts.
	time.Sleep(200 * time.Millisecond)
	if n := env.Mailer.Attempts(); n != 4 {
		t.Fatalf("delivery attempts = %d, want 4", n)
	}
}

func TestADeliveryThatSucceedsOnARetryIsSentOnce(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	env.Mailer.FailNext(2, &domain.MailSendError{Code: "smtpUnreachable", Detail: "dial tcp: i/o timeout"})

	if err := env.EnqueueMail(domain.MailRequest{Template: domain.MailTemplatePasswordReset, Recipient: userIDOf(t, bob), To: bob.Email, LinkPath: "/reset/tok"}); err != nil {
		t.Fatal(err)
	}

	eventually(t, "the mail to be sent after retries", func() bool { b, _ := getOutbox(t, admin, "sent"); return len(b.Items) == 1 })
	if b, _ := getOutbox(t, admin, "sent"); b.Items[0].Attempts != 3 {
		t.Fatalf("attempts = %d, want 3", b.Items[0].Attempts)
	}
	time.Sleep(150 * time.Millisecond)
	if n := len(env.Mailer.Sent()); n != 1 {
		t.Fatalf("delivered messages = %d, want exactly 1", n)
	}
	if b, _ := getOutbox(t, admin, "failed"); len(b.Items) != 0 {
		t.Fatalf("failed items = %+v", b.Items)
	}
}

func TestQueuedMailLivesInTheDatabaseAndIsSentByWhicheverProcessRunsTheWorker(t *testing.T) {
	t.Parallel()
	// A worker that wakes up once an hour: the mail has to wait in the database.
	env, admin := mailEnv(t, func(c *app.Config) { c.Mail.PollInterval = time.Hour })
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)

	if err := env.EnqueueMail(domain.MailRequest{Template: domain.MailTemplateInvite, Recipient: userIDOf(t, bob), To: bob.Email, LinkPath: "/invite/tok"}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)
	b, _ := getOutbox(t, admin, "")
	if len(env.Mailer.Sent()) != 0 || b.Counts.Pending != 1 || len(b.Items) != 1 || b.Items[0].Status != "pending" || b.Items[0].Attempts != 0 {
		t.Fatalf("sent = %d, outbox = %+v, want the mail waiting", len(env.Mailer.Sent()), b)
	}

	env.Restart(func(c *app.Config) { c.Mail.PollInterval = 20 * time.Millisecond })
	eventually(t, "the restarted process to send the mail", func() bool { return len(env.Mailer.SentTo(bob.Email)) == 1 })
}

func TestMailCannotBeQueuedWhileMailIsNotConfigured(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	admin := env.AdminClient()
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	req := domain.MailRequest{Template: domain.MailTemplateInvite, Recipient: userIDOf(t, bob), To: bob.Email, LinkPath: "/invite/tok"}

	err := env.EnqueueMail(req)
	if e, ok := domain.AsError(err); !ok || e.Kind != domain.KindUnavailable || e.Errors[0].Code != "mailNotConfigured" {
		t.Fatalf("EnqueueMail without SMTP = %v, want mailNotConfigured", err)
	}

	admin.Put(smtpConfigPath, smtpForm(map[string]any{"enabled": false})).ExpectStatus(http.StatusOK)
	if err := env.EnqueueMail(req); err == nil {
		t.Fatal("EnqueueMail with SMTP switched off succeeded")
	}
	if b, _ := getOutbox(t, admin, ""); len(b.Items) != 0 {
		t.Fatalf("outbox = %+v, want nothing queued", b.Items)
	}
}

func TestMailThatCannotBeRenderedIsRefusedWhenItIsQueued(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)

	// The verification-code template needs a code.
	if err := env.EnqueueMail(domain.MailRequest{Template: domain.MailTemplateVerifyCode, Recipient: userIDOf(t, bob), To: bob.Email}); err == nil {
		t.Fatal("a template missing its data was queued")
	}
	if err := env.EnqueueMail(domain.MailRequest{Template: "nope", Recipient: userIDOf(t, bob), To: bob.Email}); err == nil {
		t.Fatal("an unknown template was queued")
	}
	if err := env.EnqueueMail(domain.MailRequest{Template: domain.MailTemplateTest, Recipient: userIDOf(t, bob), To: "not-an-address"}); err == nil {
		t.Fatal("an invalid address was queued")
	}
	if b, _ := getOutbox(t, admin, ""); len(b.Items) != 0 {
		t.Fatalf("outbox = %+v, want nothing queued", b.Items)
	}
}

func TestOnlyAdministratorsMaySeeTheOutbox(t *testing.T) {
	t.Parallel()
	env := apitest.New(t)
	bob := plainUserClient(t, env, "bob")

	bob.Get("/api/mail-outbox").ExpectError(http.StatusForbidden, "", "forbidden")
	env.Client().Get("/api/mail-outbox").ExpectStatus(http.StatusUnauthorized)
	env.AdminClient().Get("/api/mail-outbox?status=bogus").ExpectError(http.StatusUnprocessableEntity, "status", "invalid")
}

func TestAOneTimeCredentialIsNotDeliveredByARetryAfterItHasExpired(t *testing.T) {
	t.Parallel()
	env, admin := mailEnv(t)
	bob := createUser(t, admin, map[string]any{"username": "bob"})
	enableMail(t, admin)
	frozenMidStep(env) // frozen: the retry the first failure schedules is not due yet
	env.Mailer.FailNext(1, &domain.MailSendError{Code: "smtpUnreachable", Detail: "dial tcp: i/o timeout"})

	if err := env.EnqueueMail(domain.MailRequest{Template: domain.MailTemplatePasswordReset, Recipient: userIDOf(t, bob), To: bob.Email, LinkPath: "/set-password/tok"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the first attempt", func() bool { return env.Mailer.Attempts() == 1 })
	env.Clock.Advance(61 * time.Minute) // the reset link is good for an hour

	eventually(t, "the mail to be given up", func() bool { b, _ := getOutbox(t, admin, "failed"); return len(b.Items) == 1 })
	b, _ := getOutbox(t, admin, "failed")
	if it := b.Items[0]; it.LastErrorCode != "mailExpired" || it.Attempts != 1 {
		t.Fatalf("outbox item = %+v, want given up as expired after the one attempt", it)
	}
	time.Sleep(100 * time.Millisecond)
	if n := env.Mailer.Attempts(); n != 1 || len(env.Mailer.Sent()) != 0 {
		t.Fatalf("attempts = %d, sent = %d, want the expired link never delivered", n, len(env.Mailer.Sent()))
	}
}

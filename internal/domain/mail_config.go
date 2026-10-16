package domain

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Mail service configuration (#20): the SMTP server the system sends authentication and
// security mail through, plus the site's external address the links in those mails are built
// from. Stored like every system configuration section (system_config.go, ADR-0005); the
// password is the only secret. The sender is built from the stored configuration whenever mail
// is sent, so a saved change is in effect at once and on every instance.

const (
	smtpSecretPassword = "password"

	maxMailFieldLength = 1000
	maxCACertLength    = 100_000
	smtpTimeout        = 20 * time.Second
)

// SMTPSettings are the non-secret mail fields. The JSON tags are the stored format and the
// API names. There is no switch to skip certificate verification, on purpose.
type SMTPSettings struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	// Security is MailSecurityTLS (465), MailSecurityStartTLS (587) or MailSecurityNone (25).
	Security string `json:"security"`
	// AllowPlaintext is the administrator's explicit confirmation that mail (and the account
	// password) may travel unencrypted. Required for MailSecurityNone, dropped otherwise.
	AllowPlaintext bool   `json:"allowPlaintext"`
	Username       string `json:"username"`
	From           string `json:"from"`
	// CACert is an optional PEM bundle trusted in addition to the system roots.
	CACert string `json:"caCert"`
	// SiteURL is the external address of this site, for example https://xp.corp.example. Links
	// in mail are built from it, never from the Host header of a request.
	SiteURL string `json:"siteUrl"`
}

// SMTPConfigInput is a save or test request, decoded from the request body as it is (the
// settings flatten into it). Password is write-only: empty keeps the stored password,
// anything else replaces it.
type SMTPConfigInput struct {
	SMTPSettings
	Password string `json:"password"`
}

// SMTPConfigView is what an administrator gets to see. The password is never part of it, only
// whether one is set. SecretsUnreadable means a stored secret cannot be decrypted with the
// current master key (it changed): enter the password again. Configured means mail can be sent.
type SMTPConfigView struct {
	SMTPSettings
	PasswordSet       bool
	SecretsUnreadable bool
	Configured        bool
	UpdatedAt         time.Time
	Saved             bool // saved at least once; false = defaults shown
}

func smtpDefaults() SMTPSettings {
	return SMTPSettings{Port: 587, Security: MailSecurityStartTLS}
}

func defaultSMTPPort(security string) int {
	switch security {
	case MailSecurityTLS:
		return 465
	case MailSecurityNone:
		return 25
	}
	return 587
}

// smtpSection declares the mail section for the shared skeleton (config_section.go).
var smtpSection = configSection[SMTPSettings, SMTPConfigInput]{
	name: SectionSMTP, defaults: smtpDefaults,
	secrets: []sectionSecret[SMTPSettings]{{
		name:       smtpSecretPassword,
		unusedWhen: func(m SMTPSettings) bool { return m.Username == "" }, // no account: no password to keep
	}},
	typed: func(in SMTPConfigInput) map[string]string {
		return map[string]string{smtpSecretPassword: in.Password}
	},
	validate: func(in SMTPConfigInput, stored configSecrets) (SMTPSettings, map[string]string, error) {
		m, password, err := validateSMTP(in, stored, false)
		return m, map[string]string{smtpSecretPassword: password}, err
	},
}

func (s *Service) readSMTP(ctx context.Context, repo Repository) (settings SMTPSettings, saved bool, at time.Time, err error) {
	return readSettings(ctx, repo, smtpSection)
}

// configured: mail is switched on and has a server and a sender to use.
func (m SMTPSettings) configured() bool { return m.Enabled && m.Host != "" && m.From != "" }

// readUsableSMTP is readSMTP plus the answer to "can mail be sent with it": it is configured and,
// when a password is stored, that password can be decrypted with the current master key. An
// unreadable password makes SendDueMail leave everything queued, so no feature may count on mail.
func (s *Service) readUsableSMTP(ctx context.Context, repo Repository) (SMTPSettings, bool, error) {
	settings, _, _, err := s.readSMTP(ctx, repo)
	if err != nil || !settings.configured() {
		return settings, false, err
	}
	secrets, err := s.loadSecrets(ctx, repo, SectionSMTP)
	if err != nil {
		return settings, false, err
	}
	return settings, !secrets.unreadable[smtpSecretPassword], nil
}

// GetSMTPConfig returns the mail configuration for the Email page. Administrators only.
func (s *Service) GetSMTPConfig(ctx context.Context, actor Actor) (SMTPConfigView, error) {
	st, err := getSection(ctx, s, actor, smtpSection)
	if err != nil {
		return SMTPConfigView{}, err
	}
	return smtpViewOf(st), nil
}

func smtpViewOf(st sectionState[SMTPSettings]) SMTPConfigView {
	return SMTPConfigView{
		SMTPSettings: st.settings, PasswordSet: st.secrets.set[smtpSecretPassword], SecretsUnreadable: st.secrets.anyUnreadable(),
		Configured: st.settings.configured() && !st.secrets.unreadable[smtpSecretPassword], UpdatedAt: st.at, Saved: st.saved,
	}
}

// MailConfigured reports whether mail can be sent: the question other features ask to decide
// whether they are available (invitations by mail, password recovery, ...). Any signed-in
// user may ask.
func (s *Service) MailConfigured(ctx context.Context) (bool, error) {
	_, ok, err := s.readUsableSMTP(ctx, s.repo)
	return ok, err
}

var hostnamePattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)

// validateSMTP checks a request against the stored state (the stored password counts when the
// request leaves the password empty) and returns the settings to save and the password to use.
// A test mail is sent with whatever was typed, so forTest does not insist on the site address,
// which only links need.
func validateSMTP(in SMTPConfigInput, secrets configSecrets, forTest bool) (SMTPSettings, string, error) {
	var c Collector
	m := in.SMTPSettings
	trim := func(field string, p *string, max int) {
		*p = strings.TrimSpace(*p)
		if len([]rune(*p)) > max {
			c.Add(field, CodeTooLong, map[string]any{"max": max})
		}
	}
	trim("host", &m.Host, maxMailFieldLength)
	trim("username", &m.Username, maxMailFieldLength)
	trim("from", &m.From, maxMailFieldLength)
	trim("siteUrl", &m.SiteURL, maxMailFieldLength)
	trim("caCert", &m.CACert, maxCACertLength)
	m.Security = strings.TrimSpace(m.Security)
	if m.Security == "" {
		m.Security = MailSecurityStartTLS
	}

	switch m.Security {
	case MailSecurityTLS, MailSecurityStartTLS, MailSecurityNone:
	default:
		c.Add("security", CodeInvalid, map[string]any{"allowed": []string{MailSecurityTLS, MailSecurityStartTLS, MailSecurityNone}})
	}
	if m.Port == 0 {
		m.Port = defaultSMTPPort(m.Security)
	}
	if m.Port < 1 || m.Port > 65535 {
		c.Add("port", CodeInvalid, map[string]any{"min": 1, "max": 65535})
	}
	if m.Host != "" && !hostnamePattern.MatchString(m.Host) && net.ParseIP(strings.Trim(m.Host, "[]")) == nil {
		c.Add("host", CodeInvalid, nil)
	} else if m.Enabled {
		c.Required("host", m.Host)
	}
	if m.Security == MailSecurityNone {
		if !m.AllowPlaintext {
			c.Add("allowPlaintext", CodeRequired, nil)
		}
	} else {
		m.AllowPlaintext = false
	}
	if m.From != "" {
		if a, err := mail.ParseAddress(m.From); err != nil || !strings.Contains(a.Address, "@") {
			c.Add("from", CodeInvalid, nil)
		}
	} else if m.Enabled {
		c.Add("from", CodeRequired, nil)
	}
	if m.CACert != "" && !validCACerts(m.CACert) {
		c.Add("caCert", CodeInvalid, nil)
	}
	if m.SiteURL != "" {
		if site, ok := normalizeSiteURL(m.SiteURL); ok {
			m.SiteURL = site
		} else {
			c.Add("siteUrl", CodeInvalid, map[string]any{"allowed": []string{"http://", "https://"}})
		}
	} else if m.Enabled && !forTest {
		c.Add("siteUrl", CodeRequired, nil)
	}

	password := ""
	switch {
	case m.Username == "" && in.Password != "":
		c.Add("username", CodeRequired, nil) // a password without an account to use it
	case m.Username != "":
		password = in.Password
		if password == "" {
			password = secrets.plain[smtpSecretPassword]
		}
		if password == "" {
			c.Add("password", CodeRequired, nil)
		}
	}
	if err := c.Err(); err != nil {
		return SMTPSettings{}, "", err
	}
	return m, password, nil
}

func validCACerts(bundle string) bool {
	rest, n := []byte(bundle), 0
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return n > 0 && len(strings.TrimSpace(string(rest))) == 0
		}
		if block.Type != "CERTIFICATE" {
			return false
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return false
		}
		n++
	}
}

// normalizeSiteURL accepts an http(s) address with a host and returns it without a trailing
// slash. A path prefix is refused: the web app only serves root-relative routes, so a link
// below a prefix would not open the page.
func normalizeSiteURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || (u.Path != "" && u.Path != "/") || strings.Contains(raw, "?") || strings.Contains(raw, "#") {
		return "", false
	}
	return strings.TrimRight(u.String(), "/"), true
}

// UpdateSMTPConfig saves the mail configuration and records which fields changed (names only)
// in the audit log. Administrators only. The new configuration is in effect at once.
func (s *Service) UpdateSMTPConfig(ctx context.Context, actor Actor, in SMTPConfigInput) (SMTPConfigView, error) {
	st, err := updateSection(ctx, s, actor, smtpSection, in)
	if err != nil {
		return SMTPConfigView{}, err
	}
	return smtpViewOf(st), nil
}

// MailTestResult is the answer of "send test email". A failed delivery is an expected outcome,
// not an error of the request: OK=false carries the failure Code (rendered by the frontend as
// "error.<code>") and the Detail, the mail server's or the network's own words.
type MailTestResult struct {
	OK     bool
	Code   string
	Detail string
	To     string // the address the test mail was sent to
}

// TestSMTPConfig sends a test email, with the (unsaved) form values, to the contact email of
// the administrator who asks, in the administrator's language. A password left empty falls
// back to the stored one. Nothing is saved, queued or audited. Administrators only.
func (s *Service) TestSMTPConfig(ctx context.Context, actor Actor, in SMTPConfigInput) (MailTestResult, error) {
	if err := requirePermission(actor, PermConfigManage); err != nil {
		return MailTestResult{}, err
	}
	secrets, err := s.loadSecrets(ctx, s.repo, SectionSMTP)
	if err != nil {
		return MailTestResult{}, err
	}
	in.Enabled = true // testing a form means: as if it were switched on
	settings, password, err := validateSMTP(in, secrets, true)
	if err != nil {
		return MailTestResult{}, err
	}
	to := strings.TrimSpace(actor.User.Email)
	if to == "" {
		return MailTestResult{}, Validation(Invalid("", CodeNoContactEmail, nil))
	}
	res := MailTestResult{To: to}
	content, err := renderMail(MailTemplateTest, mailLanguageOf(actor.User.Language), map[string]any{"Name": actor.User.DisplayName})
	if err != nil {
		return MailTestResult{}, err
	}
	if s.mailSenders == nil {
		res.Code = CodeSmtpUnreachable
		return res, nil
	}
	sender, err := s.mailSenders.Build(smtpConfig(settings, password))
	if err != nil {
		res.Code, res.Detail = CodeSmtpRejected, err.Error()
		return res, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*smtpTimeout)
	defer cancel()
	if err := sender.Send(ctx, MailMessage{From: settings.From, To: to, Subject: content.Subject, Text: content.Text, HTML: content.HTML}); err != nil {
		res.Code, res.Detail = mailFailure(err)
		return res, nil
	}
	res.OK = true
	return res, nil
}

// mailFailure turns a delivery error into the code and the detail shown to the administrator.
func mailFailure(err error) (code, detail string) {
	if e, ok := err.(*MailSendError); ok {
		return e.Code, e.Detail
	}
	return CodeSmtpUnreachable, err.Error()
}

func smtpConfig(m SMTPSettings, password string) SMTPConfig {
	return SMTPConfig{
		Host: m.Host, Port: m.Port, Security: m.Security, Username: m.Username, Password: password,
		From: m.From, CACert: m.CACert, Timeout: smtpTimeout,
	}
}

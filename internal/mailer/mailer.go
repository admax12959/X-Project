// Package mailer is the real domain.MailSenderFactory: it delivers a message to one SMTP
// server (#20). Tests of the domain use internal/fakes instead; this package has its own
// tests against a small in-process SMTP server.
//
// Security rules (iam-scope "邮件"):
//   - The server certificate is always verified. There is no option to skip it; a private CA
//     is trusted through SMTPConfig.CACert, in addition to the system roots.
//   - MailSecurityStartTLS means STARTTLS is required: a server that does not offer it is an
//     error, never a silent fall back to plaintext.
//   - Only MailSecurityNone sends the account password unencrypted; the domain only lets an
//     administrator choose it with an explicit confirmation.
package mailer

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// Factory builds senders for SMTP configurations.
type Factory struct{}

var _ domain.MailSenderFactory = Factory{}

// Build checks the configuration and returns a sender. It does not connect.
func (Factory) Build(cfg domain.SMTPConfig) (domain.MailSender, error) {
	if cfg.Host == "" || cfg.Port < 1 || cfg.Port > 65535 {
		return nil, errors.New("smtp: host and port are required")
	}
	switch cfg.Security {
	case domain.MailSecurityTLS, domain.MailSecurityStartTLS, domain.MailSecurityNone:
	default:
		return nil, fmt.Errorf("smtp: unknown connection security %q", cfg.Security)
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("smtp: sender address: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if strings.TrimSpace(cfg.CACert) != "" && !roots.AppendCertsFromPEM([]byte(cfg.CACert)) {
		return nil, errors.New("smtp: the custom CA contains no certificate")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 20 * time.Second
	}
	return &sender{cfg: cfg, from: from, tls: &tls.Config{ServerName: cfg.Host, RootCAs: roots, MinVersion: tls.VersionTLS12}}, nil
}

type sender struct {
	cfg  domain.SMTPConfig
	from *mail.Address
	tls  *tls.Config
}

func fail(code string, err error) error {
	return &domain.MailSendError{Code: code, Detail: detail(err)}
}

// detail is the error text for the administrator: the server's own words, one line, no
// longer than a page.
func detail(err error) string {
	s := strings.Join(strings.Fields(err.Error()), " ")
	if r := []rune(s); len(r) > 400 {
		s = string(r[:400])
	}
	return s
}

// Send delivers msg over a new connection.
func (s *sender) Send(ctx context.Context, msg domain.MailMessage) error {
	to, err := mail.ParseAddress(msg.To)
	if err != nil {
		return fail(domain.CodeSmtpRecipientRejected, err)
	}
	body, err := s.build(msg, to)
	if err != nil {
		return fail(domain.CodeSmtpRejected, err)
	}

	d := net.Dialer{Timeout: s.cfg.Timeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port)))
	if err != nil {
		return fail(domain.CodeSmtpUnreachable, err)
	}
	defer conn.Close()
	deadline := time.Now().Add(2 * s.cfg.Timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)

	if s.cfg.Security == domain.MailSecurityTLS {
		tc := tls.Client(conn, s.tls)
		if err := tc.HandshakeContext(ctx); err != nil {
			return fail(domain.CodeSmtpTLSFailed, err)
		}
		conn = tc
	}
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return fail(codeOf(err, domain.CodeSmtpUnreachable), err)
	}
	defer c.Close()
	if err := c.Hello(helloName()); err != nil {
		return fail(codeOf(err, domain.CodeSmtpRejected), err)
	}
	if s.cfg.Security == domain.MailSecurityStartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fail(domain.CodeSmtpStartTLSMissing, errors.New("the server does not offer STARTTLS"))
		}
		if err := c.StartTLS(s.tls); err != nil {
			return fail(domain.CodeSmtpTLSFailed, err)
		}
	}
	if s.cfg.Username != "" {
		auth, err := s.pickAuth(c)
		if err != nil {
			return fail(domain.CodeSmtpAuthFailed, err)
		}
		if err := c.Auth(auth); err != nil {
			return fail(domain.CodeSmtpAuthFailed, err)
		}
	}
	if err := c.Mail(s.from.Address); err != nil {
		return fail(domain.CodeSmtpSenderRejected, err)
	}
	if err := c.Rcpt(to.Address); err != nil {
		return fail(domain.CodeSmtpRecipientRejected, err)
	}
	w, err := c.Data()
	if err != nil {
		return fail(codeOf(err, domain.CodeSmtpRejected), err)
	}
	if _, err := w.Write(body); err != nil {
		return fail(domain.CodeSmtpUnreachable, err)
	}
	if err := w.Close(); err != nil {
		return fail(codeOf(err, domain.CodeSmtpRejected), err)
	}
	_ = c.Quit()
	return nil
}

// codeOf: a server's reply is "refused" (fallback given); anything else is a network problem.
func codeOf(err error, fallback string) string {
	var te *textproto.Error
	if errors.As(err, &te) {
		return fallback
	}
	var ne net.Error
	if errors.As(err, &ne) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return domain.CodeSmtpUnreachable
	}
	return fallback
}

func helloName() string {
	if h, err := os.Hostname(); err == nil && h != "" && !strings.ContainsAny(h, " \t") {
		return h
	}
	return "localhost"
}

// build writes the message: headers, then multipart/alternative with the plain text first.
func (s *sender) build(msg domain.MailMessage, to *mail.Address) ([]byte, error) {
	var id [12]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, err
	}
	domainPart := "localhost"
	if i := strings.LastIndex(s.from.Address, "@"); i >= 0 {
		domainPart = s.from.Address[i+1:]
	}
	var b strings.Builder
	h := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	h("From", s.from.String())
	h("To", to.String())
	h("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	h("Date", time.Now().UTC().Format(time.RFC1123Z))
	h("Message-ID", "<"+hex.EncodeToString(id[:])+"@"+domainPart+">")
	h("MIME-Version", "1.0")
	h("Auto-Submitted", "auto-generated")

	var parts strings.Builder
	mw := multipart.NewWriter(&parts)
	h("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
	b.WriteString("\r\n")
	for _, p := range []struct{ typ, body string }{{"text/plain", msg.Text}, {"text/html", msg.HTML}} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {p.typ + "; charset=utf-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		qw := quotedprintable.NewWriter(pw)
		if _, err := qw.Write([]byte(p.body)); err != nil {
			return nil, err
		}
		if err := qw.Close(); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return []byte(b.String() + parts.String()), nil
}

// pickAuth chooses PLAIN or LOGIN from what the server offers. net/smtp's own PlainAuth
// refuses to run without TLS; the domain only allows an unencrypted connection with the
// administrator's explicit confirmation, so here the choice is already made.
func (s *sender) pickAuth(c *smtp.Client) (smtp.Auth, error) {
	ok, offered := c.Extension("AUTH")
	if !ok {
		return nil, errors.New("the server does not offer authentication")
	}
	have := map[string]bool{}
	for _, m := range strings.Fields(strings.ToUpper(offered)) {
		have[m] = true
	}
	switch {
	case have["PLAIN"]:
		return plainAuth{user: s.cfg.Username, password: s.cfg.Password}, nil
	case have["LOGIN"]:
		return loginAuth{user: s.cfg.Username, password: s.cfg.Password}, nil
	}
	return nil, fmt.Errorf("the server offers no supported authentication (%s)", strings.TrimSpace(offered))
}

type plainAuth struct{ user, password string }

func (a plainAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.user + "\x00" + a.password), nil
}

func (a plainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("unexpected server challenge")
	}
	return nil, nil
}

type loginAuth struct{ user, password string }

func (a loginAuth) Start(*smtp.ServerInfo) (string, []byte, error) { return "LOGIN", nil, nil }

func (a loginAuth) Next(challenge []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(string(challenge))) {
	case "username:", "user name:":
		return []byte(a.user), nil
	case "password:":
		return []byte(a.password), nil
	}
	return nil, fmt.Errorf("unexpected server challenge %q", challenge)
}

package domain

import (
	"context"
	"time"
)

// Outgoing mail ports (#20). The domain builds the message; an adapter (internal/mailer for
// production, internal/fakes for tests) delivers it over SMTP.

// Connection security of the SMTP server. There is deliberately no way to skip the
// verification of the server certificate (ADR-0005, iam-scope "邮件"): a private CA is
// handled by SMTPConfig.CACert.
const (
	MailSecurityTLS      = "tls"      // implicit TLS, usually port 465
	MailSecurityStartTLS = "starttls" // STARTTLS is required, never silently skipped; usually port 587
	MailSecurityNone     = "none"     // no encryption; only with the administrator's explicit confirmation
)

// SMTPConfig is everything needed to deliver mail, with the password decrypted. It is built
// from the stored configuration every time mail is sent, and from the unsaved form for the
// test mail.
type SMTPConfig struct {
	Host     string
	Port     int
	Security string
	Username string // empty: no authentication
	Password string
	From     string // the sender address, "addr@host" or "Name <addr@host>"
	CACert   string // PEM, optional; trusted in addition to the system roots
	Timeout  time.Duration
}

// MailMessage is one message ready to be delivered. Text and HTML are the two alternatives of
// the same content.
type MailMessage struct {
	From    string
	To      string
	Subject string
	Text    string
	HTML    string
}

// MailSendError is how an adapter reports a failed delivery: Code says what kind of failure
// it was (the frontend shows "error.<code>"), Detail is the server's or the network's own
// words for the administrator. Detail never contains a secret.
type MailSendError struct {
	Code   string
	Detail string
}

func (e *MailSendError) Error() string { return e.Code + ": " + e.Detail }

// MailSender delivers messages through one SMTP configuration.
type MailSender interface {
	Send(ctx context.Context, msg MailMessage) error
}

// MailSenderFactory creates a MailSender for a configuration. It does not connect. A nil
// factory in Options means mail cannot be sent at all.
type MailSenderFactory interface {
	Build(cfg SMTPConfig) (MailSender, error)
}

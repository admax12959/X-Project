package domain

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
)

// The outgoing mail queue (#20). A feature that wants to send mail calls EnqueueMail (or
// enqueueMail with its own transaction, so the mail exists exactly when the change that caused
// it does). A background task (RunMailWorker) delivers the queue: a failed delivery is tried
// again after a backoff, and after the last attempt the mail is marked failed, which the
// administrator sees on the Email page. Only authentication and security mail goes through
// here (iam-scope "邮件"); no business notifications.
//
// Before asking for mail, a feature asks MailConfigured: without SMTP the mail features are
// unavailable and EnqueueMail refuses with mailNotConfigured.

// MailStatus is where a queued mail is in its life.
type MailStatus string

const (
	MailPending MailStatus = "pending" // waiting for its next delivery attempt
	MailSent    MailStatus = "sent"
	MailFailed  MailStatus = "failed" // every attempt failed; the mail is given up
)

// MailEntry is a row of the queue. The rendered content (Subject, Text, HTML) is only kept
// while the mail is pending; Subject is kept for the outbox page.
type MailEntry struct {
	ID            int64
	To            string
	Recipient     UserID // 0 when the address does not belong to a user
	Template      string
	Lang          string
	Subject       string
	Text          string
	HTML          string
	Status        MailStatus
	Attempts      int
	NextAttemptAt time.Time
	LastErrorCode string
	LastError     string
	CreatedAt     time.Time
	SentAt        time.Time // zero unless sent
	FailedAt      time.Time // zero unless failed
}

// MailOutboxRepository is the persistence port of the queue.
type MailOutboxRepository interface {
	InsertMail(ctx context.Context, e MailEntry) (int64, error)
	// ClaimDueMail leases up to limit pending mails whose next attempt is due and that no
	// other worker holds, for the duration of lease.
	ClaimDueMail(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]MailEntry, error)
	// MarkMailSent / MarkMailFailed end the life of a mail and drop its content.
	MarkMailSent(ctx context.Context, id int64, attempts int, at time.Time) error
	MarkMailFailed(ctx context.Context, id int64, attempts int, at time.Time, code, detail string) error
	// MarkMailRetry records a failed attempt and when to try again.
	MarkMailRetry(ctx context.Context, id int64, attempts int, next time.Time, code, detail string) error
	// ListMail returns mail newest first, optionally of one status.
	ListMail(ctx context.Context, status MailStatus, limit int) ([]MailEntry, error)
	CountMail(ctx context.Context) (map[MailStatus]int, error)
}

// DefaultMailRetryBackoff is how long to wait after the first, second, ... failed attempt.
// Delivery is attempted len(backoff)+1 times in all.
var DefaultMailRetryBackoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour}

const (
	mailClaimLease = 10 * time.Minute
	mailBatchSize  = 20
	maxMailError   = 500
	mailOutboxPage = 100
)

// MailRequest is what a feature hands to EnqueueMail.
type MailRequest struct {
	Template MailTemplate
	// To is the recipient address. May be empty when Recipient is set: the user's contact email.
	To string
	// Recipient is the user the mail is for; their language preference picks the language and
	// their display name is the template's .Name. 0 for an address that is not (yet) a user's.
	Recipient UserID
	// Lang is used when Recipient is 0 or has not chosen a language ("zh" or "en"; default zh).
	Lang string
	// LinkPath is appended to the site's external address to make the template's .Link, for
	// example "/invite/<token>". The Host header of a request is never used.
	LinkPath string
	// Data is extra template data, for example {"Code": "482913"}.
	Data map[string]string
}

// EnqueueMail puts a mail into the queue in its own transaction.
func (s *Service) EnqueueMail(ctx context.Context, req MailRequest) error {
	return s.repo.InTx(ctx, func(tx Repository) error {
		_, err := s.enqueueMail(ctx, tx, req)
		return err
	})
}

// enqueueMail renders the mail and puts it into the queue within tx. It refuses when mail is
// not configured, when the template cannot be rendered with the data given and when the
// address is not a plain address, so a mail that is queued can be delivered.
func (s *Service) enqueueMail(ctx context.Context, tx Repository, req MailRequest) (int64, error) {
	settings, usable, err := s.readUsableSMTP(ctx, tx)
	if err != nil {
		return 0, err
	}
	if !usable {
		return 0, newKind(KindUnavailable, CodeMailNotConfigured)
	}
	if !knownMailTemplate(req.Template) {
		return 0, fmt.Errorf("mail: unknown template %q", req.Template)
	}

	to := strings.TrimSpace(req.To)
	lang := req.Lang
	name := ""
	if req.Recipient != 0 {
		u, ok, err := tx.GetUser(ctx, req.Recipient)
		if err != nil {
			return 0, err
		}
		if !ok {
			return 0, fmt.Errorf("mail: recipient user %d does not exist", req.Recipient)
		}
		if to == "" {
			to = strings.TrimSpace(u.Email)
		}
		lang, name = u.Language, u.DisplayName
	}
	if a, err := mail.ParseAddress(to); err != nil || a.Address != to || strings.ContainsAny(to, "\r\n") {
		return 0, Validation(Invalid("to", CodeInvalid, nil))
	}
	lang = mailLanguageOf(lang)

	data := map[string]any{"Name": name}
	for k, v := range req.Data {
		data[k] = v
	}
	if req.LinkPath != "" {
		if settings.SiteURL == "" {
			return 0, newKind(KindUnavailable, CodeSiteURLRequired)
		}
		if !strings.HasPrefix(req.LinkPath, "/") {
			return 0, fmt.Errorf("mail: link path %q must start with /", req.LinkPath)
		}
		data["Link"] = settings.SiteURL + req.LinkPath
	}
	content, err := renderMail(req.Template, lang, data)
	if err != nil {
		return 0, err
	}
	now := s.nowUTC()
	return tx.InsertMail(ctx, MailEntry{
		To: to, Recipient: req.Recipient, Template: string(req.Template), Lang: lang,
		Subject: content.Subject, Text: content.Text, HTML: content.HTML,
		NextAttemptAt: now, CreatedAt: now,
	})
}

func knownMailTemplate(t MailTemplate) bool {
	for _, k := range mailTemplates {
		if k == t {
			return true
		}
	}
	return false
}

// RunMailWorker delivers the queue until ctx is cancelled: a pass now, then one every
// interval. Any number of processes may run it; they share the work through the database.
func (s *Service) RunMailWorker(ctx context.Context, interval time.Duration) {
	for {
		if err := s.SendDueMail(ctx); err != nil && ctx.Err() == nil {
			slog.Error("mail: delivering the queue", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// SendDueMail makes one pass over the queue: every pending mail that is due is delivered with
// the saved SMTP configuration, or gets its next attempt scheduled, or is given up. While mail
// is not configured the queue just waits.
func (s *Service) SendDueMail(ctx context.Context) error {
	if s.mailSenders == nil {
		return nil
	}
	settings, _, _, err := s.readSMTP(ctx, s.repo)
	if err != nil || !settings.configured() {
		return err
	}
	secrets, err := s.loadSecrets(ctx, s.repo, SectionSMTP)
	if err != nil {
		return err
	}
	if settings.Username != "" && secrets.plain[smtpSecretPassword] == "" {
		slog.Error("mail: the stored SMTP password cannot be decrypted with the current master key; enter it again on the Email page")
		return nil
	}
	sender, err := s.mailSenders.Build(smtpConfig(settings, secrets.plain[smtpSecretPassword]))
	if err != nil {
		slog.Error("mail: the stored SMTP configuration cannot be used", "err", err)
		return nil
	}
	for ctx.Err() == nil {
		entries, err := s.repo.ClaimDueMail(ctx, s.nowUTC(), mailClaimLease, mailBatchSize)
		if err != nil || len(entries) == 0 {
			return err
		}
		for _, e := range entries {
			if err := s.deliver(ctx, sender, settings.From, e); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

// mailCredentialLifetime is how long the one-time credential a template carries stays good,
// counted from when the mail was queued (the token is issued in the same transaction). 0: the
// mail carries none.
func mailCredentialLifetime(t MailTemplate) time.Duration {
	switch t {
	case MailTemplateInvite:
		return inviteTokenTTL
	case MailTemplatePasswordReset:
		return passwordResetTokenTTL
	case MailTemplateVerifyCode:
		return emailCodeTTL
	}
	return 0
}

func (s *Service) deliver(ctx context.Context, sender MailSender, from string, e MailEntry) error {
	// A retry must not hand out a link or code that no longer works.
	if life := mailCredentialLifetime(MailTemplate(e.Template)); life > 0 && !s.nowUTC().Before(e.CreatedAt.Add(life)) {
		slog.Warn("mail: giving up, the credential in it has expired", "id", e.ID, "template", e.Template, "attempts", e.Attempts)
		return s.repo.MarkMailFailed(ctx, e.ID, e.Attempts, s.nowUTC(), CodeMailExpired, "the link or code in this mail expired before it could be delivered")
	}
	sendCtx, cancel := context.WithTimeout(ctx, 2*smtpTimeout)
	defer cancel()
	sendErr := sender.Send(sendCtx, MailMessage{From: from, To: e.To, Subject: e.Subject, Text: e.Text, HTML: e.HTML})
	if ctx.Err() != nil {
		return ctx.Err() // shutting down: the lease runs out and another worker retries
	}
	attempts := e.Attempts + 1
	now := s.nowUTC()
	if sendErr == nil {
		return s.repo.MarkMailSent(ctx, e.ID, attempts, now)
	}
	code, detail := mailFailure(sendErr)
	if r := []rune(detail); len(r) > maxMailError {
		detail = string(r[:maxMailError])
	}
	if attempts > len(s.mailBackoff) {
		slog.Warn("mail: giving up", "id", e.ID, "template", e.Template, "attempts", attempts, "code", code)
		return s.repo.MarkMailFailed(ctx, e.ID, attempts, now, code, detail)
	}
	return s.repo.MarkMailRetry(ctx, e.ID, attempts, now.Add(s.mailBackoff[attempts-1]), code, detail)
}

// MailOutbox is the administrator's view of the queue.
type MailOutbox struct {
	Items   []MailEntry
	Pending int
	Sent    int
	Failed  int
}

// ListMailOutbox returns the newest queued mail, optionally of one status ("pending", "sent"
// or "failed"), with the number of mails in each status. Administrators only.
func (s *Service) ListMailOutbox(ctx context.Context, actor Actor, status string) (MailOutbox, error) {
	if err := requirePermission(actor, PermConfigManage); err != nil {
		return MailOutbox{}, err
	}
	switch MailStatus(status) {
	case "", MailPending, MailSent, MailFailed:
	default:
		return MailOutbox{}, Validation(Invalid("status", CodeInvalid, map[string]any{"allowed": []string{"pending", "sent", "failed"}}))
	}
	items, err := s.repo.ListMail(ctx, MailStatus(status), mailOutboxPage)
	if err != nil {
		return MailOutbox{}, err
	}
	counts, err := s.repo.CountMail(ctx)
	if err != nil {
		return MailOutbox{}, err
	}
	return MailOutbox{Items: items, Pending: counts[MailPending], Sent: counts[MailSent], Failed: counts[MailFailed]}, nil
}

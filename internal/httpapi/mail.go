package httpapi

import (
	"net/http"
	"strconv"

	"github.com/admax12959/X-Project/internal/domain"
)

// Mail (#20). The configuration is a system configuration section (administrators only,
// config.manage; the password is write-only). Whether mail is configured may be asked by any
// signed-in user.

func (s *server) mailRoutes() {
	s.route("GET /api/system-config/smtp", s.getSMTPConfig)
	s.route("PUT /api/system-config/smtp", s.putSMTPConfig)
	s.route("POST /api/system-config/smtp/test", s.testSMTPConfig)
	s.route("GET /api/mail/status", s.mailStatus)
	s.route("GET /api/mail-outbox", s.mailOutbox)
}

// smtpConfigBody is the request of PUT and POST .../test: the whole form. password empty or
// absent keeps the stored password.
type smtpConfigBody struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Security       string `json:"security"`
	AllowPlaintext bool   `json:"allowPlaintext"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	From           string `json:"from"`
	CACert         string `json:"caCert"`
	SiteURL        string `json:"siteUrl"`
}

func (b smtpConfigBody) input() domain.SMTPConfigInput {
	return domain.SMTPConfigInput{Password: b.Password, SMTPSettings: domain.SMTPSettings{
		Enabled: b.Enabled, Host: b.Host, Port: b.Port, Security: b.Security, AllowPlaintext: b.AllowPlaintext,
		Username: b.Username, From: b.From, CACert: b.CACert, SiteURL: b.SiteURL,
	}}
}

// smtpConfigJSON is the response: the form without the password. passwordSet says whether one
// is stored; secretsUnreadable that a stored secret cannot be decrypted with the current master
// key (enter it again); configured that mail can be sent.
type smtpConfigJSON struct {
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
	UpdatedAt         string `json:"updatedAt,omitempty"`
}

func toSMTPConfigJSON(v domain.SMTPConfigView) smtpConfigJSON {
	out := smtpConfigJSON{
		Enabled: v.Enabled, Host: v.Host, Port: v.Port, Security: v.Security, AllowPlaintext: v.AllowPlaintext,
		Username: v.Username, PasswordSet: v.PasswordSet, From: v.From, CACert: v.CACert, SiteURL: v.SiteURL,
		SecretsUnreadable: v.SecretsUnreadable, Configured: v.Configured,
	}
	if v.Saved {
		out.UpdatedAt = formatTime(v.UpdatedAt)
	}
	return out
}

func (s *server) getSMTPConfig(w http.ResponseWriter, r *http.Request) {
	v, err := s.svc.GetSMTPConfig(r.Context(), actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSMTPConfigJSON(v))
}

func (s *server) putSMTPConfig(w http.ResponseWriter, r *http.Request) {
	var body smtpConfigBody
	if !decode(w, r, &body) {
		return
	}
	v, err := s.svc.UpdateSMTPConfig(r.Context(), actorOf(r), body.input())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toSMTPConfigJSON(v))
}

// mailTestJSON is the answer of "send test email": ok, or ok=false with the failure code (shown
// as the message error.<code>) and the server's own words. A failed delivery is a 200.
type mailTestJSON struct {
	OK     bool   `json:"ok"`
	Code   string `json:"code,omitempty"`
	Detail string `json:"detail,omitempty"`
	To     string `json:"to"`
}

func (s *server) testSMTPConfig(w http.ResponseWriter, r *http.Request) {
	var body smtpConfigBody
	if !decode(w, r, &body) {
		return
	}
	res, err := s.svc.TestSMTPConfig(r.Context(), actorOf(r), body.input())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mailTestJSON{OK: res.OK, Code: res.Code, Detail: res.Detail, To: res.To})
}

// outboxItemJSON is one mail of the queue as the Email page shows it: what and to whom, never
// the content (links and codes in it are one-time secrets).
type outboxItemJSON struct {
	ID            string `json:"id"`
	To            string `json:"to"`
	Template      string `json:"template"`
	Subject       string `json:"subject"`
	Status        string `json:"status"`
	Attempts      int    `json:"attempts"`
	LastErrorCode string `json:"lastErrorCode,omitempty"`
	LastError     string `json:"lastError,omitempty"`
	CreatedAt     string `json:"createdAt"`
	NextAttemptAt string `json:"nextAttemptAt,omitempty"` // pending only
	SentAt        string `json:"sentAt,omitempty"`
	FailedAt      string `json:"failedAt,omitempty"`
}

type outboxJSON struct {
	Items  []outboxItemJSON `json:"items"`
	Counts struct {
		Pending int `json:"pending"`
		Sent    int `json:"sent"`
		Failed  int `json:"failed"`
	} `json:"counts"`
}

// GET /api/mail-outbox?status=pending|sent|failed: the newest mail of the queue, newest first.
func (s *server) mailOutbox(w http.ResponseWriter, r *http.Request) {
	box, err := s.svc.ListMailOutbox(r.Context(), actorOf(r), r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := outboxJSON{Items: []outboxItemJSON{}}
	out.Counts.Pending, out.Counts.Sent, out.Counts.Failed = box.Pending, box.Sent, box.Failed
	for _, e := range box.Items {
		it := outboxItemJSON{
			ID: strconv.FormatInt(e.ID, 10), To: e.To, Template: e.Template, Subject: e.Subject, Status: string(e.Status),
			Attempts: e.Attempts, LastErrorCode: e.LastErrorCode, LastError: e.LastError, CreatedAt: formatTime(e.CreatedAt),
		}
		if e.Status == domain.MailPending {
			it.NextAttemptAt = formatTime(e.NextAttemptAt)
		}
		if !e.SentAt.IsZero() {
			it.SentAt = formatTime(e.SentAt)
		}
		if !e.FailedAt.IsZero() {
			it.FailedAt = formatTime(e.FailedAt)
		}
		out.Items = append(out.Items, it)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) mailStatus(w http.ResponseWriter, r *http.Request) {
	configured, err := s.svc.MailConfigured(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"configured": configured})
}

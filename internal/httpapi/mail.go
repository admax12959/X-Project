package httpapi

import (
	"net/http"
	"strconv"

	"github.com/admax12959/X-Project/internal/domain"
)

// Mail (#20). The configuration is a system configuration section (system_config.go:
// administrators only, config.manage; the password is write-only). Whether mail is configured may be asked by any
// signed-in user.

func (s *server) mailRoutes() {
	s.route("GET /api/mail/status", s.mailStatus)
	s.route("GET /api/mail-outbox", s.mailOutbox)
}

// smtpConfigJSON is the response: the form without the password. passwordSet says whether one
// is stored; configured says that mail can be sent (the other sections: that the section was
// ever saved).
type smtpConfigJSON struct {
	domain.SMTPSettings
	PasswordSet bool `json:"passwordSet"`
	configStatusJSON
}

func toSMTPConfigJSON(v domain.SMTPConfigView) any {
	return smtpConfigJSON{
		SMTPSettings: v.SMTPSettings, PasswordSet: v.PasswordSet,
		configStatusJSON: newConfigStatus(v.SecretsUnreadable, v.Configured, v.Saved, v.UpdatedAt),
	}
}

// mailTestJSON is the answer of "send test email": ok, or ok=false with the failure code (shown
// as the message error.<code>) and the server's own words. A failed delivery is a 200.
type mailTestJSON struct {
	OK     bool   `json:"ok"`
	Code   string `json:"code,omitempty"`
	Detail string `json:"detail,omitempty"`
	To     string `json:"to"`
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

package fakes

import (
	"context"
	"sync"

	"github.com/admax12959/X-Project/internal/domain"
)

// SentMail is one message the fake mailer was asked to deliver, with the SMTP configuration
// (password decrypted) the sender was built with.
type SentMail struct {
	Config  domain.SMTPConfig
	Message domain.MailMessage
}

// Mailer is the fake of domain.MailSenderFactory (and of the senders it builds): no network, no
// mail server. It records every configuration it was asked to build a sender for (the saved
// one when the queue runs, the unsaved form on "send test email") and every delivered message,
// and can be told to fail.
type Mailer struct {
	mu       sync.Mutex
	configs  []domain.SMTPConfig
	sent     []SentMail
	attempts int
	buildErr error
	failNext int   // number of upcoming Send calls that fail
	failErr  error // the failure; failNext < 0 means every call
}

var _ domain.MailSenderFactory = (*Mailer)(nil)

func NewMailer() *Mailer { return &Mailer{} }

func (m *Mailer) Build(cfg domain.SMTPConfig) (domain.MailSender, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs = append(m.configs, cfg)
	if m.buildErr != nil {
		return nil, m.buildErr
	}
	return &mailSender{m: m, cfg: cfg}, nil
}

type mailSender struct {
	m   *Mailer
	cfg domain.SMTPConfig
}

func (s *mailSender) Send(_ context.Context, msg domain.MailMessage) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	s.m.attempts++
	if s.m.failNext != 0 {
		if s.m.failNext > 0 {
			s.m.failNext--
		}
		return s.m.failErr
	}
	s.m.sent = append(s.m.sent, SentMail{Config: s.cfg, Message: msg})
	return nil
}

// FailWith makes every delivery fail with err (nil = deliveries work again), like a mail server
// that is down or refuses the account.
func (m *Mailer) FailWith(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failErr = err
	m.failNext = 0
	if err != nil {
		m.failNext = -1
	}
}

// FailNext makes the next n deliveries fail with err; later ones work.
func (m *Mailer) FailNext(n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failErr, m.failNext = err, n
}

// FailBuild makes Build return err, like an adapter refusing a configuration.
func (m *Mailer) FailBuild(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buildErr = err
}

// Sent returns the delivered messages, oldest first.
func (m *Mailer) Sent() []SentMail {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]SentMail(nil), m.sent...)
}

// SentTo returns the delivered messages for one recipient address.
func (m *Mailer) SentTo(address string) []SentMail {
	var out []SentMail
	for _, s := range m.Sent() {
		if s.Message.To == address {
			out = append(out, s)
		}
	}
	return out
}

// Attempts is the number of delivery attempts so far, successful or not.
func (m *Mailer) Attempts() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempts
}

// Configs returns every configuration a sender was built for, oldest first.
func (m *Mailer) Configs() []domain.SMTPConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.SMTPConfig(nil), m.configs...)
}

// LastConfig returns the configuration of the latest Build call.
func (m *Mailer) LastConfig() (domain.SMTPConfig, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.configs) == 0 {
		return domain.SMTPConfig{}, false
	}
	return m.configs[len(m.configs)-1], true
}

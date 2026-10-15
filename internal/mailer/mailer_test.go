package mailer_test

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
	"github.com/admax12959/X-Project/internal/mailer"
)

// The real SMTP adapter is tested against a small in-process SMTP server (no mail server, no
// network): implicit TLS, STARTTLS, no encryption, authentication, certificate verification
// with and without the custom CA, and the failures an administrator has to be able to tell apart.

type pki struct {
	caPEM string
	cert  tls.Certificate
}

func newPKI(t *testing.T) pki {
	t.Helper()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test Corp CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, _ := x509.ParseCertificate(caDER)
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "smtp.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	return pki{
		caPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})),
		cert:  tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey},
	}
}

type serverOpts struct {
	implicitTLS bool   // the connection is TLS from the first byte
	startTLS    bool   // offers STARTTLS
	user, pass  string // when set, the server requires AUTH PLAIN with these
	rejectRcpt  bool
}

type stubServer struct {
	t    *testing.T
	ln   net.Listener
	opts serverOpts
	cfg  *tls.Config

	mu       sync.Mutex
	messages []string // raw DATA of delivered messages
	plainPW  bool     // an AUTH command arrived before the connection was encrypted
}

func startServer(t *testing.T, p pki, o serverOpts) *stubServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &stubServer{t: t, ln: ln, opts: o, cfg: &tls.Config{Certificates: []tls.Certificate{p.cert}, MinVersion: tls.VersionTLS12}}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return s
}

func (s *stubServer) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *stubServer) delivered() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.messages...)
}

func (s *stubServer) serve(raw net.Conn) {
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(10 * time.Second))
	var conn net.Conn = raw
	encrypted := false
	if s.opts.implicitTLS {
		conn, encrypted = tls.Server(raw, s.cfg), true
	}
	r, w := bufio.NewReader(conn), func(line string) { _, _ = io.WriteString(conn, line+"\r\n") }
	w("220 stub ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		verb := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(verb, "EHLO"), strings.HasPrefix(verb, "HELO"):
			w("250-stub")
			if s.opts.startTLS && !encrypted {
				w("250-STARTTLS")
			}
			if s.opts.user != "" {
				w("250-AUTH PLAIN")
			}
			w("250 8BITMIME")
		case verb == "STARTTLS":
			w("220 ready to start TLS")
			conn = tls.Server(raw, s.cfg)
			r, encrypted = bufio.NewReader(conn), true
			w = func(line string) { _, _ = io.WriteString(conn, line+"\r\n") }
		case strings.HasPrefix(verb, "AUTH PLAIN"):
			if !encrypted {
				s.mu.Lock()
				s.plainPW = true
				s.mu.Unlock()
			}
			b, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(line[len("AUTH PLAIN"):]))
			if string(b) == "\x00"+s.opts.user+"\x00"+s.opts.pass {
				w("235 2.7.0 accepted")
			} else {
				w("535 5.7.8 Username and Password not accepted")
			}
		case strings.HasPrefix(verb, "MAIL FROM:"):
			w("250 ok")
		case strings.HasPrefix(verb, "RCPT TO:"):
			if s.opts.rejectRcpt {
				w("550 5.1.1 mailbox unavailable")
			} else {
				w("250 ok")
			}
		case verb == "DATA":
			w("354 end with <CRLF>.<CRLF>")
			var data strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				data.WriteString(strings.TrimPrefix(l, "."))
			}
			s.mu.Lock()
			s.messages = append(s.messages, data.String())
			s.mu.Unlock()
			w("250 queued")
		case verb == "QUIT":
			w("221 bye")
			return
		default:
			w("502 not implemented")
		}
	}
}

func cfgFor(s *stubServer, over func(*domain.SMTPConfig)) domain.SMTPConfig {
	c := domain.SMTPConfig{
		Host: "127.0.0.1", Port: s.port(), Security: domain.MailSecurityStartTLS,
		From: "X-Project <noreply@corp.example>", Timeout: 5 * time.Second,
	}
	if over != nil {
		over(&c)
	}
	return c
}

var msg = domain.MailMessage{
	From: "X-Project <noreply@corp.example>", To: "bob@corp.example",
	Subject: "邀请你加入 X-Project", Text: "你好 Bob,\n链接: https://xp.corp.example/invite/tok\n", HTML: "<html><body><p>你好 Bob</p></body></html>",
}

func send(t *testing.T, cfg domain.SMTPConfig) error {
	t.Helper()
	s, err := mailer.Factory{}.Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return s.Send(t.Context(), msg)
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var se *domain.MailSendError
	if !errors.As(err, &se) {
		t.Fatalf("error = %v, want a *domain.MailSendError", err)
	}
	if se.Detail == "" {
		t.Errorf("error %q has no detail", se.Code)
	}
	return se.Code
}

// checkMessage parses what the server received and checks both alternatives and the headers.
func checkMessage(t *testing.T, raw string) {
	t.Helper()
	m, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("not a mail message: %v\n%s", err, raw)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil || subject != msg.Subject {
		t.Errorf("subject = %q (%v), want %q", subject, err, msg.Subject)
	}
	if got := m.Header.Get("To"); !strings.Contains(got, "bob@corp.example") {
		t.Errorf("To = %q", got)
	}
	if got := m.Header.Get("From"); !strings.Contains(got, "noreply@corp.example") {
		t.Errorf("From = %q", got)
	}
	if m.Header.Get("Message-ID") == "" || m.Header.Get("Date") == "" {
		t.Errorf("missing Message-ID or Date: %v", m.Header)
	}
	mt, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" {
		t.Fatalf("content type = %q (%v)", mt, err)
	}
	mr := multipart.NewReader(m.Body, params["boundary"])
	var types, bodies []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(p)
		ct, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		types, bodies = append(types, ct), append(bodies, string(b))
	}
	if len(types) != 2 || types[0] != "text/plain" || types[1] != "text/html" {
		t.Fatalf("parts = %v, want text/plain then text/html", types)
	}
	if !strings.Contains(bodies[0], "你好 Bob") || !strings.Contains(bodies[0], "https://xp.corp.example/invite/tok") || !strings.Contains(bodies[1], "<p>你好 Bob</p>") {
		t.Errorf("bodies = %q", bodies)
	}
}

func TestDeliversOverImplicitTLSTrustingTheCustomCA(t *testing.T) {
	t.Parallel()
	p := newPKI(t)
	s := startServer(t, p, serverOpts{implicitTLS: true, user: "mailer", pass: "pw-1"})

	err := send(t, cfgFor(s, func(c *domain.SMTPConfig) {
		c.Security, c.CACert, c.Username, c.Password = domain.MailSecurityTLS, p.caPEM, "mailer", "pw-1"
	}))

	if err != nil {
		t.Fatal(err)
	}
	if got := s.delivered(); len(got) != 1 {
		t.Fatalf("delivered = %d", len(got))
	} else {
		checkMessage(t, got[0])
	}
}

func TestDeliversOverSTARTTLSAndSendsThePasswordOnlyAfterEncryption(t *testing.T) {
	t.Parallel()
	p := newPKI(t)
	s := startServer(t, p, serverOpts{startTLS: true, user: "mailer", pass: "pw-1"})

	err := send(t, cfgFor(s, func(c *domain.SMTPConfig) { c.CACert, c.Username, c.Password = p.caPEM, "mailer", "pw-1" }))

	if err != nil {
		t.Fatal(err)
	}
	if got := s.delivered(); len(got) != 1 {
		t.Fatalf("delivered = %d", len(got))
	} else {
		checkMessage(t, got[0])
	}
	if s.plainPW {
		t.Fatal("the password was sent before the connection was encrypted")
	}
}

func TestTheServerCertificateIsAlwaysVerified(t *testing.T) {
	t.Parallel()
	p := newPKI(t)
	for name, security := range map[string]string{"implicit TLS": domain.MailSecurityTLS, "STARTTLS": domain.MailSecurityStartTLS} {
		s := startServer(t, p, serverOpts{implicitTLS: security == domain.MailSecurityTLS, startTLS: true, user: "mailer", pass: "pw-1"})

		// The CA is not trusted (no custom CA given): refuse, send nothing, never the password.
		err := send(t, cfgFor(s, func(c *domain.SMTPConfig) { c.Security, c.Username, c.Password = security, "mailer", "pw-1" }))

		if got := codeOf(t, err); got != domain.CodeSmtpTLSFailed {
			t.Errorf("%s: code = %s, want %s", name, got, domain.CodeSmtpTLSFailed)
		}
		if len(s.delivered()) != 0 || s.plainPW {
			t.Errorf("%s: mail or password went to a server whose certificate is not trusted", name)
		}
	}
}

func TestSTARTTLSIsRequiredNotOptional(t *testing.T) {
	t.Parallel()
	p := newPKI(t)
	s := startServer(t, p, serverOpts{startTLS: false, user: "mailer", pass: "pw-1"})

	err := send(t, cfgFor(s, func(c *domain.SMTPConfig) { c.CACert, c.Username, c.Password = p.caPEM, "mailer", "pw-1" }))

	if got := codeOf(t, err); got != domain.CodeSmtpStartTLSMissing {
		t.Fatalf("code = %s, want %s", got, domain.CodeSmtpStartTLSMissing)
	}
	if len(s.delivered()) != 0 || s.plainPW {
		t.Fatal("the adapter fell back to plaintext")
	}
}

func TestUnencryptedDeliveryWorksWhenChosenExplicitly(t *testing.T) {
	t.Parallel()
	s := startServer(t, newPKI(t), serverOpts{user: "mailer", pass: "pw-1"})

	err := send(t, cfgFor(s, func(c *domain.SMTPConfig) { c.Security, c.Username, c.Password = domain.MailSecurityNone, "mailer", "pw-1" }))

	if err != nil {
		t.Fatal(err)
	}
	if got := s.delivered(); len(got) != 1 {
		t.Fatalf("delivered = %d", len(got))
	} else {
		checkMessage(t, got[0])
	}
}

func TestFailuresAreToldApartWithTheServersOwnWords(t *testing.T) {
	t.Parallel()
	p := newPKI(t)
	secure := func(c *domain.SMTPConfig) { c.CACert = p.caPEM }

	t.Run("wrong password", func(t *testing.T) {
		s := startServer(t, p, serverOpts{startTLS: true, user: "mailer", pass: "pw-1"})
		err := send(t, cfgFor(s, func(c *domain.SMTPConfig) { secure(c); c.Username, c.Password = "mailer", "wrong" }))
		if got := codeOf(t, err); got != domain.CodeSmtpAuthFailed || !strings.Contains(err.Error(), "535 5.7.8 Username and Password not accepted") {
			t.Fatalf("error = %v (code %s)", err, got)
		}
	})
	t.Run("recipient refused", func(t *testing.T) {
		s := startServer(t, p, serverOpts{startTLS: true, rejectRcpt: true})
		err := send(t, cfgFor(s, secure))
		if got := codeOf(t, err); got != domain.CodeSmtpRecipientRejected || !strings.Contains(err.Error(), "550 5.1.1") {
			t.Fatalf("error = %v (code %s)", err, got)
		}
	})
	t.Run("nobody listening", func(t *testing.T) {
		s := startServer(t, p, serverOpts{})
		port := s.port()
		s.ln.Close()
		err := send(t, cfgFor(s, func(c *domain.SMTPConfig) { secure(c); c.Port = port }))
		if got := codeOf(t, err); got != domain.CodeSmtpUnreachable {
			t.Fatalf("code = %s, want %s", got, domain.CodeSmtpUnreachable)
		}
	})
	t.Run("account configured but the server offers no authentication", func(t *testing.T) {
		s := startServer(t, p, serverOpts{startTLS: true})
		err := send(t, cfgFor(s, func(c *domain.SMTPConfig) { secure(c); c.Username, c.Password = "mailer", "pw-1" }))
		if got := codeOf(t, err); got != domain.CodeSmtpAuthFailed {
			t.Fatalf("code = %s, want %s", got, domain.CodeSmtpAuthFailed)
		}
	})
}

func TestBuildRefusesAConfigurationThatCannotWork(t *testing.T) {
	t.Parallel()
	good := domain.SMTPConfig{Host: "smtp.corp.example", Port: 587, Security: domain.MailSecurityStartTLS, From: "noreply@corp.example"}
	for name, mutate := range map[string]func(*domain.SMTPConfig){
		"no host":          func(c *domain.SMTPConfig) { c.Host = "" },
		"bad port":         func(c *domain.SMTPConfig) { c.Port = 70000 },
		"unknown security": func(c *domain.SMTPConfig) { c.Security = "ssl3" },
		"bad sender":       func(c *domain.SMTPConfig) { c.From = "nope" },
		"CA without certs": func(c *domain.SMTPConfig) { c.CACert = "-----BEGIN CERTIFICATE-----\nnope\n-----END CERTIFICATE-----" },
	} {
		cfg := good
		mutate(&cfg)
		if _, err := (mailer.Factory{}).Build(cfg); err == nil {
			t.Errorf("%s: Build accepted %+v", name, cfg)
		}
	}
	if _, err := (mailer.Factory{}).Build(good); err != nil {
		t.Errorf("Build refused a good configuration: %v", err)
	}
}

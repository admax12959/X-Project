package domain

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
)

// Mail templates (#20). They are packaged in the program: internal/domain/mailtemplates/<lang>/
// <name>.txt and <name>.html, one pair per template and language ("zh" and "en"). The .txt
// file starts with a "Subject: ..." line, then a blank line, then the plain-text body; the
// .html file is the HTML alternative of the same message. Both are Go templates over the same
// data: .Name (the recipient's display name, may be empty), .Link (when the request has a
// link) and whatever the request's Data carries (for example .Code). HTML is escaped
// automatically. A template that uses a value the request did not provide fails when the
// mail is created, not when it is sent.
//
// To add a template: add a MailTemplate constant to mailTemplates below, then the four files
// (zh and en, .txt and .html). Every template in the list is rendered in both languages by the
// API tests, so a missing file or a broken template fails the build.

//go:embed mailtemplates
var mailTemplateFS embed.FS

// MailTemplate names a bundled mail template.
type MailTemplate string

const (
	// MailTemplateTest is the "send test email" message. Data: none.
	MailTemplateTest MailTemplate = "test"
	// MailTemplateInvite invites a user to set a password. Data: a link (MailRequest.LinkPath).
	MailTemplateInvite MailTemplate = "invite"
	// MailTemplatePasswordReset lets a user choose a new password. Data: a link.
	MailTemplatePasswordReset MailTemplate = "passwordReset"
	// MailTemplateVerifyCode carries a one-time code that proves the recipient owns the address.
	// Data: "Code".
	MailTemplateVerifyCode MailTemplate = "verifyCode"
	// MailTemplateLoginEmailChanged tells the old address that the login email was changed.
	// Data: "OldEmail", "NewEmail".
	MailTemplateLoginEmailChanged MailTemplate = "loginEmailChanged"
	// MailTemplateIdentityBound / MailTemplateIdentityUnbound tell the contact email that a
	// login identity was added or removed. Data: "Kind" ("ldap" or "email"), "LoginName".
	MailTemplateIdentityBound   MailTemplate = "identityBound"
	MailTemplateIdentityUnbound MailTemplate = "identityUnbound"
)

// mailTemplates is every bundled template.
var mailTemplates = []MailTemplate{MailTemplateTest, MailTemplateInvite, MailTemplatePasswordReset, MailTemplateVerifyCode,
	MailTemplateLoginEmailChanged, MailTemplateIdentityBound, MailTemplateIdentityUnbound}

// MailTemplates lists the bundled templates.
func MailTemplates() []MailTemplate { return append([]MailTemplate(nil), mailTemplates...) }

// mailLanguages are the languages every template exists in.
var mailLanguages = []string{LanguageZh, LanguageEn}

// defaultMailLanguage is used for a recipient who never chose a language.
const defaultMailLanguage = LanguageZh

// mailContent is a rendered message.
type mailContent struct {
	Subject string
	Text    string
	HTML    string
}

// mailLanguageOf maps a language preference (empty = never chosen) to a template language.
func mailLanguageOf(pref string) string {
	if pref == LanguageEn || pref == LanguageZh {
		return pref
	}
	return defaultMailLanguage
}

func renderMail(name MailTemplate, lang string, data map[string]any) (mailContent, error) {
	read := func(ext string) (string, error) {
		b, err := mailTemplateFS.ReadFile(fmt.Sprintf("mailtemplates/%s/%s.%s", lang, name, ext))
		return string(b), err
	}
	txt, err := read("txt")
	if err != nil {
		return mailContent{}, fmt.Errorf("mail template %s/%s: %w", lang, name, err)
	}
	html, err := read("html")
	if err != nil {
		return mailContent{}, fmt.Errorf("mail template %s/%s: %w", lang, name, err)
	}

	tt, err := texttemplate.New("txt").Option("missingkey=error").Parse(txt)
	if err != nil {
		return mailContent{}, fmt.Errorf("mail template %s/%s.txt: %w", lang, name, err)
	}
	var tb bytes.Buffer
	if err := tt.Execute(&tb, data); err != nil {
		return mailContent{}, fmt.Errorf("mail template %s/%s.txt: %w", lang, name, err)
	}
	head, body, ok := strings.Cut(strings.ReplaceAll(tb.String(), "\r\n", "\n"), "\n\n")
	subject, found := strings.CutPrefix(head, "Subject: ")
	if !ok || !found || strings.ContainsAny(subject, "\r\n") || strings.TrimSpace(subject) == "" {
		return mailContent{}, fmt.Errorf("mail template %s/%s.txt: want a single \"Subject: ...\" line, a blank line and the body", lang, name)
	}

	ht, err := htmltemplate.New("html").Option("missingkey=error").Parse(html)
	if err != nil {
		return mailContent{}, fmt.Errorf("mail template %s/%s.html: %w", lang, name, err)
	}
	var hb bytes.Buffer
	if err := ht.Execute(&hb, data); err != nil {
		return mailContent{}, fmt.Errorf("mail template %s/%s.html: %w", lang, name, err)
	}
	return mailContent{Subject: strings.TrimSpace(subject), Text: strings.TrimSpace(body) + "\n", HTML: hb.String()}, nil
}

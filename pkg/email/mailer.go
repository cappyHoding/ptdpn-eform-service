// Package email provides a simple SMTP email sender for outbound notifications.
// Uses net/smtp from standard library — no external dependency.
package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
)

// Mailer handles sending emails via SMTP.
type Mailer struct {
	host      string
	port      int
	username  string
	password  string
	fromName  string
	fromEmail string
}

// New creates a new Mailer with the given SMTP configuration.
func New(host string, port int, username, password, fromName, fromEmail string) *Mailer {
	return &Mailer{
		host:      host,
		port:      port,
		username:  username,
		password:  password,
		fromName:  fromName,
		fromEmail: fromEmail,
	}
}

// Message represents an outbound email.
type Message struct {
	To      string // recipient email address
	Subject string
	Body    string // HTML body
}

// Send sends a single email via SMTP.
// Returns the provider message ID (empty for SMTP) or an error.
func (m *Mailer) Send(msg Message) error {
	auth := smtp.PlainAuth("", m.username, m.password, m.host)

	from := fmt.Sprintf("%s <%s>", m.fromName, m.fromEmail)

	headers := strings.Join([]string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", msg.To),
		fmt.Sprintf("Subject: %s", msg.Subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
	}, "\r\n")

	body := headers + "\r\n\r\n" + msg.Body

	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	return smtp.SendMail(addr, auth, m.fromEmail, []string{msg.To}, []byte(body))
}

// ─── Template helpers ─────────────────────────────────────────────────────────

// RenderHTML renders an HTML template string with data.
func RenderHTML(tmpl string, data any) (string, error) {
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("template parse failed: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template render failed: %w", err)
	}
	return buf.String(), nil
}

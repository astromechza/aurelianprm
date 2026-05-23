// Package mailer provides SMTP email sending for AurelianPRM.
package mailer

import (
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/astromechza/aurelianprm/internal/config"
)

// Send delivers a plain-text email via the configured SMTP server.
// It uses STARTTLS when credentials are provided (port 587 / production).
// When Username is empty, it connects without authentication — suitable for
// local dev relays such as MailHog.
func Send(cfg config.SMTP, to, subject, body string) error {
	return send(cfg, to, subject, buildMessage(cfg.From, to, subject, body, "text/plain"))
}

// SendHTML delivers an HTML email via the configured SMTP server.
func SendHTML(cfg config.SMTP, to, subject, body string) error {
	return send(cfg, to, subject, buildMessage(cfg.From, to, subject, body, "text/html"))
}

func send(cfg config.SMTP, to, subject, msg string) error {
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	if cfg.Username == "" {
		return smtp.SendMail(addr, nil, cfg.From, []string{to}, []byte(msg))
	}
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	return smtp.SendMail(addr, auth, cfg.From, []string{to}, []byte(msg))
}

// buildMessage constructs a minimal RFC 2822 message with the given content type.
func buildMessage(from, to, subject, body, contentType string) string {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: " + contentType + "; charset=utf-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}

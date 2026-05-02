package digest

import (
	"fmt"

	"github.com/wneessen/go-mail"
)

// newSMTPSender returns a senderFunc that delivers mail via SMTP using go-mail.
// Set cfg.SMTPSSL=true for implicit TLS on port 465 (Fastmail recommended).
// Default uses STARTTLS mandatory on port 587.
func newSMTPSender(cfg Config) senderFunc {
	return func(subject, body, from, to string) error {
		msg := mail.NewMsg()
		if err := msg.From(from); err != nil {
			return fmt.Errorf("set from address: %w", err)
		}
		if err := msg.To(to); err != nil {
			return fmt.Errorf("set to address: %w", err)
		}
		msg.Subject(subject)
		msg.SetBodyString(mail.TypeTextPlain, body)

		port := cfg.SMTPPort
		if port == 0 {
			port = 587
		}

		opts := []mail.Option{
			mail.WithPort(port),
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.SMTPUser),
			mail.WithPassword(cfg.SMTPPass),
		}
		if cfg.SMTPSSL {
			opts = append(opts, mail.WithSSL())
		} else {
			opts = append(opts, mail.WithTLSPortPolicy(mail.TLSMandatory))
		}

		client, err := mail.NewClient(cfg.SMTPHost, opts...)
		if err != nil {
			return fmt.Errorf("create mail client: %w", err)
		}
		if err := client.DialAndSend(msg); err != nil {
			return fmt.Errorf("send mail: %w", err)
		}
		return nil
	}
}

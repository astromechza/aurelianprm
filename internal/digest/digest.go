package digest

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/astromechza/aurelianprm/internal/dal"
)

// Reminder is a single upcoming event to include in the digest email.
type Reminder struct {
	Date     time.Time
	Label    string
	PersonID string // entity ID; non-empty enables a hyperlink in the HTML email
}

// Config holds SMTP and recipient settings for the digest mailer.
// SMTPHost, SMTPUser, SMTPPass, SMTPFrom, and DigestTo are required.
// SMTPPort defaults to 587 if zero. ExternalURL is optional; when set,
// person reminders include a hyperlink to the person's detail page.
type Config struct {
	SMTPHost    string
	SMTPPort    int // default 587
	SMTPUser    string
	SMTPPass    string
	SMTPFrom    string
	SMTPSSL     bool // true = implicit TLS (port 465); false = STARTTLS mandatory (port 587)
	DigestTo    string
	ExternalURL string // optional base URL for person links (e.g. https://aurelianprm.example.com)
	Force       bool   // if true, send even when there are no reminders
}

// senderFunc delivers a composed message. Injected in tests to avoid live SMTP.
type senderFunc func(subject, body, from, to string) error

// FindReminders aggregates all reminder sources for the window [now, now+6] and
// returns results sorted by date ascending, then label ascending.
func FindReminders(persons []dal.Entity, now time.Time) []Reminder {
	all := make([]Reminder, 0, len(persons))
	all = append(all, findBirthdayReminders(persons, now)...)
	// Future sources: all = append(all, findNoteReminders(notes, now)...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].Date.Equal(all[j].Date) {
			return all[i].Label < all[j].Label
		}
		return all[i].Date.Before(all[j].Date)
	})
	return all
}

// templateReminder is the per-row data passed to the HTML email template.
type templateReminder struct {
	DateStr string
	Label   string
	URL     string // empty → render label without link
}

var digestTemplate = template.Must(template.New("digest").Parse(`<!DOCTYPE html>
<html>
<body>
<p>Upcoming reminders in the next 7 days:</p>
<ul>
{{range .}}<li>{{.DateStr}} &mdash; {{if .URL}}<a href="{{.URL}}">{{.Label}}</a>{{else}}{{.Label}}{{end}}</li>
{{end}}</ul>
</body>
</html>
`))

// composeHTMLBody formats the HTML body for the digest email.
// externalURL is the base URL used to build person links; empty disables linking.
func composeHTMLBody(reminders []Reminder, externalURL string) string {
	base := strings.TrimRight(externalURL, "/")
	items := make([]templateReminder, 0, len(reminders))
	for _, r := range reminders {
		tr := templateReminder{DateStr: r.Date.Format("Mon 2 Jan"), Label: r.Label}
		if r.PersonID != "" && base != "" {
			tr.URL = base + "/persons/" + r.PersonID
		}
		items = append(items, tr)
	}
	var buf bytes.Buffer
	if err := digestTemplate.Execute(&buf, items); err != nil {
		// Template execution should never fail with valid data; fall back to label list.
		for _, r := range reminders {
			fmt.Fprintf(&buf, "%s — %s\n", r.Date.Format("Mon 2 Jan"), r.Label)
		}
	}
	return buf.String()
}

// sendDigest is the internal entry point; send is injected for testability.
func sendDigest(ctx context.Context, d *dal.DAL, cfg Config, now time.Time, send senderFunc) error {
	var persons []dal.Entity
	if err := d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		persons, err = q.ListEntitiesByType(ctx, "Person")
		return err
	}); err != nil {
		return fmt.Errorf("load persons: %w", err)
	}

	reminders := FindReminders(persons, now)
	slog.Info("reminders found", "count", len(reminders))
	if len(reminders) == 0 && !cfg.Force {
		slog.Info("no reminders and --force not set, skipping email")
		return nil
	}

	subject := fmt.Sprintf("Reminders for %s", now.Format("Mon 2 Jan 2006"))
	body := composeHTMLBody(reminders, cfg.ExternalURL)
	slog.Info("sending digest email", "to", cfg.DigestTo, "subject", subject)
	if err := send(subject, body, cfg.SMTPFrom, cfg.DigestTo); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	slog.Info("digest email sent successfully", "to", cfg.DigestTo)
	return nil
}

// SendDigest is the public entry point. Loads persons from the database, finds
// upcoming reminders, and sends an HTML digest email. No email is sent if
// no reminders are found.
func SendDigest(ctx context.Context, d *dal.DAL, cfg Config) error {
	return sendDigest(ctx, d, cfg, time.Now(), newSMTPSender(cfg))
}

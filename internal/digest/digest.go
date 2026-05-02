package digest

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/astromechza/aurelianprm/internal/dal"
)

// Reminder is a single upcoming event to include in the digest email.
type Reminder struct {
	Date  time.Time
	Label string
}

// Config holds SMTP and recipient settings for the digest mailer.
// All string fields except SMTPSSL are required; SMTPPort defaults to 587 if zero.
type Config struct {
	SMTPHost string
	SMTPPort int // default 587
	SMTPUser string
	SMTPPass string
	SMTPFrom string
	SMTPSSL  bool // true = implicit TLS (port 465); false = STARTTLS mandatory (port 587)
	DigestTo string
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

// composeBody formats the plain-text body for the digest email.
func composeBody(reminders []Reminder) string {
	var sb strings.Builder
	sb.WriteString("Upcoming reminders in the next 7 days:\n\n")
	for _, r := range reminders {
		fmt.Fprintf(&sb, "• %s — %s\n", r.Date.Format("Mon 2 Jan"), r.Label)
	}
	return sb.String()
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
	if len(reminders) == 0 {
		return nil // nothing to send
	}

	subject := fmt.Sprintf("Reminders for %s", now.Format("Mon 2 Jan 2006"))
	body := composeBody(reminders)
	return send(subject, body, cfg.SMTPFrom, cfg.DigestTo)
}

// SendDigest is the public entry point. Loads persons from the database, finds
// upcoming reminders, and sends a plain-text digest email. No email is sent if
// no reminders are found.
func SendDigest(ctx context.Context, d *dal.DAL, cfg Config) error {
	return sendDigest(ctx, d, cfg, time.Now(), newSMTPSender(cfg))
}

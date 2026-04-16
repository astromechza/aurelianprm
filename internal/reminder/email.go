package reminder

import (
	"fmt"
	"strings"
	"time"

	"github.com/astromechza/aurelianprm/internal/config"
	"github.com/astromechza/aurelianprm/internal/mailer"
)

// DayReminders groups reminder items that were due on a particular trigger date.
type DayReminders struct {
	Date  time.Time      // the trigger date (midnight in configured timezone)
	Items []Item // items due that day
}

// FormatDigest builds the plain-text body of the reminder digest email.
// sendDate is the actual date the email is being sent (today in the scheduler's
// timezone). days is ordered oldest-first; the last entry is always sendDate.
func FormatDigest(sendDate time.Time, days []DayReminders) string {
	var sb strings.Builder
	sb.WriteString("Upcoming reminders \u2014 AurelianPRM\n\n")

	for _, d := range days {
		isCatchUp := !sameDay(d.Date, sendDate)

		if isCatchUp {
			fmt.Fprintf(&sb, "[Catch-up from %s]\n", d.Date.Format("2 January 2006"))
		} else {
			fmt.Fprintf(&sb, "%s:\n", d.Date.Format("2 January 2006"))
		}

		for _, item := range d.Items {
			rel := relativeDate(sendDate, item.OccursOn)
			fmt.Fprintf(&sb, "\u2022 %s (%s)\n", item.Label, rel)
		}
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// relativeDate returns a human-readable relative date string comparing
// occursOn to sendDate. Both dates should be midnight in the same timezone.
func relativeDate(sendDate, occursOn time.Time) string {
	diff := int(occursOn.Sub(sendDate).Hours() / 24)
	dateStr := occursOn.Format("2 January")
	switch {
	case diff < 0:
		return fmt.Sprintf("was %s", dateStr)
	case diff == 0:
		return fmt.Sprintf("today, %s", dateStr)
	case diff == 1:
		return fmt.Sprintf("tomorrow, %s", dateStr)
	default:
		return fmt.Sprintf("in %d days, %s", diff, dateStr)
	}
}

// sameDay reports whether two times fall on the same calendar day.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// SendDigest sends the formatted digest email via the configured SMTP server.
func SendDigest(cfg config.SMTP, to, subject, body string) error {
	return mailer.Send(cfg, to, subject, body)
}

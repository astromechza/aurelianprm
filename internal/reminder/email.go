package reminder

import (
	"fmt"
	"html"
	"strings"
	"time"
)

// DayReminders groups reminder items that were due on a particular trigger date.
type DayReminders struct {
	Date  time.Time // the trigger date (midnight in configured timezone)
	Items []Item    // items due that day
}

// FormatDigest builds the HTML body of the reminder digest email.
// sendDate is the actual date the email is being sent (today in the scheduler's
// timezone). days is ordered oldest-first; the last entry is always sendDate.
// externalURL is the public base URL (e.g. "https://prm.example.com") used to
// build deep links; pass an empty string to omit links.
func FormatDigest(sendDate time.Time, days []DayReminders, externalURL string) string {
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n<html>\n<head><meta charset=\"utf-8\"></head>\n<body>\n")
	sb.WriteString("<h2>Upcoming reminders \u2014 AurelianPRM</h2>\n")

	for _, d := range days {
		isCatchUp := !sameDay(d.Date, sendDate)
		if isCatchUp {
			fmt.Fprintf(&sb, "<h3>[Catch-up from %s]</h3>\n", html.EscapeString(d.Date.Format("2 January 2006")))
		} else {
			fmt.Fprintf(&sb, "<h3>%s</h3>\n", html.EscapeString(d.Date.Format("2 January 2006")))
		}

		sb.WriteString("<ul>\n")
		for _, item := range d.Items {
			rel := relativeDate(sendDate, item.OccursOn)
			label := html.EscapeString(item.Label)
			relEsc := html.EscapeString(rel)
			if item.PersonID != "" && externalURL != "" {
				url := html.EscapeString(strings.TrimRight(externalURL, "/") + "/people/" + item.PersonID)
				fmt.Fprintf(&sb, "  <li><a href=\"%s\">%s</a> (%s)</li>\n", url, label, relEsc)
			} else {
				fmt.Fprintf(&sb, "  <li>%s (%s)</li>\n", label, relEsc)
			}
		}
		sb.WriteString("</ul>\n")
	}

	sb.WriteString("</body>\n</html>")
	return sb.String()
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


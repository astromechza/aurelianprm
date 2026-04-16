package reminder_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/astromechza/aurelianprm/internal/reminder"
)

func TestFormatDigest_SingleDay(t *testing.T) {
	sendDate := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	days := []reminder.DayReminders{
		{
			Date: sendDate,
			Items: []reminder.Item{
				{Label: "Alice Smith \u2014 birthday", OccursOn: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)},
				{Label: "Bob Jones \u2014 birthday", OccursOn: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)},
			},
		},
	}

	body := reminder.FormatDigest(sendDate, days)

	assert.Contains(t, body, "13 June 2026")
	assert.Contains(t, body, "Alice Smith \u2014 birthday")
	assert.Contains(t, body, "in 2 days")   // June 15 is 2 days after June 13
	assert.Contains(t, body, "Bob Jones \u2014 birthday")
	assert.Contains(t, body, "tomorrow")    // June 14 is tomorrow
	assert.NotContains(t, body, "Catch-up") // single day, no catch-up
}

func TestFormatDigest_CatchUp(t *testing.T) {
	sendDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	missedDay := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)

	days := []reminder.DayReminders{
		{
			Date: missedDay,
			Items: []reminder.Item{
				{Label: "Carol \u2014 birthday", OccursOn: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)},
			},
		},
		{
			Date: sendDate,
			Items: []reminder.Item{
				{Label: "Dave \u2014 birthday", OccursOn: time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)},
			},
		},
	}

	body := reminder.FormatDigest(sendDate, days)

	assert.Contains(t, body, "15 June 2026")
	assert.Contains(t, body, "Dave \u2014 birthday")
	assert.Contains(t, body, "tomorrow")                     // June 16 from send date June 15
	assert.Contains(t, body, "[Catch-up from 14 June 2026]")
	assert.Contains(t, body, "Carol \u2014 birthday")
	assert.Contains(t, body, "was 14 June") // OccursOn < sendDate
}

func TestFormatDigest_TodayEvent(t *testing.T) {
	sendDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	days := []reminder.DayReminders{
		{
			Date: sendDate,
			Items: []reminder.Item{
				{Label: "Eve \u2014 birthday", OccursOn: sendDate},
			},
		},
	}
	body := reminder.FormatDigest(sendDate, days)
	assert.Contains(t, body, "today")
}

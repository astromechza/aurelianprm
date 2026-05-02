package digest

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/astromechza/aurelianprm/internal/dal"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makePerson builds a minimal dal.Entity with the given birth fields.
func makePerson(name string, birthYear, birthMonth, birthDay int) dal.Entity {
	data, _ := json.Marshal(map[string]any{
		"name":       name,
		"birthYear":  birthYear,
		"birthMonth": birthMonth,
		"birthDay":   birthDay,
	})
	return dal.Entity{ID: "test-" + name, Type: "Person", Data: data}
}

func TestFindBirthdayReminders(t *testing.T) {
	// Reference: Monday 27 Apr 2026. Window = Apr 27 – May 3 inclusive.
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	today := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		entity    dal.Entity
		wantCount int
		wantLabel string
		wantDate  time.Time
	}{
		{
			name:      "birthday today (day 0, included)",
			entity:    makePerson("Alice", 1990, 4, 27),
			wantCount: 1,
			wantLabel: "Alice's birthday (turning 36)",
			wantDate:  today,
		},
		{
			name:      "birthday tomorrow (day 1, included)",
			entity:    makePerson("Bob", 0, 4, 28),
			wantCount: 1,
			wantLabel: "Bob's birthday",
			wantDate:  today.AddDate(0, 0, 1),
		},
		{
			name:      "birthday day 6 (last included day)",
			entity:    makePerson("Carol", 0, 5, 3),
			wantCount: 1,
			wantDate:  time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "birthday day 7 (excluded)",
			entity:    makePerson("Dave", 0, 5, 4),
			wantCount: 0,
		},
		{
			name:      "no birthday set (both zero)",
			entity:    makePerson("Eve", 0, 0, 0),
			wantCount: 0,
		},
		{
			name:      "birth month set but day zero",
			entity:    makePerson("Frank", 0, 4, 0),
			wantCount: 0,
		},
		{
			name:      "unknown birth year — no age shown",
			entity:    makePerson("Grace", 0, 4, 28),
			wantCount: 1,
			wantLabel: "Grace's birthday",
		},
		{
			name:      "birthday yesterday (day -1, excluded; next year not in window)",
			entity:    makePerson("Hal", 0, 4, 26),
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findBirthdayReminders([]dal.Entity{tc.entity}, now)
			require.Len(t, got, tc.wantCount)
			if tc.wantCount > 0 {
				if tc.wantLabel != "" {
					assert.Equal(t, tc.wantLabel, got[0].Label)
				}
				if !tc.wantDate.IsZero() {
					assert.Equal(t, tc.wantDate, got[0].Date)
				}
			}
		})
	}
}

func TestFindBirthdayReminders_leapDay(t *testing.T) {
	// Feb 27, 2026 — non-leap year. Person born Feb 29.
	// Go normalises time.Date(2026, 2, 29, ...) → 2026-03-01.
	// Mar 1 is 2 days ahead: falls in window (day 2).
	now := time.Date(2026, 2, 27, 10, 0, 0, 0, time.UTC)
	entity := makePerson("Leaper", 2000, 2, 29)
	got := findBirthdayReminders([]dal.Entity{entity}, now)
	require.Len(t, got, 1)
	assert.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), got[0].Date)
	// Age: 2026 - 2000 = 26 (using normalised year 2026)
	assert.Equal(t, "Leaper's birthday (turning 26)", got[0].Label)
}

func TestFindReminders_sortedByDateThenLabel(t *testing.T) {
	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	persons := []dal.Entity{
		makePerson("Zebra", 0, 5, 1),
		makePerson("Apple", 0, 4, 28),
		makePerson("Mango", 0, 4, 29),
		makePerson("Berry", 0, 4, 29), // same day as Mango, sorts before alphabetically
	}
	got := FindReminders(persons, now)
	require.Len(t, got, 4)
	assert.Equal(t, "Apple's birthday", got[0].Label)
	assert.Equal(t, "Berry's birthday", got[1].Label)
	assert.Equal(t, "Mango's birthday", got[2].Label)
	assert.Equal(t, "Zebra's birthday", got[3].Label)
}

func TestComposeBody(t *testing.T) {
	reminders := []Reminder{
		{Date: time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC), Label: "Alice's birthday"},
		{Date: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Label: "Bob's birthday (turning 30)"},
	}
	body := composeBody(reminders)
	assert.Contains(t, body, "Upcoming reminders in the next 7 days:")
	assert.Contains(t, body, "• Tue 28 Apr — Alice's birthday")
	assert.Contains(t, body, "• Fri 1 May — Bob's birthday (turning 30)")
}

func TestSendDigest_noReminders_doesNotSend(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	d := dal.New(sqlDB, ":memory:")

	sendCalled := false
	err = sendDigest(t.Context(), d, Config{}, time.Now(), func(_, _, _, _ string) error {
		sendCalled = true
		return nil
	})
	require.NoError(t, err)
	assert.False(t, sendCalled, "send must not be called when no reminders")
}

func TestSendDigest_withReminders_sendsEmail(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	d := dal.New(sqlDB, ":memory:")

	now := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)

	require.NoError(t, d.WithTx(t.Context(), func(q *dal.Queries) error {
		data, _ := json.Marshal(map[string]any{
			"name": "Alice", "birthMonth": 4, "birthDay": 27,
		})
		_, err := q.CreateEntity(t.Context(), dal.CreateEntityParams{
			Type: "Person",
			Data: data,
		})
		return err
	}))

	var gotSubject, gotBody, gotTo string
	err = sendDigest(t.Context(), d, Config{SMTPFrom: "from@test.com", DigestTo: "to@test.com"}, now,
		func(subject, body, _ string, to string) error {
			gotSubject = subject
			gotBody = body
			gotTo = to
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, "Reminders for Mon 27 Apr 2026", gotSubject)
	assert.Contains(t, gotBody, "Alice's birthday")
	assert.Equal(t, "to@test.com", gotTo)
}

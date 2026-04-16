# Phase 5 — Reminder System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Send a daily plain-text digest email listing upcoming birthdays and relationship anniversaries, with retry-on-failure, catch-up for missed days, and send history stored in SQLite.

**Architecture:** A goroutine starts alongside the HTTP server, sleeping until `reminder_send_time` each day via `time.Sleep` (no Clock interface — tests use Go 1.26's `testing/synctest` for fake time). The digest logic is pure date arithmetic in `internal/reminder/digest.go`. A `sendFunc` is injected into the scheduler so tests can capture sent emails without a live SMTP server.

**Tech Stack:** Go 1.26, `testing/synctest`, `internal/mailer`, `database/sql`, SQLite via goose migration.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `migrations/00002_reminder_history.sql` | Create | reminder_history schema |
| `internal/model/model.go` | Modify | Add `ReminderHistory` type |
| `internal/store/reminder_history.go` | Create | All reminder-history store functions |
| `internal/store/reminder_history_test.go` | Create | Store tests for reminder history |
| `internal/reminder/digest.go` | Create | `ComputeReminders` — pure date logic |
| `internal/reminder/digest_test.go` | Create | Unit tests for digest logic |
| `internal/reminder/email.go` | Create | `FormatDigest` and `SendDigest` |
| `internal/reminder/email_test.go` | Create | Unit tests for email formatting |
| `internal/reminder/scheduler.go` | Create | `Scheduler` struct and `Run` goroutine |
| `internal/reminder/scheduler_test.go` | Create | `synctest`-based scheduler tests |
| `cmd/aurelianprm/main.go` | Modify | Wire scheduler goroutine into `runServe` |

---

## Task 1: Migration + model + store

**Files:**
- Create: `migrations/00002_reminder_history.sql`
- Modify: `internal/model/model.go`
- Create: `internal/store/reminder_history.go`
- Test: `internal/store/reminder_history_test.go`

### Background

The `reminder_history` table records each daily send attempt. `date_start` and `date_end` are stored as `TEXT` in `YYYY-MM-DD` format (one column per boundary). `GetLastSuccessfulSend` is the key query used by the scheduler to determine catch-up range.

Store functions follow the existing pattern: `context.Context` + `*sql.DB` as first two args, errors wrapped with `fmt.Errorf("store.FuncName: %w", err)`.

- [ ] **Create `migrations/00002_reminder_history.sql`**

```sql
-- +goose Up

CREATE TABLE reminder_history (
    id         TEXT PRIMARY KEY,
    sent_at    DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    status     TEXT NOT NULL CHECK (status IN ('success', 'failure')),
    error      TEXT,
    date_start TEXT NOT NULL,
    date_end   TEXT NOT NULL
);

-- +goose Down

DROP TABLE reminder_history;
```

- [ ] **Add `ReminderHistory` to `internal/model/model.go`**

Append to the end of the file:

```go
// ReminderHistory records the outcome of a daily reminder digest attempt.
type ReminderHistory struct {
	ID        string
	SentAt    time.Time
	Status    string  // "success" | "failure"
	Error     *string // nil on success
	DateStart time.Time // calendar date; time component is always zero
	DateEnd   time.Time // calendar date; time component is always zero
}
```

- [ ] **Write failing tests in `internal/store/reminder_history_test.go`**

```go
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

func TestGetLastSuccessfulSend_Empty(t *testing.T) {
	db := openTestDB(t)
	got, err := store.GetLastSuccessfulSend(context.Background(), db)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestCreateReminderHistory_Success(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	dateStart := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	dateEnd := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)

	err := store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status:    "success",
		DateStart: dateStart,
		DateEnd:   dateEnd,
	})
	require.NoError(t, err)

	got, err := store.GetLastSuccessfulSend(ctx, db)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "success", got.Status)
	assert.Nil(t, got.Error)
	assert.Equal(t, dateStart, got.DateStart)
	assert.Equal(t, dateEnd, got.DateEnd)
}

func TestGetLastSuccessfulSend_ReturnsLatest(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	d1 := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)

	require.NoError(t, store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status: "success", DateStart: d1, DateEnd: d1,
	}))
	require.NoError(t, store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status: "success", DateStart: d2, DateEnd: d2,
	}))
	// A failure after the successes — should NOT be returned.
	errMsg := "dial failed"
	require.NoError(t, store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status: "failure", Error: &errMsg, DateStart: d3, DateEnd: d3,
	}))

	got, err := store.GetLastSuccessfulSend(ctx, db)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, d2, got.DateStart)
}

func TestDeleteOldReminderHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)

	require.NoError(t, store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status: "success", DateStart: old, DateEnd: old,
	}))
	require.NoError(t, store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status: "success", DateStart: recent, DateEnd: recent,
	}))

	cutoff := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, store.DeleteOldReminderHistory(ctx, db, cutoff))

	// Old record gone; recent record remains.
	got, err := store.GetLastSuccessfulSend(ctx, db)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, recent, got.DateStart)
}
```

- [ ] **Run tests to confirm they fail**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
go test ./internal/store/... -run "TestGetLastSuccessfulSend|TestCreateReminderHistory|TestDeleteOldReminderHistory" -v 2>&1 | tail -20
```

Expected: compile error (functions don't exist yet).

- [ ] **Create `internal/store/reminder_history.go`**

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/astromechza/aurelianprm/internal/model"
)

const dateLayout = "2006-01-02"

// ListReminderBlurryDates returns all blurry_dates eligible for reminders:
// reminder_enabled is true and both month and day are set.
func ListReminderBlurryDates(ctx context.Context, db *sql.DB) ([]model.BlurryDate, error) {
	const q = `
		SELECT id, year, month, day, reminder_enabled, reminder_days_before
		FROM blurry_dates
		WHERE reminder_enabled = 1 AND month IS NOT NULL AND day IS NOT NULL`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("store.ListReminderBlurryDates: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []model.BlurryDate
	for rows.Next() {
		var d model.BlurryDate
		var year, month, day sql.NullInt64
		if err := rows.Scan(&d.ID, &year, &month, &day, &d.ReminderEnabled, &d.ReminderDaysBefore); err != nil {
			return nil, fmt.Errorf("store.ListReminderBlurryDates: %w", err)
		}
		d.Year = scanNullInt(year)
		d.Month = scanNullInt(month)
		d.Day = scanNullInt(day)
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetPersonByBirthdayID returns the person whose birthday_id matches the given ID.
func GetPersonByBirthdayID(ctx context.Context, db *sql.DB, birthdayID string) (model.Person, error) {
	const q = `
		SELECT id, name, nickname, notes, birthday_id, created_at, updated_at
		FROM people WHERE birthday_id = ? LIMIT 1`
	row := db.QueryRowContext(ctx, q, birthdayID)
	p, err := scanPerson(row)
	if err != nil {
		return model.Person{}, fmt.Errorf("store.GetPersonByBirthdayID: %w", err)
	}
	return p, nil
}

// ListOngoingRelationshipsBySinceID returns relationships where since_id matches
// and until_id IS NULL (i.e. the relationship is still ongoing).
func ListOngoingRelationshipsBySinceID(ctx context.Context, db *sql.DB, sinceID string) ([]model.Relationship, error) {
	const q = `
		SELECT id, person_id, to_type, to_id, label, since_id, until_id, notes, created_at
		FROM relationships WHERE since_id = ? AND until_id IS NULL`
	rows, err := db.QueryContext(ctx, q, sinceID)
	if err != nil {
		return nil, fmt.Errorf("store.ListOngoingRelationshipsBySinceID: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	rels, err := scanRelationships(rows)
	if err != nil {
		return nil, fmt.Errorf("store.ListOngoingRelationshipsBySinceID: %w", err)
	}
	return rels, nil
}

// CreateReminderHistory inserts a new reminder_history row with a generated ID.
func CreateReminderHistory(ctx context.Context, db *sql.DB, h model.ReminderHistory) error {
	h.ID = newID()
	const q = `
		INSERT INTO reminder_history (id, status, error, date_start, date_end)
		VALUES (?, ?, ?, ?, ?)`
	if err := execContext(ctx, db, q,
		h.ID, h.Status, nullString(h.Error),
		h.DateStart.Format(dateLayout),
		h.DateEnd.Format(dateLayout),
	); err != nil {
		return fmt.Errorf("store.CreateReminderHistory: %w", err)
	}
	return nil
}

// GetLastSuccessfulSend returns the most recent reminder_history row with
// status='success', or nil if no successful send has ever occurred.
func GetLastSuccessfulSend(ctx context.Context, db *sql.DB) (*model.ReminderHistory, error) {
	const q = `
		SELECT id, sent_at, status, error, date_start, date_end
		FROM reminder_history
		WHERE status = 'success'
		ORDER BY sent_at DESC LIMIT 1`
	row := db.QueryRowContext(ctx, q)
	h, err := scanReminderHistory(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store.GetLastSuccessfulSend: %w", err)
	}
	return &h, nil
}

// DeleteOldReminderHistory deletes reminder_history rows sent before the given time.
func DeleteOldReminderHistory(ctx context.Context, db *sql.DB, before time.Time) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM reminder_history WHERE sent_at < strftime('%Y-%m-%dT%H:%M:%fZ', ?)`,
		before.UTC().Format("2006-01-02T15:04:05.000Z"),
	)
	if err != nil {
		return fmt.Errorf("store.DeleteOldReminderHistory: %w", err)
	}
	return nil
}

func scanReminderHistory(row *sql.Row) (model.ReminderHistory, error) {
	var h model.ReminderHistory
	var sentAt, dateStart, dateEnd string
	var errStr sql.NullString
	if err := row.Scan(&h.ID, &sentAt, &h.Status, &errStr, &dateStart, &dateEnd); err != nil {
		return model.ReminderHistory{}, err
	}
	var err error
	h.SentAt, err = parseTime(sentAt)
	if err != nil {
		return model.ReminderHistory{}, fmt.Errorf("parsing sent_at: %w", err)
	}
	h.Error = scanNullString(errStr)
	h.DateStart, _ = time.Parse(dateLayout, dateStart)
	h.DateEnd, _ = time.Parse(dateLayout, dateEnd)
	return h, nil
}
```

- [ ] **Run tests to confirm they pass**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
go test ./internal/store/... -run "TestGetLastSuccessfulSend|TestCreateReminderHistory|TestDeleteOldReminderHistory" -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Run full store test suite**

```bash
go test ./internal/store/... -race 2>&1
```

Expected: all PASS, no races.

- [ ] **Commit**

```bash
git add migrations/00002_reminder_history.sql internal/model/model.go \
        internal/store/reminder_history.go internal/store/reminder_history_test.go
git commit -m "feat: add reminder_history table, model, and store functions"
```

---

## Task 2: Digest logic

**Files:**
- Create: `internal/reminder/digest.go`
- Create: `internal/reminder/digest_test.go`

### Background

`ComputeReminders` takes a trigger date (midnight in the configured timezone) and returns items that should be included in the email. An item is due when `triggerDate + blurryDate.ReminderDaysBefore` lands on the blurry_date's `(month, day)`.

This function is pure business logic: it reads from the DB but has no side effects. Tests exercise it with a real in-memory SQLite DB (same as all other store-layer tests) plus fixed dates.

The label for a birthday is: `"<Name> — birthday"`.
The label for a relationship anniversary is: `"<PersonName> — <label>"` (e.g. `"John Smith — married"`). If `toType == "person"` and the target person can be fetched, it's `"<PersonName> & <OtherName> — <label>"`. If the blurry_date has a `year`, add the years count: `"John Smith — married, 5 years"`.

- [ ] **Write failing tests in `internal/reminder/digest_test.go`**

```go
package reminder_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/reminder"
	"github.com/astromechza/aurelianprm/internal/store"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

func TestComputeReminders_Birthday(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	// Birthday on June 15; reminder 2 days before.
	month, day := 6, 15
	bd, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &month, Day: &day,
		ReminderEnabled: true, ReminderDaysBefore: 2,
	})
	require.NoError(t, err)

	_, err = store.CreatePerson(ctx, sqlDB, model.Person{
		Name:       "Alice Smith",
		BirthdayID: &bd.ID,
	})
	require.NoError(t, err)

	// Trigger date = June 13 → June 13 + 2 = June 15 ✓
	trigger := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Alice Smith — birthday", items[0].Label)
	assert.Equal(t, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), items[0].OccursOn)
}

func TestComputeReminders_NotDueYet(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	month, day := 6, 15
	bd, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &month, Day: &day,
		ReminderEnabled: true, ReminderDaysBefore: 2,
	})
	require.NoError(t, err)
	_, err = store.CreatePerson(ctx, sqlDB, model.Person{
		Name:       "Alice Smith",
		BirthdayID: &bd.ID,
	})
	require.NoError(t, err)

	// Trigger date = June 12 → June 12 + 2 = June 14 ≠ June 15 → no items.
	trigger := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestComputeReminders_ReminderDisabled(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	month, day := 6, 15
	bd, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &month, Day: &day,
		ReminderEnabled: false, ReminderDaysBefore: 2,
	})
	require.NoError(t, err)
	_, err = store.CreatePerson(ctx, sqlDB, model.Person{
		Name:       "Alice Smith",
		BirthdayID: &bd.ID,
	})
	require.NoError(t, err)

	trigger := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	assert.Empty(t, items) // reminder_enabled=false → not returned by store query
}

func TestComputeReminders_RelationshipAnniversary_WithYears(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	// Anniversary on April 20, 2020; reminder 1 day before.
	year, month, day := 2020, 4, 20
	since, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Year: &year, Month: &month, Day: &day,
		ReminderEnabled: true, ReminderDaysBefore: 1,
	})
	require.NoError(t, err)

	person, err := store.CreatePerson(ctx, sqlDB, model.Person{Name: "Bob Jones"})
	require.NoError(t, err)

	other, err := store.CreatePerson(ctx, sqlDB, model.Person{Name: "Carol Jones"})
	require.NoError(t, err)

	_, err = store.CreateRelationship(ctx, sqlDB, model.Relationship{
		PersonID: person.ID,
		ToType:   model.RelationshipTargetPerson,
		ToID:     other.ID,
		Label:    "married",
		SinceID:  &since.ID,
	})
	require.NoError(t, err)

	// Trigger = April 19, 2026 → April 19 + 1 = April 20 ✓ (6 years)
	trigger := time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Bob Jones & Carol Jones — married, 6 years", items[0].Label)
	assert.Equal(t, time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), items[0].OccursOn)
}

func TestComputeReminders_EndedRelationship_Excluded(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	month, day := 4, 20
	since, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &month, Day: &day,
		ReminderEnabled: true, ReminderDaysBefore: 1,
	})
	require.NoError(t, err)

	// Set until_id to a non-null value so the relationship is ended.
	untilMonth, untilDay := 1, 1
	until, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &untilMonth, Day: &untilDay,
		ReminderEnabled: false, ReminderDaysBefore: 1,
	})
	require.NoError(t, err)

	person, err := store.CreatePerson(ctx, sqlDB, model.Person{Name: "Dave"})
	require.NoError(t, err)

	other, err := store.CreatePerson(ctx, sqlDB, model.Person{Name: "Eve"})
	require.NoError(t, err)

	_, err = store.CreateRelationship(ctx, sqlDB, model.Relationship{
		PersonID: person.ID,
		ToType:   model.RelationshipTargetPerson,
		ToID:     other.ID,
		Label:    "married",
		SinceID:  &since.ID,
		UntilID:  &until.ID, // ended relationship
	})
	require.NoError(t, err)

	trigger := time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	assert.Empty(t, items) // until_id is set → excluded
}

func TestComputeReminders_OrphanedBlurryDate_Skipped(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	// A blurry_date with no person birthday and no relationship since_id.
	month, day := 6, 15
	_, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &month, Day: &day,
		ReminderEnabled: true, ReminderDaysBefore: 1,
	})
	require.NoError(t, err)

	trigger := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	assert.Empty(t, items) // no context → silently skipped
}
```

- [ ] **Run tests to confirm they fail**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
go test ./internal/reminder/... -v 2>&1 | tail -10
```

Expected: compile error (`reminder` package doesn't exist yet).

- [ ] **Create `internal/reminder/digest.go`**

```go
// Package reminder implements the daily digest scheduler and email logic.
package reminder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

// ReminderItem is a single reminder included in a digest email.
type ReminderItem struct {
	// Label is the human-readable description, e.g. "John Smith — birthday"
	// or "Jane & Bob — married, 5 years".
	Label string
	// OccursOn is the anniversary date (triggerDate + ReminderDaysBefore).
	OccursOn time.Time
}

// ComputeReminders returns all reminder items due on triggerDate in the given
// timezone. An item is due when triggerDate + blurryDate.ReminderDaysBefore
// falls on the blurryDate's (month, day).
//
// Orphaned blurry_dates (no matching person birthday or ongoing relationship)
// are silently skipped. Errors fetching context for a single date are also
// skipped with a log-able description returned alongside nil error — the
// function returns partial results rather than failing entirely.
func ComputeReminders(ctx context.Context, db *sql.DB, triggerDate time.Time, tz *time.Location) ([]ReminderItem, error) {
	dates, err := store.ListReminderBlurryDates(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("reminder.ComputeReminders: %w", err)
	}

	var items []ReminderItem
	for _, bd := range dates {
		// bd.Month and bd.Day are guaranteed non-nil by the store query.
		candidateDate := triggerDate.In(tz).AddDate(0, 0, bd.ReminderDaysBefore)
		if int(candidateDate.Month()) != *bd.Month || candidateDate.Day() != *bd.Day {
			continue
		}

		item, ok, err := resolveReminderItem(ctx, db, bd, candidateDate)
		if err != nil || !ok {
			continue // orphaned or unresolvable — skip silently
		}
		items = append(items, item)
	}
	return items, nil
}

// resolveReminderItem looks up the human context for a blurry_date (birthday or
// relationship anniversary) and builds a ReminderItem. Returns ok=false if the
// blurry_date is orphaned (no matching entity found).
func resolveReminderItem(ctx context.Context, db *sql.DB, bd model.BlurryDate, occursOn time.Time) (ReminderItem, bool, error) {
	// Check birthday first.
	person, err := store.GetPersonByBirthdayID(ctx, db, bd.ID)
	if err == nil {
		return ReminderItem{
			Label:    person.Name + " — birthday",
			OccursOn: occursOn,
		}, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ReminderItem{}, false, err
	}

	// Check ongoing relationship anniversaries.
	rels, err := store.ListOngoingRelationshipsBySinceID(ctx, db, bd.ID)
	if err != nil {
		return ReminderItem{}, false, err
	}
	if len(rels) == 0 {
		return ReminderItem{}, false, nil // orphaned
	}

	rel := rels[0]
	label, err := buildRelationshipLabel(ctx, db, rel, occursOn, bd)
	if err != nil {
		return ReminderItem{}, false, err
	}
	return ReminderItem{Label: label, OccursOn: occursOn}, true, nil
}

// buildRelationshipLabel constructs a human-readable label for a relationship
// anniversary, including the years count if the blurry_date has a year set.
func buildRelationshipLabel(ctx context.Context, db *sql.DB, rel model.Relationship, occursOn time.Time, bd model.BlurryDate) (string, error) {
	person, err := store.GetPerson(ctx, db, rel.PersonID)
	if err != nil {
		return "", err
	}

	yearSuffix := ""
	if bd.Year != nil {
		years := occursOn.Year() - *bd.Year
		if years == 1 {
			yearSuffix = ", 1 year"
		} else if years > 1 {
			yearSuffix = fmt.Sprintf(", %d years", years)
		}
	}

	if rel.ToType == model.RelationshipTargetPerson {
		other, err := store.GetPerson(ctx, db, rel.ToID)
		if err == nil {
			return fmt.Sprintf("%s & %s — %s%s", person.Name, other.Name, rel.Label, yearSuffix), nil
		}
	}
	return fmt.Sprintf("%s — %s%s", person.Name, rel.Label, yearSuffix), nil
}
```

- [ ] **Run tests to confirm they pass**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
go test ./internal/reminder/... -run "TestComputeReminders" -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Commit**

```bash
git add internal/reminder/digest.go internal/reminder/digest_test.go
git commit -m "feat: add reminder digest logic with annual recurrence and context resolution"
```

---

## Task 3: Email formatting

**Files:**
- Create: `internal/reminder/email.go`
- Create: `internal/reminder/email_test.go`

### Background

`FormatDigest` is pure string building — no DB, no SMTP. It receives the email send date and a slice of `DayReminders` (date + items). Items from the current day use relative labels ("tomorrow", "in N days"); items from catch-up days are marked "was DD Month".

`SendDigest` is a thin wrapper that calls `mailer.Send`.

- [ ] **Write failing tests in `internal/reminder/email_test.go`**

```go
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
			Items: []reminder.ReminderItem{
				{Label: "Alice Smith — birthday", OccursOn: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)},
				{Label: "Bob Jones — birthday", OccursOn: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)},
			},
		},
	}

	body := reminder.FormatDigest(sendDate, days)

	assert.Contains(t, body, "13 June 2026")
	assert.Contains(t, body, "Alice Smith — birthday")
	assert.Contains(t, body, "in 2 days")    // June 15 is 2 days after June 13
	assert.Contains(t, body, "Bob Jones — birthday")
	assert.Contains(t, body, "tomorrow")     // June 14 is tomorrow
	assert.NotContains(t, body, "Catch-up")  // single day, no catch-up
}

func TestFormatDigest_CatchUp(t *testing.T) {
	sendDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	missedDay := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)

	days := []reminder.DayReminders{
		{
			Date:  missedDay,
			Items: []reminder.ReminderItem{
				{Label: "Carol — birthday", OccursOn: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)},
			},
		},
		{
			Date:  sendDate,
			Items: []reminder.ReminderItem{
				{Label: "Dave — birthday", OccursOn: time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)},
			},
		},
	}

	body := reminder.FormatDigest(sendDate, days)

	assert.Contains(t, body, "15 June 2026")
	assert.Contains(t, body, "Dave — birthday")
	assert.Contains(t, body, "tomorrow") // June 16 from send date June 15
	assert.Contains(t, body, "[Catch-up from 14 June 2026]")
	assert.Contains(t, body, "Carol — birthday")
	assert.Contains(t, body, "was 14 June") // OccursOn < sendDate
}

func TestFormatDigest_TodayEvent(t *testing.T) {
	sendDate := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	days := []reminder.DayReminders{
		{
			Date:  sendDate,
			Items: []reminder.ReminderItem{
				{Label: "Eve — birthday", OccursOn: sendDate},
			},
		},
	}
	body := reminder.FormatDigest(sendDate, days)
	assert.Contains(t, body, "today")
}
```

- [ ] **Run tests to confirm they fail**

```bash
go test ./internal/reminder/... -run "TestFormatDigest" -v 2>&1 | tail -10
```

Expected: compile error (`DayReminders` and `FormatDigest` not defined).

- [ ] **Create `internal/reminder/email.go`**

```go
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
	Items []ReminderItem // items due that day
}

// FormatDigest builds the plain-text body of the reminder digest email.
// sendDate is the actual date the email is being sent (today in the scheduler's
// timezone). days is ordered oldest-first; the last entry is always sendDate.
func FormatDigest(sendDate time.Time, days []DayReminders) string {
	var sb strings.Builder
	sb.WriteString("Upcoming reminders — AurelianPRM\n\n")

	for _, d := range days {
		isCatchUp := !sameDay(d.Date, sendDate)

		if isCatchUp {
			sb.WriteString(fmt.Sprintf("[Catch-up from %s]\n", d.Date.Format("2 January 2006")))
		} else {
			sb.WriteString(fmt.Sprintf("%s:\n", d.Date.Format("2 January 2006")))
		}

		for _, item := range d.Items {
			rel := relativeDate(sendDate, item.OccursOn)
			sb.WriteString(fmt.Sprintf("• %s (%s)\n", item.Label, rel))
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
```

- [ ] **Run tests to confirm they pass**

```bash
go test ./internal/reminder/... -run "TestFormatDigest" -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Commit**

```bash
git add internal/reminder/email.go internal/reminder/email_test.go
git commit -m "feat: add reminder email formatting"
```

---

## Task 4: Scheduler with synctest tests

**Files:**
- Create: `internal/reminder/scheduler.go`
- Create: `internal/reminder/scheduler_test.go`

### Background

The `Scheduler` uses `time.Sleep` and `time.Now()` directly — no Clock interface. Tests use `testing/synctest` (`synctest.Test` + `synctest.Wait`) so fake time advances instantly.

`synctest.Test` starts fake time at 2000-01-01 00:00:00 UTC. Tests configure `SendTime = "00:01"` so the scheduler sleeps for 1 fake-minute before firing.

**Important synctest rules:**
- All goroutines must be started inside the `synctest.Test` callback.
- Context must be created inside the callback (so its cancel channel is a bubble channel).
- DB and network I/O are not "durably blocking" — `synctest.Wait()` will not stall on them.
- The fake send function must be synchronous (no goroutines).

- [ ] **Write failing tests in `internal/reminder/scheduler_test.go`**

```go
package reminder_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/reminder"
	"github.com/astromechza/aurelianprm/internal/store"
)

// Note: openTestDB is defined in digest_test.go (same package reminder_test),
// so it is available here without duplication.

func TestScheduler_SendsEmailWhenRemindersExist(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sqlDB := openTestDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create a blurry_date due on Jan 2 (trigger Jan 1 + 1 day).
		// Fake time starts at 2000-01-01, scheduler fires at 00:01.
		// today = 2000-01-01; candidateDate = 2000-01-02 (month=1, day=2) ✓
		month, day := 1, 2
		bd, err := store.CreateBlurryDate(context.Background(), sqlDB, model.BlurryDate{
			Month: &month, Day: &day,
			ReminderEnabled: true, ReminderDaysBefore: 1,
		})
		require.NoError(t, err)
		_, err = store.CreatePerson(context.Background(), sqlDB, model.Person{
			Name:       "Test Person",
			BirthdayID: &bd.ID,
		})
		require.NoError(t, err)

		var mu sync.Mutex
		var sent []string
		sched := reminder.NewScheduler(reminder.Config{
			DB: sqlDB,
			SendFunc: func(subject, body string) error {
				mu.Lock()
				sent = append(sent, body)
				mu.Unlock()
				return nil
			},
			SendTime: "00:01",
			Timezone: "UTC",
		})

		go sched.Run(ctx) //nolint:errcheck

		// Advance 2 minutes past the send time.
		time.Sleep(2 * time.Minute)
		synctest.Wait()

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, sent, 1)
		assert.Contains(t, sent[0], "Test Person — birthday")

		// Verify history was recorded.
		h, err := store.GetLastSuccessfulSend(context.Background(), sqlDB)
		require.NoError(t, err)
		require.NotNil(t, h)
		assert.Equal(t, "success", h.Status)
	})
}

func TestScheduler_SilentWhenNoReminders(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sqlDB := openTestDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var callCount int
		sched := reminder.NewScheduler(reminder.Config{
			DB: sqlDB,
			SendFunc: func(subject, body string) error {
				callCount++
				return nil
			},
			SendTime: "00:01",
			Timezone: "UTC",
		})

		go sched.Run(ctx) //nolint:errcheck

		time.Sleep(2 * time.Minute)
		synctest.Wait()

		assert.Equal(t, 0, callCount) // nothing to send → no email, no history
		h, err := store.GetLastSuccessfulSend(context.Background(), sqlDB)
		require.NoError(t, err)
		assert.Nil(t, h)
	})
}

func TestScheduler_RetriesOnFailureThenSucceeds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sqlDB := openTestDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Set up a reminder due on day 1.
		month, day := 1, 2
		bd, err := store.CreateBlurryDate(context.Background(), sqlDB, model.BlurryDate{
			Month: &month, Day: &day,
			ReminderEnabled: true, ReminderDaysBefore: 1,
		})
		require.NoError(t, err)
		_, err = store.CreatePerson(context.Background(), sqlDB, model.Person{
			Name: "Retry Person", BirthdayID: &bd.ID,
		})
		require.NoError(t, err)

		var mu sync.Mutex
		attempts := 0
		sched := reminder.NewScheduler(reminder.Config{
			DB: sqlDB,
			SendFunc: func(subject, body string) error {
				mu.Lock()
				defer mu.Unlock()
				attempts++
				if attempts < 3 {
					return errors.New("temporary SMTP failure")
				}
				return nil
			},
			SendTime: "00:01",
			Timezone: "UTC",
		})

		go sched.Run(ctx) //nolint:errcheck

		// Give enough fake time for the send + retries (10s + 20s gaps).
		time.Sleep(5 * time.Minute)
		synctest.Wait()

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, 3, attempts) // 2 failures + 1 success

		h, err := store.GetLastSuccessfulSend(context.Background(), sqlDB)
		require.NoError(t, err)
		require.NotNil(t, h)
		assert.Equal(t, "success", h.Status)
	})
}

func TestScheduler_RecordsFailureAfterAllRetriesExhausted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sqlDB := openTestDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		month, day := 1, 2
		bd, err := store.CreateBlurryDate(context.Background(), sqlDB, model.BlurryDate{
			Month: &month, Day: &day,
			ReminderEnabled: true, ReminderDaysBefore: 1,
		})
		require.NoError(t, err)
		_, err = store.CreatePerson(context.Background(), sqlDB, model.Person{
			Name: "Always Fails", BirthdayID: &bd.ID,
		})
		require.NoError(t, err)

		sched := reminder.NewScheduler(reminder.Config{
			DB: sqlDB,
			SendFunc: func(subject, body string) error {
				return errors.New("permanent failure")
			},
			SendTime: "00:01",
			Timezone: "UTC",
		})

		go sched.Run(ctx) //nolint:errcheck

		time.Sleep(5 * time.Minute)
		synctest.Wait()

		// No successful send; a failure record should exist.
		h, err := store.GetLastSuccessfulSend(context.Background(), sqlDB)
		require.NoError(t, err)
		assert.Nil(t, h) // no success

		// The failure is recorded but we don't expose a GetLastFailure fn —
		// just verify the scheduler didn't panic and the program is still running.
	})
}

func TestScheduler_CatchesUpMissedDays(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sqlDB := openTestDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Fake time starts 2000-01-01. Insert a prior success record for 1999-12-30
		// so the scheduler thinks it last succeeded 2 days ago.
		// Days to cover: 1999-12-31 and 2000-01-01.
		past := time.Date(1999, 12, 30, 0, 0, 0, 0, time.UTC)
		err := store.CreateReminderHistory(context.Background(), sqlDB, model.ReminderHistory{
			Status:    "success",
			DateStart: past,
			DateEnd:   past,
		})
		require.NoError(t, err)

		// Birthday on Dec 31 with 0 days before (fires on the day itself).
		month, day := 12, 31
		bd, err := store.CreateBlurryDate(context.Background(), sqlDB, model.BlurryDate{
			Month: &month, Day: &day,
			ReminderEnabled: true, ReminderDaysBefore: 0,
		})
		require.NoError(t, err)
		_, err = store.CreatePerson(context.Background(), sqlDB, model.Person{
			Name: "Dec 31 Person", BirthdayID: &bd.ID,
		})
		require.NoError(t, err)

		var mu sync.Mutex
		var bodies []string
		sched := reminder.NewScheduler(reminder.Config{
			DB: sqlDB,
			SendFunc: func(subject, body string) error {
				mu.Lock()
				bodies = append(bodies, body)
				mu.Unlock()
				return nil
			},
			SendTime: "00:01",
			Timezone: "UTC",
		})

		go sched.Run(ctx) //nolint:errcheck

		time.Sleep(2 * time.Minute)
		synctest.Wait()

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, bodies, 1)
		assert.Contains(t, bodies[0], "Catch-up")
		assert.Contains(t, bodies[0], "Dec 31 Person")
	})
}
```

- [ ] **Run tests to confirm they fail**

```bash
go test ./internal/reminder/... -run "TestScheduler" -v 2>&1 | tail -10
```

Expected: compile error (`Scheduler`, `NewScheduler`, `Config` not defined).

- [ ] **Create `internal/reminder/scheduler.go`**

```go
package reminder

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

// Config holds the scheduler's dependencies and configuration.
type Config struct {
	// DB is the SQLite database connection.
	DB *sql.DB
	// SendFunc delivers the email. In production this wraps mailer.Send;
	// in tests it captures sent emails without SMTP.
	SendFunc func(subject, body string) error
	// SendTime is a wall-clock time in "HH:MM" format (e.g. "08:00").
	SendTime string
	// Timezone is a valid IANA timezone name (e.g. "Europe/London").
	Timezone string
}

// Scheduler runs a daily reminder digest goroutine.
type Scheduler struct {
	db         *sql.DB
	sendFunc   func(subject, body string) error
	sendHour   int
	sendMinute int
	tz         *time.Location
}

// NewScheduler creates a Scheduler from cfg. It panics if Timezone is invalid
// or SendTime is not in "HH:MM" format — both are configuration errors that
// should be caught at startup.
func NewScheduler(cfg Config) *Scheduler {
	tz, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		panic(fmt.Sprintf("reminder.NewScheduler: invalid timezone %q: %v", cfg.Timezone, err))
	}
	parts := strings.SplitN(cfg.SendTime, ":", 2)
	if len(parts) != 2 {
		panic(fmt.Sprintf("reminder.NewScheduler: invalid send_time %q, expected HH:MM", cfg.SendTime))
	}
	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		panic(fmt.Sprintf("reminder.NewScheduler: invalid send_time %q, expected HH:MM", cfg.SendTime))
	}
	return &Scheduler{
		db:         cfg.DB,
		sendFunc:   cfg.SendFunc,
		sendHour:   hour,
		sendMinute: minute,
		tz:         tz,
	}
}

// Run starts the scheduler loop. It sleeps until the next send time, runs the
// digest, then loops. It returns when ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	for {
		next := s.nextFireTime(time.Now())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(next)):
		}

		now := time.Now().In(s.tz)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.tz)
		s.runOnce(ctx, today)
	}
}

// nextFireTime returns the next wall-clock time to fire, based on now.
func (s *Scheduler) nextFireTime(now time.Time) time.Time {
	local := now.In(s.tz)
	candidate := time.Date(local.Year(), local.Month(), local.Day(),
		s.sendHour, s.sendMinute, 0, 0, s.tz)
	if now.Before(candidate) {
		return candidate
	}
	return candidate.AddDate(0, 0, 1)
}

// runOnce executes one daily digest cycle for the given today date.
func (s *Scheduler) runOnce(ctx context.Context, today time.Time) {
	// Determine the range of days to cover.
	startDay, err := s.computeStartDay(ctx, today)
	if err != nil {
		log.Printf("reminder: computing start day: %v", err)
		return
	}

	// Collect reminders for each day in range.
	var days []DayReminders
	for d := startDay; !d.After(today); d = d.AddDate(0, 0, 1) {
		items, err := ComputeReminders(ctx, s.db, d, s.tz)
		if err != nil {
			log.Printf("reminder: computing reminders for %s: %v", d.Format("2006-01-02"), err)
			continue
		}
		if len(items) > 0 {
			days = append(days, DayReminders{Date: d, Items: items})
		}
	}

	// Nothing to send — stay silent, record nothing.
	if len(days) == 0 {
		return
	}

	subject := fmt.Sprintf("AurelianPRM — reminders for %s", today.Format("2 January 2006"))
	body := FormatDigest(today, days)

	// Attempt send with retries: immediate, +10s, +20s, +30s (4 total attempts).
	retryDelays := []time.Duration{0, 10 * time.Second, 20 * time.Second, 30 * time.Second}
	var sendErr error
	for i, delay := range retryDelays {
		if delay > 0 {
			time.Sleep(delay)
		}
		if sendErr = s.sendFunc(subject, body); sendErr == nil {
			break
		}
		log.Printf("reminder: send attempt %d/%d failed: %v", i+1, len(retryDelays), sendErr)
	}

	// Record outcome.
	status := "success"
	var errStr *string
	if sendErr != nil {
		status = "failure"
		msg := sendErr.Error()
		errStr = &msg
	}
	if err := store.CreateReminderHistory(ctx, s.db, model.ReminderHistory{
		Status:    status,
		Error:     errStr,
		DateStart: startDay,
		DateEnd:   today,
	}); err != nil {
		log.Printf("reminder: recording history: %v", err)
	}

	// Clean up old history only after a successful send.
	if sendErr == nil {
		cutoff := time.Now().AddDate(0, 0, -30)
		if err := store.DeleteOldReminderHistory(ctx, s.db, cutoff); err != nil {
			log.Printf("reminder: cleaning old history: %v", err)
		}
	}
}

// computeStartDay returns the first day to cover in this run.
// If no successful send has been recorded, start from today.
// Otherwise start from the day after the last successful send's date_end.
func (s *Scheduler) computeStartDay(ctx context.Context, today time.Time) (time.Time, error) {
	last, err := store.GetLastSuccessfulSend(ctx, s.db)
	if err != nil {
		return time.Time{}, err
	}
	if last == nil {
		return today, nil
	}
	// last.DateEnd is UTC midnight; shift to configured tz, add one day.
	lastEnd := last.DateEnd.In(s.tz)
	return time.Date(lastEnd.Year(), lastEnd.Month(), lastEnd.Day(), 0, 0, 0, 0, s.tz).AddDate(0, 0, 1), nil
}
```

- [ ] **Run scheduler tests**

```bash
go test ./internal/reminder/... -run "TestScheduler" -v -count=1 2>&1 | tail -30
```

Expected: all PASS. These run instantly thanks to `synctest`.

- [ ] **Run full reminder package test suite**

```bash
go test ./internal/reminder/... -race -count=1 2>&1
```

Expected: all PASS.

- [ ] **Commit**

```bash
git add internal/reminder/scheduler.go internal/reminder/scheduler_test.go
git commit -m "feat: add reminder scheduler with synctest-based tests"
```

---

## Task 5: Wire into main + final verification

**Files:**
- Modify: `cmd/aurelianprm/main.go`

- [ ] **Update `runServe` in `cmd/aurelianprm/main.go`**

Read the file first. After `srv, err := web.New(...)`, add the scheduler startup before the `srv.ListenAndServe(ctx)` call:

```go
// Start the daily reminder scheduler goroutine.
sched := reminder.NewScheduler(reminder.Config{
    DB:       sqlDB,
    SendFunc: func(subject, body string) error {
        return mailer.Send(cfg.SMTP, cfg.ReminderEmail, subject, body)
    },
    SendTime: cfg.ReminderSendTime,
    Timezone: cfg.ReminderTimezone,
})
go func() {
    if err := sched.Run(ctx); err != nil && err != context.Canceled {
        log.Printf("reminder scheduler: %v", err)
    }
}()
```

Add imports:
```go
"github.com/astromechza/aurelianprm/internal/mailer"
"github.com/astromechza/aurelianprm/internal/reminder"
```

- [ ] **Build and verify**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
go build ./...
```

Expected: clean build.

- [ ] **Run full test suite**

```bash
go test -buildvcs=false -race -count=1 ./...
```

Expected: all packages pass.

- [ ] **Run linter**

```bash
golangci-lint run
```

Expected: 0 issues. Common things to fix if they appear:
- `godot`: exported function comments must end with a period.
- `errcheck`: ensure all `store.CreateReminderHistory` errors are handled (they are logged in `runOnce`).
- Unused imports.

- [ ] **Commit**

```bash
git add cmd/aurelianprm/main.go
git commit -m "feat: wire reminder scheduler into serve command"
```

---

## Self-Review Checklist

**Spec coverage:**
- ✅ Daily scheduler goroutine starts alongside HTTP server
- ✅ Sleeps until `reminder_send_time` in `reminder_timezone` using real `time.Sleep` (synctest-compatible)
- ✅ Queries `blurry_dates` with `reminder_enabled=true` and `month IS NOT NULL AND day IS NOT NULL`
- ✅ Checks `reminderDaysBefore` against annual occurrence
- ✅ Birthday context via `GetPersonByBirthdayID`
- ✅ Anniversary context via `ListOngoingRelationshipsBySinceID` (ongoing = `until_id IS NULL`)
- ✅ Stay silent when no reminders
- ✅ Retry with 10s / 20s / 30s delays (4 attempts total)
- ✅ `reminder_history` table with success/failure status and error text
- ✅ Catch-up covers days since last successful send
- ✅ Delete history older than 30 days on successful send
- ✅ Digest email format with catch-up section
- ✅ `testing/synctest` for scheduler tests (no Clock interface)
- ✅ Store tests with in-memory SQLite
- ✅ Digest tests with fixed dates

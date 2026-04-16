# Phase 5 — Reminder System Design

## Goal

Send a daily plain-text digest email listing upcoming birthdays and relationship anniversaries. Stay silent on days with no reminders. Retry on SMTP failure and catch up on missed days when the service recovers.

## Architecture

A single goroutine starts alongside the HTTP server. It sleeps until `reminder_send_time` (wall-clock, in `reminder_timezone`) each day, computes what is due, and sends one email if anything is pending. An injected `Clock` interface allows deterministic unit testing without real sleeps.

## Package Structure

**`internal/reminder/`** — four files, each with one clear responsibility:

| File | Responsibility |
|---|---|
| `clock.go` | `Clock` interface (`Now() time.Time`) + `RealClock` production implementation |
| `digest.go` | `ComputeReminders(ctx, db, date, tz)` — pure date logic, finds what is due on a given calendar day |
| `email.go` | `FormatDigest(...)` builds the plain-text body; `SendDigest(...)` wraps `internal/mailer` |
| `scheduler.go` | `Scheduler` struct + `Run(ctx context.Context) error` — the goroutine loop, retry, history |

**`internal/store/reminder_history.go`** — new store file for history queries.

**`migrations/00002_reminder_history.sql`** — new migration.

## Database

New table added by migration `00002_reminder_history.sql`:

```sql
CREATE TABLE reminder_history (
  id         TEXT PRIMARY KEY,
  sent_at    DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  status     TEXT NOT NULL CHECK (status IN ('success', 'failure')),
  error      TEXT,           -- null on success, error string on failure
  date_start TEXT NOT NULL,  -- YYYY-MM-DD, first calendar day this email covers
  date_end   TEXT NOT NULL   -- YYYY-MM-DD, last calendar day this email covers
);
```

## New Store Functions

All in `internal/store/reminder_history.go`:

- `ListReminderBlurryDates(ctx, db) ([]model.BlurryDate, error)` — all blurry_dates where `reminder_enabled = true AND month IS NOT NULL AND day IS NOT NULL`. Full scan; dataset is small.
- `GetPersonByBirthdayID(ctx, db, birthdayID string) (model.Person, error)` — person whose `birthday_id` matches.
- `ListOngoingRelationshipsBySinceID(ctx, db, sinceID string) ([]model.Relationship, error)` — relationships where `since_id = ?` AND `until_id IS NULL`.
- `CreateReminderHistory(ctx, db, h model.ReminderHistory) error`
- `GetLastSuccessfulSend(ctx, db) (*model.ReminderHistory, error)` — most recent row with `status = 'success'`, or nil if none.
- `DeleteOldReminderHistory(ctx, db, before time.Time) error` — deletes rows where `sent_at < before`.

New model type in `internal/model/model.go`:

```go
type ReminderHistory struct {
    ID        string
    SentAt    time.Time
    Status    string // "success" | "failure"
    Error     *string
    DateStart time.Time // date only, time component ignored
    DateEnd   time.Time // date only, time component ignored
}
```

## Digest Logic (`digest.go`)

`ComputeReminders` accepts a calendar date and returns a slice of `ReminderItem`:

```go
type ReminderItem struct {
    Label      string    // e.g. "John Smith — birthday" or "Jane & Bob — married"
    OccursOn   time.Time // the actual upcoming date of the event (the anniversary date)
    DaysAway   int       // days from the trigger date to OccursOn (may be 0 for "today")
}
```

**Algorithm for each blurry_date:**

1. The date must have both `month` and `day` set (enforced by the store query).
2. Compute the next annual occurrence of `(month, day)` on or after `triggerDate` in `tz`.
3. Calculate `daysUntil = occursOn - triggerDate` (calendar days in `tz`).
4. If `daysUntil == blurryDate.ReminderDaysBefore`, this item is due — include it.
5. Resolve context: query `GetPersonByBirthdayID` first; if no match, query `ListOngoingRelationshipsBySinceID`. Skip if neither returns a result (orphaned blurry_date).
6. Build a human-readable `Label`:
   - Birthday: `"John Smith — birthday"`
   - Relationship: `"<person.Name> — <relationship.Label>"` (e.g. `"Jane Smith — married"`)

**Year-less dates:** blurry_dates with no `year` always match annually. Dates with a `year` are treated the same way — the year is used only for computing "N years" anniversary counts in the label (e.g. `"married, 5 years"`), not for filtering.

## Scheduler Flow (`scheduler.go`)

```
On startup:
  if reminder_send_time has not yet passed today (in tz):
    sleep until today's send time
  else:
    sleep until tomorrow's send time

Each day on wake:
  today = clock.Now() in tz, truncated to date

  lastSuccess = GetLastSuccessfulSend()
  if lastSuccess == nil:
    daysToCheck = [today]
  else:
    daysToCheck = [lastSuccess.DateEnd + 1 day, ..., today]

  allReminders = map[date][]ReminderItem
  for each day in daysToCheck:
    reminders = ComputeReminders(ctx, db, day, tz)
    if len(reminders) > 0:
      allReminders[day] = reminders

  if len(allReminders) == 0:
    sleep until tomorrow — no email, no history record

  attempt to SendDigest with retries:
    delays between attempts: 10s, 20s, 30s (max 4 total attempts)
    on success:
      CreateReminderHistory(status=success, date_start=daysToCheck[0], date_end=today)
      DeleteOldReminderHistory(before=now - 30 days)
    on final failure:
      CreateReminderHistory(status=failure, error=err.Error(), date_start=daysToCheck[0], date_end=today)

  sleep until tomorrow's send time
```

## Email Format

Subject: `AurelianPRM — reminders for 16 April 2026`

Body (plain text):

```
Upcoming reminders — AurelianPRM

16 April 2026:
• John Smith — birthday (in 2 days, 18 April)
• Jane Smith — married (tomorrow, 17 April)

[Catch-up from 15 April 2026:]
• Sarah Connor — birthday (was 15 April)
```

- Days in the current run are listed first, labeled by date.
- Catch-up days (from missed sends) are clearly marked.
- If `OccursOn == triggerDate`: `"today, 16 April"`.
- If `OccursOn == triggerDate + 1`: `"tomorrow, 17 April"`.
- If `OccursOn < triggerDate` (catch-up): `"was 15 April"`.
- Otherwise: `"in N days, DD Month"`.
- Anniversary labels include years if the blurry_date has a `year`: `"married, 5 years"`.

## Wiring (`cmd/aurelianprm/main.go`)

In `runServe`, after creating the web server, start the scheduler goroutine:

```go
sched := reminder.NewScheduler(reminder.Config{
    DB:           sqlDB,
    Mailer:       cfg.SMTP,
    To:           cfg.ReminderEmail,
    SendTime:     cfg.ReminderSendTime,   // "HH:MM"
    Timezone:     cfg.ReminderTimezone,   // IANA name
    Clock:        reminder.RealClock{},
})
go func() {
    if err := sched.Run(ctx); err != nil {
        log.Printf("reminder scheduler stopped: %v", err)
    }
}()
```

The scheduler exits cleanly when `ctx` is cancelled (server shutdown).

## Testing

- **`digest_test.go`**: pure unit tests with fixed dates. Covers: due today, due tomorrow, due in N days, year-less date, anniversary with year count, no match (wrong days_before), orphaned blurry_date (skipped).
- **`scheduler_test.go`**: `MockClock` returns controlled times. Real in-memory SQLite. Fake sender function (captures calls). Tests: send fires at correct time, catch-up covers missed days, retry on failure then success, stay silent when no reminders, history cleanup runs after success.
- **`store/reminder_history_test.go`**: standard store tests against in-memory SQLite.

## Constraints

- Scheduler runs inside the server process — not a separate binary or cron job.
- No email is ever sent for days with zero reminders (stay silent).
- Anniversary reminders only fire for ongoing relationships (`until_id IS NULL`).
- History older than 30 days is deleted on each successful send.
- Check-in interval reminders are out of scope (deferred to a future version).

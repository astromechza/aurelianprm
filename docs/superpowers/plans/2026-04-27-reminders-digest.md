# Reminders Digest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `send-digest` CLI subcommand that queries the database for upcoming reminders (v1: birthdays in the next 7 days) and sends a plain-text email digest; no email is sent when there are nothing to report.

**Architecture:** New `internal/digest/` package owns all reminder and email logic. `main.go` gains subcommand dispatch — `send-digest` runs the digest, any other invocation serves HTTP as before. SMTP credentials and recipient address come from environment variables. No database migration required.

**Tech Stack:** Go 1.26, SQLite (existing), `github.com/wneessen/go-mail` for SMTP/TLS, testify.

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/digest/birthday.go` | Create | Birthday reminder source (`findBirthdayReminders`) |
| `internal/digest/digest.go` | Create | `Reminder` type, `Config`, `FindReminders`, `composeBody`, `sendDigest`, `SendDigest` |
| `internal/digest/smtp.go` | Create | go-mail SMTP sender wired to `Config` |
| `internal/digest/digest_test.go` | Create | All unit tests (no live SMTP) |
| `main.go` | Modify | Subcommand dispatch + `runDigest` + env-var reading |

---

## Task 1: Birthday reminder source

**Files:**
- Create: `internal/digest/birthday.go`
- Create: `internal/digest/digest_test.go` (birthday tests only for now)

The birthday window is **today through today+6 inclusive** (7 days, today counts). Birthday arithmetic is done in Go — SQLite has no clean month/day range query. Go's `time.Date` automatically normalises Feb 29 → Mar 1 in non-leap years, which matches the spec.

- [ ] **Step 1: Create `internal/digest/birthday.go`**

```go
package digest

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/astromechza/aurelianprm/internal/dal"
)

// personBirthData holds the birthday fields decoded from a Person entity's JSON data.
type personBirthData struct {
	Name       string `json:"name"`
	BirthYear  int    `json:"birthYear"`
	BirthMonth int    `json:"birthMonth"`
	BirthDay   int    `json:"birthDay"`
}

// findBirthdayReminders returns Reminders for persons whose birthday falls in
// [today, today+6] (inclusive). now is used as the reference point.
// Go normalises Feb 29 → Mar 1 for non-leap years automatically.
func findBirthdayReminders(persons []dal.Entity, now time.Time) []Reminder {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windowEnd := today.AddDate(0, 0, 7) // exclusive upper bound

	var reminders []Reminder
	for _, entity := range persons {
		var pd personBirthData
		if err := json.Unmarshal(entity.Data, &pd); err != nil {
			continue
		}
		if pd.BirthMonth == 0 || pd.BirthDay == 0 {
			continue
		}
		// Try this year then next year; take the first candidate that falls in window.
		for _, year := range []int{today.Year(), today.Year() + 1} {
			candidate := time.Date(year, time.Month(pd.BirthMonth), pd.BirthDay, 0, 0, 0, 0, today.Location())
			if !candidate.Before(today) && candidate.Before(windowEnd) {
				label := fmt.Sprintf("%s's birthday", pd.Name)
				if pd.BirthYear > 0 {
					label = fmt.Sprintf("%s's birthday (turning %d)", pd.Name, year-pd.BirthYear)
				}
				reminders = append(reminders, Reminder{Date: candidate, Label: label})
				break
			}
		}
	}
	return reminders
}
```

Note: `Reminder` type is defined in `digest.go` (Task 2). This file compiles once Task 2 exists.

- [ ] **Step 2: Create `internal/digest/digest_test.go`** with birthday tests

```go
package digest

import (
	"context"
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

// --- placeholders for Task 2 tests (compile guard) ---
var _ = context.Background
var _ = db.Open
```

- [ ] **Step 3: Run tests — expect FAIL (Reminder type undefined)**

```bash
go test ./internal/digest/... -count=1 -v 2>&1 | tail -10
```

Expected: compile error `undefined: Reminder` — correct, Task 2 defines it.

- [ ] **Step 4: Commit `birthday.go` and test file skeleton**

```bash
git add internal/digest/birthday.go internal/digest/digest_test.go
git commit -m "feat: add birthday reminder source and test skeleton"
```

---

## Task 2: Digest core — Reminder, FindReminders, composeBody, sendDigest

**Files:**
- Create: `internal/digest/digest.go`
- Modify: `internal/digest/digest_test.go` (add digest tests)

`SendDigest` (public) calls `sendDigest` (internal) with a real SMTP sender. Tests inject a `senderFunc` mock — no live SMTP needed.

- [ ] **Step 1: Write failing tests** — append to `internal/digest/digest_test.go`

Replace the placeholder comment block at the end of the test file with:

```go
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
	err = sendDigest(t.Context(), d, Config{}, time.Now(), func(subject, body, from, to string) error {
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

	// Insert a person with birthday today
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
		func(subject, body, from, to string) error {
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
```

- [ ] **Step 2: Run tests — expect FAIL (FindReminders, composeBody, sendDigest undefined)**

```bash
go test ./internal/digest/... -count=1 2>&1 | tail -5
```

Expected: compile errors — correct.

- [ ] **Step 3: Create `internal/digest/digest.go`**

```go
// Package digest provides the send-digest command: queries the database for
// upcoming reminders and sends a plain-text email digest via SMTP.
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
// All fields except SMTPPort are required; SMTPPort defaults to 587 if zero.
type Config struct {
	SMTPHost string
	SMTPPort int    // default 587
	SMTPUser string
	SMTPPass string
	SMTPFrom string
	SMTPSSL  bool   // true = implicit TLS (port 465); false = STARTTLS mandatory (port 587)
	DigestTo string
}

// senderFunc delivers a composed message. Injected in tests to avoid live SMTP.
type senderFunc func(subject, body, from, to string) error

// FindReminders aggregates all reminder sources for the window [now, now+6] and
// returns results sorted by date ascending, then label ascending.
func FindReminders(persons []dal.Entity, now time.Time) []Reminder {
	var all []Reminder
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
		sb.WriteString(fmt.Sprintf("• %s — %s\n", r.Date.Format("Mon 2 Jan"), r.Label))
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

// SendDigest is the public entry point. Opens the DB, finds reminders, and sends
// a digest email. No email is sent if no reminders are found.
func SendDigest(ctx context.Context, d *dal.DAL, cfg Config) error {
	return sendDigest(ctx, d, cfg, time.Now(), newSMTPSender(cfg))
}
```

- [ ] **Step 4: Create a stub `internal/digest/smtp.go`** so the package compiles before go-mail is added

```go
package digest

// newSMTPSender returns a senderFunc backed by real SMTP (implemented in Task 3).
// This stub satisfies the compiler; Task 3 replaces it with go-mail.
func newSMTPSender(_ Config) senderFunc {
	return func(subject, body, from, to string) error {
		panic("newSMTPSender not yet implemented — run Task 3")
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/digest/... -count=1 -v 2>&1 | tail -20
```

Expected: all 11 tests PASS. The `TestSendDigest_*` tests use the injected `senderFunc` mock, never hitting the stub panic.

- [ ] **Step 6: Commit**

```bash
git add internal/digest/digest.go internal/digest/smtp.go internal/digest/digest_test.go
git commit -m "feat: add digest core — Reminder, FindReminders, composeBody, sendDigest"
```

---

## Task 3: go-mail SMTP sender + main.go subcommand

**Files:**
- Modify: `internal/digest/smtp.go` (replace stub with real go-mail implementation)
- Modify: `main.go` (add subcommand dispatch, `runDigest`, env-var reading)

- [ ] **Step 1: Add go-mail dependency**

```bash
go get github.com/wneessen/go-mail@latest
```

Expected output: `go: added github.com/wneessen/go-mail vX.Y.Z`

- [ ] **Step 2: Replace `internal/digest/smtp.go`** with the real go-mail sender

```go
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
```

- [ ] **Step 3: Run tests to confirm no regressions**

```bash
go test ./internal/digest/... -count=1 -v 2>&1 | tail -15
```

Expected: all 11 tests still PASS (none of them invoke `newSMTPSender`).

- [ ] **Step 4: Replace `main.go`** with subcommand dispatch

```go
// Package main is the entrypoint for aurelianprm.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/astromechza/aurelianprm/internal/dal"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/astromechza/aurelianprm/internal/digest"
	"github.com/astromechza/aurelianprm/internal/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "send-digest" {
		return runDigest()
	}
	return runServe()
}

func runServe() error {
	dbPath := flag.String("db", "aurelianprm.db", "path to SQLite database file")
	addr := flag.String("addr", "", "HTTP listen address (overrides PORT env var)")
	flag.Parse()

	listenAddr := *addr
	if listenAddr == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		listenAddr = ":" + port
	}

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close() //nolint:errcheck

	srv, err := web.NewServer(dal.New(sqlDB, *dbPath))
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	fmt.Fprintf(os.Stderr, "listening on %s\n", listenAddr)
	return http.ListenAndServe(listenAddr, srv.Handler())
}

func runDigest() error {
	fs := flag.NewFlagSet("send-digest", flag.ContinueOnError)
	dbPath := fs.String("db", "aurelianprm.db", "path to SQLite database file")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	cfg, err := digestConfigFromEnv()
	if err != nil {
		return err
	}

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close() //nolint:errcheck

	return digest.SendDigest(context.Background(), dal.New(sqlDB, *dbPath), cfg)
}

// digestConfigFromEnv reads SMTP and recipient config from environment variables.
// Returns an error listing all missing required variables.
func digestConfigFromEnv() (digest.Config, error) {
	var missing []string
	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	port := 587
	if v := os.Getenv("SMTP_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return digest.Config{}, fmt.Errorf("SMTP_PORT must be an integer: %w", err)
		}
		port = p
	}

	cfg := digest.Config{
		SMTPHost: require("SMTP_HOST"),
		SMTPPort: port,
		SMTPUser: require("SMTP_USER"),
		SMTPPass: require("SMTP_PASS"),
		SMTPFrom: require("SMTP_FROM"),
		SMTPSSL:  os.Getenv("SMTP_SSL") == "true",
		DigestTo: require("DIGEST_EMAIL_ADDRESS"),
	}
	if len(missing) > 0 {
		return digest.Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}
```

- [ ] **Step 5: Build**

```bash
go build ./... 2>&1
```

Expected: success.

- [ ] **Step 6: Run all tests**

```bash
go test ./... -count=1 2>&1 | tail -10
```

Expected: all packages PASS.

- [ ] **Step 7: Run `make verify`**

```bash
make verify
```

Expected: 0 issues.

- [ ] **Step 8: Commit**

```bash
git add internal/digest/smtp.go main.go go.mod go.sum
git commit -m "feat: add go-mail SMTP sender and send-digest subcommand"
```

---

## Self-Review

**Spec coverage check:**
- ✅ `send-digest` subcommand — Task 3
- ✅ Birthday window today+0 to today+6 — Task 1
- ✅ No email when no reminders — Task 2 (`sendDigest` early return)
- ✅ Age shown when `BirthYear > 0` — Task 1
- ✅ Leap day Feb 29 → Mar 1 — Task 1 (Go normalises automatically, test confirms)
- ✅ SMTP env vars (all 7) — Task 3
- ✅ SMTP_SSL for implicit TLS (Fastmail port 465) — Task 3
- ✅ Plain text email — Task 2 (`mail.TypeTextPlain`)
- ✅ Subject format "Reminders for Mon 27 Apr 2026" — Task 2
- ✅ Body format "• Mon 28 Apr — label" — Task 2
- ✅ Sorted by date then label — Task 2
- ✅ Extensibility: `FindReminders` comment shows where to add next source — Task 2
- ✅ Tests: `TestFindBirthdayReminders` table-driven, `TestFindBirthdayReminders_leapDay`, `TestFindReminders_sortedByDateThenLabel`, `TestComposeBody`, `TestSendDigest_noReminders_doesNotSend`, `TestSendDigest_withReminders_sendsEmail` — Tasks 1–2

**Type consistency:** `Reminder{Date, Label}` used identically across `birthday.go`, `digest.go`, and `digest_test.go`. `senderFunc` signature `func(subject, body, from, to string) error` consistent across `digest.go`, `smtp.go`, and test mocks. `Config.SMTPSSL` (bool) used identically in `digest.go` and `smtp.go`.

**No placeholders found.**

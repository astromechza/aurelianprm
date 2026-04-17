# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **FK ordering bug in `handlePersonDelete`**: blurry_date for birthday was deleted before the `people` row FK reference was removed, causing a silent orphan (SQLite FK enforcement blocked the delete, error was swallowed). Fixed by deleting the person first, then cleaning up the orphaned blurry_date.
- **FK ordering bug in `handlePersonUpdate` (clear birthday)**: clearing a birthday called `updateBlurryDate` which deleted the blurry_date row before `UpdatePerson` NULLed the FK, causing a FK violation and HTTP 500. Fixed by using the `applyBlurryDateCreate` + deferred-delete pattern (same as address handlers). Added `TestPersonUpdate_ClearBirthday` to cover this path.

### Added

- **Addresses on person detail page**: inline card display, accordion add-form, inline edit-form row-swap via HTMX, and delete. Current and past addresses (by until date) displayed separately. (`internal/store/person_addresses.go`, `internal/web/addresses.go`, `assets/templates/addresses.html`)
- `internal/store`: `ListPersonAddresses`, `GetPersonAddressRow` — single JOIN query across relationships/addresses/blurry_locations/blurry_dates
- `internal/store`: `UpdateRelationshipDates` — targeted UPDATE of `since_id`/`until_id` on a relationship row
- **Reminder system** (`internal/reminder`): daily email digest of upcoming and catch-up anniversaries/birthdays
  - `digest.go`: `ComputeReminders` — annual recurrence logic with `resolveReminderItem` and `buildRelationshipLabel`
  - `email.go`: `FormatDigest` — plain-text email body with relative date formatting
  - `scheduler.go`: `Scheduler` — goroutine that fires once daily at a configurable time; retries 4× on failure; records send history; cleans up history older than 30 days
  - `reminder_history` table via `migrations/00002_reminder_history.sql`
- **People CRUD**: list, create, view detail, edit, delete (`internal/web/`, `assets/templates/`)
- **Contact methods**: add, edit, delete on person detail page
- **Interactions**: add, edit, delete on person detail page
- **Relationships**: add (person/address/pet/work), edit dates, delete on person detail page
- **Addresses model**: `addresses` table, `store.CreateAddress`, `store.UpdateAddress`, `store.DeleteAddress`
- **Authentication**: session-cookie login/logout with bcrypt password verification
- **SMTP mailer** (`internal/mailer`): send email via configured SMTP relay
- `internal/config`: `ReminderSendTime`, `ReminderTimezone`, `ReminderEmail` fields for reminder configuration
- Project scaffold: Go module (`github.com/astromechza/aurelianprm`), directory structure, `CLAUDE.md`, `.golangci.yml`
- `internal/config`: JSON config loading with strict unknown-field rejection
- `migrations/00001_initial_schema.sql`: full SQLite schema — `blurry_dates`, `blurry_locations`, `people`, `contact_methods`, `addresses`, `pets`, `work`, `interactions`, `relationships`
- `internal/db`: SQLite connection with WAL mode, foreign-key enforcement, busy-timeout, and Goose migration runner
- `cmd/aurelianprm`: CLI entry point with `--version`, `hash-password` (bcrypt cost 12), `serve`, and `send-test-email`

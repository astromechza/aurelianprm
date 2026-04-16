package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/astromechza/aurelianprm/internal/model"
)

const dateOnlyLayout = "2006-01-02"

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
		h.DateStart.Format(dateOnlyLayout),
		h.DateEnd.Format(dateOnlyLayout),
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
		ORDER BY sent_at DESC, rowid DESC LIMIT 1`
	row := db.QueryRowContext(ctx, q)
	h, err := scanReminderHistory(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store.GetLastSuccessfulSend: %w", err)
	}
	return &h, nil
}

// DeleteOldReminderHistory deletes reminder_history rows whose sent_at is before the given time.
func DeleteOldReminderHistory(ctx context.Context, db *sql.DB, before time.Time) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM reminder_history WHERE sent_at < strftime('%Y-%m-%dT%H:%M:%fZ', ?)`,
		before.UTC().Format(timeLayout),
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
	if h.DateStart, err = time.Parse(dateOnlyLayout, dateStart); err != nil {
		return model.ReminderHistory{}, fmt.Errorf("parsing date_start: %w", err)
	}
	if h.DateEnd, err = time.Parse(dateOnlyLayout, dateEnd); err != nil {
		return model.ReminderHistory{}, fmt.Errorf("parsing date_end: %w", err)
	}
	return h, nil
}

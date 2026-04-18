package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
)

// CreateBlurryDate inserts a new blurry_dates row and returns it with its generated ID.
func CreateBlurryDate(ctx context.Context, db DBTX, d model.BlurryDate) (model.BlurryDate, error) {
	d.ID = newID()
	const q = `
		INSERT INTO blurry_dates (id, year, month, day, reminder_enabled, reminder_days_before)
		VALUES (?, ?, ?, ?, ?, ?)`
	if err := execContext(ctx, db, q,
		d.ID, nullInt(d.Year), nullInt(d.Month), nullInt(d.Day),
		d.ReminderEnabled, d.ReminderDaysBefore,
	); err != nil {
		return model.BlurryDate{}, fmt.Errorf("store.CreateBlurryDate: %w", err)
	}
	return d, nil
}

// GetBlurryDate fetches a single blurry_dates row by ID.
func GetBlurryDate(ctx context.Context, db DBTX, id string) (model.BlurryDate, error) {
	const q = `
		SELECT id, year, month, day, reminder_enabled, reminder_days_before
		FROM blurry_dates WHERE id = ?`
	row := db.QueryRowContext(ctx, q, id)
	d, err := scanBlurryDate(row)
	if err != nil {
		return model.BlurryDate{}, fmt.Errorf("store.GetBlurryDate: %w", err)
	}
	return d, nil
}

// UpdateBlurryDate replaces all mutable fields of a blurry_dates row.
func UpdateBlurryDate(ctx context.Context, db DBTX, d model.BlurryDate) error {
	const q = `
		UPDATE blurry_dates
		SET year = ?, month = ?, day = ?, reminder_enabled = ?, reminder_days_before = ?
		WHERE id = ?`
	res, err := db.ExecContext(ctx, q,
		nullInt(d.Year), nullInt(d.Month), nullInt(d.Day),
		d.ReminderEnabled, d.ReminderDaysBefore, d.ID,
	)
	if err != nil {
		return fmt.Errorf("store.UpdateBlurryDate: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.UpdateBlurryDate: %w", sql.ErrNoRows)
	}
	return nil
}

// DeleteBlurryDate removes a blurry_dates row by ID.
func DeleteBlurryDate(ctx context.Context, db DBTX, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM blurry_dates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store.DeleteBlurryDate: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.DeleteBlurryDate: %w", sql.ErrNoRows)
	}
	return nil
}

func scanBlurryDate(row *sql.Row) (model.BlurryDate, error) {
	var d model.BlurryDate
	var year, month, day sql.NullInt64
	err := row.Scan(&d.ID, &year, &month, &day, &d.ReminderEnabled, &d.ReminderDaysBefore)
	if err != nil {
		return model.BlurryDate{}, err
	}
	d.Year = scanNullInt(year)
	d.Month = scanNullInt(month)
	d.Day = scanNullInt(day)
	return d, nil
}

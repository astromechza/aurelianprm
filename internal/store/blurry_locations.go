package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
)

// CreateBlurryLocation inserts a new blurry_locations row and returns it with its generated ID.
func CreateBlurryLocation(ctx context.Context, db *sql.DB, l model.BlurryLocation) (model.BlurryLocation, error) {
	l.ID = newID()
	const q = `
		INSERT INTO blurry_locations (id, country, city, street, latitude, longitude)
		VALUES (?, ?, ?, ?, ?, ?)`
	if err := execContext(ctx, db, q,
		l.ID, l.Country, nullString(l.City), nullString(l.Street),
		nullFloat(l.Latitude), nullFloat(l.Longitude),
	); err != nil {
		return model.BlurryLocation{}, fmt.Errorf("store.CreateBlurryLocation: %w", err)
	}
	return l, nil
}

// GetBlurryLocation fetches a single blurry_locations row by ID.
func GetBlurryLocation(ctx context.Context, db *sql.DB, id string) (model.BlurryLocation, error) {
	const q = `
		SELECT id, country, city, street, latitude, longitude
		FROM blurry_locations WHERE id = ?`
	row := db.QueryRowContext(ctx, q, id)
	l, err := scanBlurryLocation(row)
	if err != nil {
		return model.BlurryLocation{}, fmt.Errorf("store.GetBlurryLocation: %w", err)
	}
	return l, nil
}

// UpdateBlurryLocation replaces all mutable fields of a blurry_locations row.
func UpdateBlurryLocation(ctx context.Context, db *sql.DB, l model.BlurryLocation) error {
	const q = `
		UPDATE blurry_locations
		SET country = ?, city = ?, street = ?, latitude = ?, longitude = ?
		WHERE id = ?`
	res, err := db.ExecContext(ctx, q,
		l.Country, nullString(l.City), nullString(l.Street),
		nullFloat(l.Latitude), nullFloat(l.Longitude), l.ID,
	)
	if err != nil {
		return fmt.Errorf("store.UpdateBlurryLocation: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.UpdateBlurryLocation: %w", sql.ErrNoRows)
	}
	return nil
}

// DeleteBlurryLocation removes a blurry_locations row by ID.
func DeleteBlurryLocation(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM blurry_locations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store.DeleteBlurryLocation: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.DeleteBlurryLocation: %w", sql.ErrNoRows)
	}
	return nil
}

func scanBlurryLocation(row *sql.Row) (model.BlurryLocation, error) {
	var l model.BlurryLocation
	var city, street sql.NullString
	var lat, lon sql.NullFloat64
	if err := row.Scan(&l.ID, &l.Country, &city, &street, &lat, &lon); err != nil {
		return model.BlurryLocation{}, err
	}
	l.City = scanNullString(city)
	l.Street = scanNullString(street)
	l.Latitude = scanNullFloat(lat)
	l.Longitude = scanNullFloat(lon)
	return l, nil
}

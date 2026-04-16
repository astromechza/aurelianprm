package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
)

// PersonAddressRow is a joined view of a person's address relationship,
// the address record, and its blurry_location. SinceYear/Month and
// UntilYear/Month are populated from linked blurry_dates when present.
type PersonAddressRow struct {
	RelationshipID string
	SinceID        *string // blurry_date ID for move-in date, nil if not set
	UntilID        *string // blurry_date ID for move-out date, nil = current address
	SinceYear      *int
	SinceMonth     *int
	UntilYear      *int
	UntilMonth     *int
	Address        model.Address
	Location       model.BlurryLocation
}

const personAddressSelect = `
	SELECT r.id, r.since_id, r.until_id,
	       a.id, a.location_id, a.label, a.notes, a.created_at,
	       bl.id, bl.country, bl.city, bl.street, bl.latitude, bl.longitude,
	       bd_since.year, bd_since.month,
	       bd_until.year, bd_until.month
	FROM relationships r
	JOIN addresses a ON r.to_id = a.id
	JOIN blurry_locations bl ON a.location_id = bl.id
	LEFT JOIN blurry_dates bd_since ON r.since_id = bd_since.id
	LEFT JOIN blurry_dates bd_until ON r.until_id = bd_until.id`

// ListPersonAddresses returns all address relationships for a person, ordered
// by the relationship's created_at time. Returns an empty slice (not nil) when
// no addresses exist.
func ListPersonAddresses(ctx context.Context, db *sql.DB, personID string) ([]PersonAddressRow, error) {
	q := personAddressSelect + `
	WHERE r.person_id = ? AND r.to_type = 'address'
	ORDER BY r.created_at`
	rows, err := db.QueryContext(ctx, q, personID)
	if err != nil {
		return nil, fmt.Errorf("store.ListPersonAddresses: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	out := []PersonAddressRow{}
	for rows.Next() {
		row, err := scanPersonAddressRow(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("store.ListPersonAddresses: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// GetPersonAddressRow fetches a single address relationship for a person by
// address ID. Returns sql.ErrNoRows (wrapped) if no match is found.
func GetPersonAddressRow(ctx context.Context, db *sql.DB, personID, addrID string) (PersonAddressRow, error) {
	q := personAddressSelect + `
	WHERE r.person_id = ? AND r.to_id = ? AND r.to_type = 'address'`
	row := db.QueryRowContext(ctx, q, personID, addrID)
	par, err := scanPersonAddressRow(row.Scan)
	if err != nil {
		return PersonAddressRow{}, fmt.Errorf("store.GetPersonAddressRow: %w", err)
	}
	return par, nil
}

// scanPersonAddressRow accepts either rows.Scan or row.Scan and populates a PersonAddressRow.
func scanPersonAddressRow(scanFn func(dest ...any) error) (PersonAddressRow, error) {
	var row PersonAddressRow
	var sinceID, untilID, label, notes, city, street sql.NullString
	var lat, lon sql.NullFloat64
	var sinceYear, sinceMonth, untilYear, untilMonth sql.NullInt64
	var createdAt string

	if err := scanFn(
		&row.RelationshipID, &sinceID, &untilID,
		&row.Address.ID, &row.Address.LocationID, &label, &notes, &createdAt,
		&row.Location.ID, &row.Location.Country, &city, &street, &lat, &lon,
		&sinceYear, &sinceMonth, &untilYear, &untilMonth,
	); err != nil {
		return PersonAddressRow{}, err
	}
	row.SinceID = scanNullString(sinceID)
	row.UntilID = scanNullString(untilID)
	row.Address.Label = scanNullString(label)
	row.Address.Notes = scanNullString(notes)
	row.Location.City = scanNullString(city)
	row.Location.Street = scanNullString(street)
	row.Location.Latitude = scanNullFloat(lat)
	row.Location.Longitude = scanNullFloat(lon)
	row.SinceYear = scanNullInt(sinceYear)
	row.SinceMonth = scanNullInt(sinceMonth)
	row.UntilYear = scanNullInt(untilYear)
	row.UntilMonth = scanNullInt(untilMonth)
	var err error
	if row.Address.CreatedAt, err = parseTime(createdAt); err != nil {
		return PersonAddressRow{}, fmt.Errorf("parsing created_at: %w", err)
	}
	return row, nil
}

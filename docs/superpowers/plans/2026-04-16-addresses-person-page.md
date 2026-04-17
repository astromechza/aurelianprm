# Addresses on Person Detail Page — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a HTMX-powered Addresses section to the person detail page: accordion add-form, inline card display, inline edit-form swap, and delete.

**Architecture:** A JOIN query in `internal/store/person_addresses.go` fetches all address data (relationship + address + blurry_location + since/until blurry_date year/month) in one query. Five handlers in `internal/web/addresses.go` serve card, edit-form, create, update, and delete. Two named templates in `assets/templates/addresses.html`. The existing `updateBlurryDate` and `nullableFormValue` helpers in `internal/web/people.go` (same package) are reused.

**Tech Stack:** Go 1.26, SQLite, HTMX, Pico CSS v2, `github.com/stretchr/testify`, `net/http/httptest`.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `internal/store/person_addresses.go` | Create | `PersonAddressRow` type, `ListPersonAddresses`, `GetPersonAddressRow` |
| `internal/store/person_addresses_test.go` | Create | Store tests |
| `internal/store/relationships.go` | Modify | Add `UpdateRelationshipDates` |
| `assets/templates/addresses.html` | Create | `address_card.html` and `address_edit_form.html` named templates |
| `assets/templates/people_detail.html` | Modify | Add Addresses `<article>` section |
| `internal/web/people.go` | Modify | Add `addressCardData`, `addressEditData`, `formatLocation`, `formatMonthYear`, `buildAddressCardData`; update `personDetailData` and `buildPersonDetailData`; add `"strings"` import |
| `internal/web/addresses.go` | Create | 5 handlers: `handleAddressCard`, `handleAddressEditForm`, `handleAddressCreate`, `handleAddressUpdate`, `handleAddressDelete` + private helpers |
| `internal/web/addresses_test.go` | Create | Handler tests |
| `internal/web/server.go` | Modify | Register 5 new routes |

---

## Task 1: Store layer

**Files:**
- Create: `internal/store/person_addresses.go`
- Create: `internal/store/person_addresses_test.go`
- Modify: `internal/store/relationships.go`

### Background

`PersonAddressRow` is a read-only joined view used by the web layer. The JOIN uses LEFT JOINs on `blurry_dates` so that since/until year+month are available for display without separate queries. `UpdateRelationshipDates` is a simple targeted UPDATE added to `relationships.go` to update `since_id`/`until_id` without delete+recreate.

The store test file is in `package store_test`, same as other store tests. It has access to `openTestDB(t)` defined in `internal/store/testhelper_test.go`.

- [ ] **Write failing tests in `internal/store/person_addresses_test.go`**

```go
package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

func TestListPersonAddresses_Empty(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Alice"})
	require.NoError(t, err)

	rows, err := store.ListPersonAddresses(ctx, db, person.ID)
	require.NoError(t, err)
	assert.NotNil(t, rows) // must be [] not nil
	assert.Empty(t, rows)
}

func TestListPersonAddresses_CurrentAndPast(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Bob"})
	require.NoError(t, err)

	// Current address — no until_id.
	city1 := "London"
	loc1, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "UK", City: &city1})
	require.NoError(t, err)
	lbl1 := "home"
	addr1, err := store.CreateAddress(ctx, db, model.Address{LocationID: loc1.ID, Label: &lbl1})
	require.NoError(t, err)
	yr1, mo1 := 2021, 3
	since1, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &yr1, Month: &mo1})
	require.NoError(t, err)
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID, ToType: model.RelationshipTargetAddress,
		ToID: addr1.ID, SinceID: &since1.ID,
	})
	require.NoError(t, err)

	// Past address — with until_id.
	loc2, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "DE"})
	require.NoError(t, err)
	addr2, err := store.CreateAddress(ctx, db, model.Address{LocationID: loc2.ID})
	require.NoError(t, err)
	yr2, mo2, yr2u, mo2u := 2019, 1, 2021, 2
	since2, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &yr2, Month: &mo2})
	require.NoError(t, err)
	until2, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &yr2u, Month: &mo2u})
	require.NoError(t, err)
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID, ToType: model.RelationshipTargetAddress,
		ToID: addr2.ID, SinceID: &since2.ID, UntilID: &until2.ID,
	})
	require.NoError(t, err)

	rows, err := store.ListPersonAddresses(ctx, db, person.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	current := rows[0]
	assert.Equal(t, addr1.ID, current.Address.ID)
	assert.Nil(t, current.UntilID)
	assert.Equal(t, "UK", current.Location.Country)
	assert.Equal(t, &yr1, current.SinceYear)
	assert.Equal(t, &mo1, current.SinceMonth)
	assert.Nil(t, current.UntilYear)

	past := rows[1]
	assert.Equal(t, addr2.ID, past.Address.ID)
	assert.NotNil(t, past.UntilID)
	assert.Equal(t, &yr2u, past.UntilYear)
	assert.Equal(t, &mo2u, past.UntilMonth)
}

func TestGetPersonAddressRow_Found(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Carol"})
	require.NoError(t, err)
	city := "Paris"
	loc, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "FR", City: &city})
	require.NoError(t, err)
	notes := "nice area"
	addr, err := store.CreateAddress(ctx, db, model.Address{LocationID: loc.ID, Notes: &notes})
	require.NoError(t, err)
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID, ToType: model.RelationshipTargetAddress, ToID: addr.ID,
	})
	require.NoError(t, err)

	row, err := store.GetPersonAddressRow(ctx, db, person.ID, addr.ID)
	require.NoError(t, err)
	assert.Equal(t, addr.ID, row.Address.ID)
	assert.Equal(t, "FR", row.Location.Country)
	require.NotNil(t, row.Address.Notes)
	assert.Equal(t, "nice area", *row.Address.Notes)
	assert.Nil(t, row.SinceYear)
	assert.Nil(t, row.UntilYear)
}

func TestGetPersonAddressRow_WrongPerson(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person1, err := store.CreatePerson(ctx, db, model.Person{Name: "Dave"})
	require.NoError(t, err)
	person2, err := store.CreatePerson(ctx, db, model.Person{Name: "Eve"})
	require.NoError(t, err)
	loc, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "US"})
	require.NoError(t, err)
	addr, err := store.CreateAddress(ctx, db, model.Address{LocationID: loc.ID})
	require.NoError(t, err)
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person1.ID, ToType: model.RelationshipTargetAddress, ToID: addr.ID,
	})
	require.NoError(t, err)

	_, err = store.GetPersonAddressRow(ctx, db, person2.ID, addr.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUpdateRelationshipDates(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Frank"})
	require.NoError(t, err)
	loc, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "AU"})
	require.NoError(t, err)
	addr, err := store.CreateAddress(ctx, db, model.Address{LocationID: loc.ID})
	require.NoError(t, err)
	rel, err := store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID, ToType: model.RelationshipTargetAddress, ToID: addr.ID,
	})
	require.NoError(t, err)
	assert.Nil(t, rel.SinceID)

	yr, mo := 2020, 5
	since, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &yr, Month: &mo})
	require.NoError(t, err)

	require.NoError(t, store.UpdateRelationshipDates(ctx, db, rel.ID, &since.ID, nil))

	row, err := store.GetPersonAddressRow(ctx, db, person.ID, addr.ID)
	require.NoError(t, err)
	assert.Equal(t, &yr, row.SinceYear)
	assert.Equal(t, &mo, row.SinceMonth)
	assert.Nil(t, row.UntilYear)
}
```

- [ ] **Run tests to confirm they fail**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
go test ./internal/store/... -run "TestListPersonAddresses|TestGetPersonAddressRow|TestUpdateRelationshipDates" -v 2>&1 | tail -15
```

Expected: compile error — `store.ListPersonAddresses`, `store.GetPersonAddressRow`, `store.UpdateRelationshipDates` not defined yet.

- [ ] **Create `internal/store/person_addresses.go`**

```go
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
```

- [ ] **Add `UpdateRelationshipDates` to `internal/store/relationships.go`**

Append before the final `scanRelationships` function:

```go
// UpdateRelationshipDates updates the since_id and until_id foreign keys on a
// relationships row without changing any other fields.
func UpdateRelationshipDates(ctx context.Context, db *sql.DB, id string, sinceID, untilID *string) error {
	res, err := db.ExecContext(ctx,
		`UPDATE relationships SET since_id = ?, until_id = ? WHERE id = ?`,
		nullString(sinceID), nullString(untilID), id,
	)
	if err != nil {
		return fmt.Errorf("store.UpdateRelationshipDates: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.UpdateRelationshipDates: %w", sql.ErrNoRows)
	}
	return nil
}
```

- [ ] **Run tests to confirm they pass**

```bash
go test ./internal/store/... -run "TestListPersonAddresses|TestGetPersonAddressRow|TestUpdateRelationshipDates" -v 2>&1 | tail -15
```

Expected: all PASS.

- [ ] **Run full store test suite**

```bash
go test ./internal/store/... -race 2>&1
```

Expected: all PASS, no races.

- [ ] **Commit**

```bash
git add internal/store/person_addresses.go internal/store/person_addresses_test.go \
        internal/store/relationships.go
git commit -m "feat: add ListPersonAddresses, GetPersonAddressRow, UpdateRelationshipDates"
```

---

## Task 2: Template data types, templates, and people_detail.html

**Files:**
- Modify: `internal/web/people.go`
- Create: `assets/templates/addresses.html`
- Modify: `assets/templates/people_detail.html`

### Background

Template data types and helper functions go in `people.go` because they are in the same `web` package as the other detail-page types (`personDetailData`, `relationshipRow`, etc.). `addresses.go` (created in Task 3) is in the same package and calls these helpers directly.

`formatMonthYear` reuses the existing `formatBlurryDate` helper (also in `people.go`) by constructing a temporary `model.BlurryDate` with just year+month.

- [ ] **Update `internal/web/people.go`**

Add `"strings"` to the import block:

```go
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)
```

Add new types after the existing `personFormData` struct:

```go
// addressCardData is passed to address_card.html.
type addressCardData struct {
	PersonID        string
	Row             store.PersonAddressRow
	LocationDisplay string // e.g. "42 Baker St, London, UK"
	SinceDisplay    string // e.g. "March 2021", empty if not set
	UntilDisplay    string // e.g. "December 2023", empty if not set
}

// addressEditData is passed to address_edit_form.html.
type addressEditData struct {
	PersonID  string
	Row       store.PersonAddressRow
	FromYear  string // pre-formatted for <input value>, e.g. "2021" or ""
	FromMonth string // pre-formatted for <input value>, e.g. "3" or ""
	ToYear    string
	ToMonth   string
	Error     string
}
```

Update `personDetailData` to add address fields:

```go
// personDetailData is passed to people_detail.html.
type personDetailData struct {
	Person           model.Person
	BirthdayDisplay  string // pre-formatted, empty if no birthday
	ContactMethods   []model.ContactMethod
	Interactions     []model.Interaction
	Relationships    []relationshipRow
	CurrentAddresses []addressCardData // relationships where until_id IS NULL
	PastAddresses    []addressCardData // relationships where until_id IS NOT NULL
}
```

Update the `data` initialisation in `buildPersonDetailData` to initialise the new slices:

```go
data := personDetailData{
	Person:           p,
	ContactMethods:   []model.ContactMethod{},
	Interactions:     []model.Interaction{},
	Relationships:    []relationshipRow{},
	CurrentAddresses: []addressCardData{},
	PastAddresses:    []addressCardData{},
}
```

Add address loading at the end of `buildPersonDetailData` (before the final `return data, nil`):

```go
addrs, err := store.ListPersonAddresses(ctx, s.db, p.ID)
if err != nil {
	return personDetailData{}, fmt.Errorf("loading addresses: %w", err)
}
for _, addr := range addrs {
	card := buildAddressCardData(p.ID, addr)
	if addr.UntilID == nil {
		data.CurrentAddresses = append(data.CurrentAddresses, card)
	} else {
		data.PastAddresses = append(data.PastAddresses, card)
	}
}
```

Add helper functions at the bottom of `people.go`, after `formatBlurryDate`:

```go
// formatLocation builds a comma-separated display string for a BlurryLocation,
// omitting blank optional parts.
func formatLocation(loc model.BlurryLocation) string {
	var parts []string
	if loc.Street != nil && *loc.Street != "" {
		parts = append(parts, *loc.Street)
	}
	if loc.City != nil && *loc.City != "" {
		parts = append(parts, *loc.City)
	}
	parts = append(parts, loc.Country)
	return strings.Join(parts, ", ")
}

// formatMonthYear returns a human-readable month+year string (e.g. "March 2021").
// Returns an empty string if both inputs are nil.
func formatMonthYear(year, month *int) string {
	if year == nil && month == nil {
		return ""
	}
	return formatBlurryDate(model.BlurryDate{Year: year, Month: month})
}

// buildAddressCardData assembles display data for the address_card.html template.
func buildAddressCardData(personID string, row store.PersonAddressRow) addressCardData {
	return addressCardData{
		PersonID:        personID,
		Row:             row,
		LocationDisplay: formatLocation(row.Location),
		SinceDisplay:    formatMonthYear(row.SinceYear, row.SinceMonth),
		UntilDisplay:    formatMonthYear(row.UntilYear, row.UntilMonth),
	}
}
```

- [ ] **Create `assets/templates/addresses.html`**

```html
{{define "address_card.html"}}
<article>
  <div class="flex-between">
    <div>
      {{if .Row.Address.Label}}<strong>{{deref .Row.Address.Label}}</strong> &mdash; {{end}}{{.LocationDisplay}}
      {{if or .SinceDisplay .UntilDisplay}}
      <br><small>
        {{- if and .SinceDisplay .UntilDisplay}}{{.SinceDisplay}} &ndash; {{.UntilDisplay}}
        {{- else if .SinceDisplay}}since {{.SinceDisplay}}
        {{- else}}until {{.UntilDisplay}}
        {{- end}}
      </small>
      {{end}}
      {{if .Row.Address.Notes}}<br><small>{{deref .Row.Address.Notes}}</small>{{end}}
    </div>
    <div class="flex-gap">
      <button hx-get="/people/{{.PersonID}}/addresses/{{.Row.Address.ID}}/edit"
              hx-target="closest article"
              hx-swap="outerHTML"
              class="outline">Edit</button>
      <button hx-delete="/people/{{.PersonID}}/addresses/{{.Row.Address.ID}}"
              hx-target="closest article"
              hx-swap="outerHTML"
              hx-confirm="Remove this address? This cannot be undone."
              class="outline secondary">&#x2715;</button>
    </div>
  </div>
</article>
{{end}}

{{define "address_edit_form.html"}}
<article>
  {{if .Error}}<p><mark>{{.Error}}</mark></p>{{end}}
  <form hx-post="/people/{{.PersonID}}/addresses/{{.Row.Address.ID}}/edit"
        hx-target="closest article"
        hx-swap="outerHTML">
    <div class="grid">
      <label>Country *
        <input name="country" value="{{.Row.Location.Country}}" required>
      </label>
      <label>City
        <input name="city" value="{{deref .Row.Location.City}}">
      </label>
    </div>
    <label>Street
      <input name="street" value="{{deref .Row.Location.Street}}">
    </label>
    <div class="grid">
      <label>Label
        <input name="label" value="{{deref .Row.Address.Label}}" placeholder="e.g. home, work">
      </label>
      <label>Notes
        <input name="notes" value="{{deref .Row.Address.Notes}}">
      </label>
    </div>
    <fieldset>
      <legend>Dates (optional)</legend>
      <div class="grid">
        <label>Move-in year
          <input type="number" name="from_year" min="1900" max="2100" value="{{.FromYear}}">
        </label>
        <label>Move-in month
          <input type="number" name="from_month" min="1" max="12" value="{{.FromMonth}}">
        </label>
        <label>Move-out year
          <input type="number" name="to_year" min="1900" max="2100" value="{{.ToYear}}">
        </label>
        <label>Move-out month
          <input type="number" name="to_month" min="1" max="12" value="{{.ToMonth}}">
        </label>
      </div>
    </fieldset>
    <div class="flex-gap">
      <button type="submit">Save</button>
      <button type="button"
              hx-get="/people/{{.PersonID}}/addresses/{{.Row.Address.ID}}"
              hx-target="closest article"
              hx-swap="outerHTML"
              class="outline secondary">Cancel</button>
    </div>
  </form>
</article>
{{end}}
```

- [ ] **Update `assets/templates/people_detail.html`**

Insert the Addresses `<article>` between the birthday line and the Contact Methods article. Find this block:

```html
{{if .BirthdayDisplay}}<p><strong>Birthday:</strong> {{.BirthdayDisplay}}</p>{{end}}

<article>
  <header><strong>Contact Methods</strong></header>
```

Replace with:

```html
{{if .BirthdayDisplay}}<p><strong>Birthday:</strong> {{.BirthdayDisplay}}</p>{{end}}

<article>
  <header><strong>Addresses</strong></header>
  <div id="addresses-current">
    {{range .CurrentAddresses}}{{template "address_card.html" .}}{{end}}
  </div>
  {{if .PastAddresses}}
  <details>
    <summary>Past addresses ({{len .PastAddresses}})</summary>
    <div id="addresses-past">
      {{range .PastAddresses}}{{template "address_card.html" .}}{{end}}
    </div>
  </details>
  {{end}}
  <details>
    <summary>Add address</summary>
    <form hx-post="/people/{{.Person.ID}}/addresses"
          hx-target="#addresses-current"
          hx-swap="beforeend"
          hx-on::after-request="this.reset()">
      <div class="grid">
        <label>Country *
          <input name="country" required>
        </label>
        <label>City
          <input name="city">
        </label>
      </div>
      <label>Street
        <input name="street">
      </label>
      <div class="grid">
        <label>Label
          <input name="label" placeholder="e.g. home, work">
        </label>
        <label>Notes
          <input name="notes">
        </label>
      </div>
      <fieldset>
        <legend>Dates (optional)</legend>
        <div class="grid">
          <label>Move-in year
            <input type="number" name="from_year" min="1900" max="2100">
          </label>
          <label>Move-in month
            <input type="number" name="from_month" min="1" max="12">
          </label>
          <label>Move-out year
            <input type="number" name="to_year" min="1900" max="2100">
          </label>
          <label>Move-out month
            <input type="number" name="to_month" min="1" max="12">
          </label>
        </div>
      </fieldset>
      <button type="submit">Add</button>
    </form>
  </details>
</article>

<article>
  <header><strong>Contact Methods</strong></header>
```

- [ ] **Verify build**

```bash
go build ./... 2>&1
```

Expected: clean build. Template parse errors are caught by the existing template unit test in the test suite.

- [ ] **Commit**

```bash
git add internal/web/people.go assets/templates/addresses.html \
        assets/templates/people_detail.html
git commit -m "feat: add address template data types, card/edit templates, and addresses section to person detail"
```

---

## Task 3: Handlers, routes, and handler tests

**Files:**
- Create: `internal/web/addresses.go`
- Create: `internal/web/addresses_test.go`
- Modify: `internal/web/server.go`

### Background

All five handlers live in `internal/web/addresses.go` in `package web`. They have access to `updateBlurryDate`, `nullableFormValue`, `buildAddressCardData`, `addressCardData`, `addressEditData` (all in `people.go`, same package).

The `createPersonForTest` and `extractAddrID` helpers in the test file are private to the test file. `newWebTestServer`, `doWeb`, and `doHTMX` are defined in `people_test.go` (same `web_test` package) and are available here without re-declaration.

- [ ] **Register routes in `internal/web/server.go`**

After the `protected.HandleFunc("DELETE /relationships/{id}", ...)` line, add:

```go
// Address routes
protected.HandleFunc("GET /people/{id}/addresses/{addr_id}", s.handleAddressCard)
protected.HandleFunc("GET /people/{id}/addresses/{addr_id}/edit", s.handleAddressEditForm)
protected.HandleFunc("POST /people/{id}/addresses/{addr_id}/edit", s.handleAddressUpdate)
protected.HandleFunc("POST /people/{id}/addresses", s.handleAddressCreate)
protected.HandleFunc("DELETE /people/{id}/addresses/{addr_id}", s.handleAddressDelete)
```

- [ ] **Verify build fails with missing handler references**

```bash
go build ./... 2>&1
```

Expected: compile error — handler methods not defined yet.

- [ ] **Create `internal/web/addresses.go`**

```go
package web

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

// handleAddressCard returns an address_card.html fragment.
// Called by the Cancel button on the edit form to restore the card display.
func (s *Server) handleAddressCard(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	addrID := r.PathValue("addr_id")

	row, err := store.GetPersonAddressRow(r.Context(), s.db, personID, addrID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "address_card.html", buildAddressCardData(personID, row))
}

// handleAddressEditForm returns the inline edit form for an address.
func (s *Server) handleAddressEditForm(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	addrID := r.PathValue("addr_id")

	row, err := store.GetPersonAddressRow(r.Context(), s.db, personID, addrID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "address_edit_form.html", buildAddressEditData(personID, row, ""))
}

// handleAddressCreate creates a new address entry for a person.
// On success returns an address_card.html fragment for HTMX to append.
func (s *Server) handleAddressCreate(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	country := r.FormValue("country")
	if country == "" {
		http.Error(w, "country is required", http.StatusUnprocessableEntity)
		return
	}

	loc, err := store.CreateBlurryLocation(r.Context(), s.db, model.BlurryLocation{
		Country: country,
		City:    nullableFormValue(r, "city"),
		Street:  nullableFormValue(r, "street"),
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	addr, err := store.CreateAddress(r.Context(), s.db, model.Address{
		LocationID: loc.ID,
		Label:      nullableFormValue(r, "label"),
		Notes:      nullableFormValue(r, "notes"),
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sinceBD := parseMonthYearForm(r, "from_year", "from_month")
	sinceID, err := createOptionalBlurryDate(r, s, sinceBD)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	untilBD := parseMonthYearForm(r, "to_year", "to_month")
	untilID, err := createOptionalBlurryDate(r, s, untilBD)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rel, err := store.CreateRelationship(r.Context(), s.db, model.Relationship{
		PersonID: personID,
		ToType:   model.RelationshipTargetAddress,
		ToID:     addr.ID,
		SinceID:  sinceID,
		UntilID:  untilID,
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	row := store.PersonAddressRow{
		RelationshipID: rel.ID,
		SinceID:        sinceID,
		UntilID:        untilID,
		Address:        addr,
		Location:       loc,
	}
	if sinceBD != nil {
		row.SinceYear = sinceBD.Year
		row.SinceMonth = sinceBD.Month
	}
	if untilBD != nil {
		row.UntilYear = untilBD.Year
		row.UntilMonth = untilBD.Month
	}
	s.templates.Render(w, r, "address_card.html", buildAddressCardData(personID, row))
}

// handleAddressUpdate saves edits to an address entry.
// Returns an address_card.html fragment on success.
func (s *Server) handleAddressUpdate(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	addrID := r.PathValue("addr_id")

	row, err := store.GetPersonAddressRow(r.Context(), s.db, personID, addrID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	country := r.FormValue("country")
	if country == "" {
		s.templates.Render(w, r, "address_edit_form.html", buildAddressEditData(personID, row, "Country is required."))
		return
	}

	loc := row.Location
	loc.Country = country
	loc.City = nullableFormValue(r, "city")
	loc.Street = nullableFormValue(r, "street")
	if err := store.UpdateBlurryLocation(r.Context(), s.db, loc); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	addr := row.Address
	addr.Label = nullableFormValue(r, "label")
	addr.Notes = nullableFormValue(r, "notes")
	if err := store.UpdateAddress(r.Context(), s.db, addr); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	newSinceID, err := updateBlurryDate(r.Context(), s.db,
		parseMonthYearForm(r, "from_year", "from_month"), row.SinceID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	newUntilID, err := updateBlurryDate(r.Context(), s.db,
		parseMonthYearForm(r, "to_year", "to_month"), row.UntilID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := store.UpdateRelationshipDates(r.Context(), s.db, row.RelationshipID, newSinceID, newUntilID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	updated, err := store.GetPersonAddressRow(r.Context(), s.db, personID, addrID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "address_card.html", buildAddressCardData(personID, updated))
}

// handleAddressDelete removes an address entry (relationship + blurry_dates +
// address + blurry_location).
func (s *Server) handleAddressDelete(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	addrID := r.PathValue("addr_id")

	row, err := store.GetPersonAddressRow(r.Context(), s.db, personID, addrID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := store.DeleteRelationship(r.Context(), s.db, row.RelationshipID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if row.SinceID != nil {
		_ = store.DeleteBlurryDate(r.Context(), s.db, *row.SinceID)
	}
	if row.UntilID != nil {
		_ = store.DeleteBlurryDate(r.Context(), s.db, *row.UntilID)
	}
	if err := store.DeleteAddress(r.Context(), s.db, addrID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.DeleteBlurryLocation(r.Context(), s.db, row.Location.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// parseMonthYearForm parses year and month from two form fields into a
// BlurryDate. Returns nil if both fields are blank.
func parseMonthYearForm(r *http.Request, yearKey, monthKey string) *model.BlurryDate {
	yearStr := r.FormValue(yearKey)
	monthStr := r.FormValue(monthKey)
	if yearStr == "" && monthStr == "" {
		return nil
	}
	bd := &model.BlurryDate{}
	if v, err := strconv.Atoi(yearStr); err == nil {
		bd.Year = &v
	}
	if v, err := strconv.Atoi(monthStr); err == nil {
		bd.Month = &v
	}
	return bd
}

// buildAddressEditData assembles data for the address_edit_form.html template.
func buildAddressEditData(personID string, row store.PersonAddressRow, errMsg string) addressEditData {
	d := addressEditData{PersonID: personID, Row: row, Error: errMsg}
	if row.SinceYear != nil {
		d.FromYear = strconv.Itoa(*row.SinceYear)
	}
	if row.SinceMonth != nil {
		d.FromMonth = strconv.Itoa(*row.SinceMonth)
	}
	if row.UntilYear != nil {
		d.ToYear = strconv.Itoa(*row.UntilYear)
	}
	if row.UntilMonth != nil {
		d.ToMonth = strconv.Itoa(*row.UntilMonth)
	}
	return d
}

// createOptionalBlurryDate creates a blurry_date row if bd is non-nil and
// returns its ID. Returns nil, nil when bd is nil.
func createOptionalBlurryDate(r *http.Request, s *Server, bd *model.BlurryDate) (*string, error) {
	if bd == nil {
		return nil, nil
	}
	created, err := store.CreateBlurryDate(r.Context(), s.db, *bd)
	if err != nil {
		return nil, err
	}
	return &created.ID, nil
}
```

- [ ] **Verify build is clean**

```bash
go build ./... 2>&1
```

Expected: no errors.

- [ ] **Write handler tests in `internal/web/addresses_test.go`**

```go
package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createPersonForTest creates a person via the form and returns its ID.
func createPersonForTest(t *testing.T, srv *Server, name string) string {
	t.Helper()
	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {name}})
	require.Equal(t, http.StatusSeeOther, rec.Code)
	loc := rec.Header().Get("Location")
	parts := strings.Split(loc, "/")
	return parts[len(parts)-1]
}

// extractAddrID parses the address ID out of an address_card.html fragment by
// looking for the hx-get edit URL pattern.
func extractAddrID(t *testing.T, body, personID string) string {
	t.Helper()
	prefix := fmt.Sprintf(`hx-get="/people/%s/addresses/`, personID)
	idx := strings.Index(body, prefix)
	require.True(t, idx >= 0, "could not find address ID in response body")
	start := idx + len(prefix)
	end := strings.Index(body[start:], "/")
	require.True(t, end >= 0, "could not find end of address ID")
	return body[start : start+end]
}

func TestAddressCreate_Success(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Alice")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country": {"UK"},
		"city":    {"London"},
		"label":   {"home"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "London, UK")
	assert.Contains(t, body, "home")

	// Person detail page shows the address.
	detail := doWeb(t, srv, http.MethodGet, "/people/"+personID, nil)
	assert.Contains(t, detail.Body.String(), "London, UK")
}

func TestAddressCreate_WithDates(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Bob")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country":    {"DE"},
		"city":       {"Berlin"},
		"from_year":  {"2020"},
		"from_month": {"3"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Berlin, DE")
	assert.Contains(t, body, "2020")
}

func TestAddressCreate_MissingCountry(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Carol")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"city": {"London"},
	})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestAddressEditForm_Loads(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Dave")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country": {"FR"},
		"city":    {"Paris"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), personID)

	rec = doHTMX(t, srv, http.MethodGet, "/people/"+personID+"/addresses/"+addrID+"/edit", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `value="FR"`)
	assert.Contains(t, body, `value="Paris"`)
}

func TestAddressUpdate_Success(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Eve")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country": {"UK"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), personID)

	rec = doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses/"+addrID+"/edit", url.Values{
		"country":    {"DE"},
		"city":       {"Berlin"},
		"label":      {"work"},
		"from_year":  {"2022"},
		"from_month": {"6"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Berlin, DE")
	assert.Contains(t, body, "work")
	assert.Contains(t, body, "2022")
}

func TestAddressUpdate_ClearsDate(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Frank")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country":    {"AU"},
		"from_year":  {"2019"},
		"from_month": {"1"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), personID)

	// Update without from_year/from_month — clears the date.
	rec = doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses/"+addrID+"/edit", url.Values{
		"country": {"AU"},
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "2019")
}

func TestAddressDelete_Success(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Grace")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country": {"NZ"},
		"city":    {"Auckland"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), personID)

	rec = doHTMX(t, srv, http.MethodDelete, "/people/"+personID+"/addresses/"+addrID, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	detail := doWeb(t, srv, http.MethodGet, "/people/"+personID, nil)
	assert.NotContains(t, detail.Body.String(), "Auckland")
}

func TestAddressCard_CancelRestoresCard(t *testing.T) {
	srv := newWebTestServer(t)
	personID := createPersonForTest(t, srv, "Heidi")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/addresses", url.Values{
		"country": {"CA"},
		"city":    {"Toronto"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), personID)

	rec = doHTMX(t, srv, http.MethodGet, "/people/"+personID+"/addresses/"+addrID, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Toronto, CA")
}

func TestAddressDelete_WrongPerson_NotFound(t *testing.T) {
	srv := newWebTestServer(t)
	person1ID := createPersonForTest(t, srv, "Ivan")
	person2ID := createPersonForTest(t, srv, "Judy")

	rec := doHTMX(t, srv, http.MethodPost, "/people/"+person1ID+"/addresses", url.Values{
		"country": {"JP"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	addrID := extractAddrID(t, rec.Body.String(), person1ID)

	rec = doHTMX(t, srv, http.MethodDelete, "/people/"+person2ID+"/addresses/"+addrID, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
```

- [ ] **Run handler tests**

```bash
go test ./internal/web/... -run "TestAddress" -v 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Run full test suite**

```bash
go test -buildvcs=false -race -count=1 ./... 2>&1
```

Expected: all packages PASS.

- [ ] **Run linter**

```bash
golangci-lint run 2>&1
```

Expected: 0 issues. Common fixes if they appear:
- `godot`: exported function comments must end with a period.
- `unused`: check that `createOptionalBlurryDate` helper is called.

- [ ] **Commit**

```bash
git add internal/web/addresses.go internal/web/addresses_test.go \
        internal/web/server.go
git commit -m "feat: address handlers, routes, and tests for person detail page"
```

---

## Task 4: Final verification

- [ ] **Full build**

```bash
go build ./... 2>&1
```

Expected: clean.

- [ ] **Full test suite with race detector**

```bash
go test -buildvcs=false -race -count=1 ./... 2>&1
```

Expected: all PASS.

- [ ] **Lint**

```bash
golangci-lint run 2>&1
```

Expected: 0 issues.

- [ ] **Manual smoke test** (optional but recommended)

```bash
go run ./cmd/aurelianprm serve --config ./config.dev.json
```

Visit a person detail page, open the "Add address" accordion, add an address with country/city/label and move-in year+month. Verify the card appears in "Addresses (current)". Click Edit, change the city, click Save — card updates inline. Click ✕ — card removed. Add another and set a move-out date — it appears in "Past addresses".

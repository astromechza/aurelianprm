# Addresses on Person Detail Page — Design Spec

**Date:** 2026-04-16
**Status:** Approved

## Goal

Show a person's current and past addresses on their detail page, with the ability to add, edit inline, and delete each address entry. The add form lives in a `<details>` accordion; editing swaps the card with an inline form via HTMX.

---

## Data Model

One address entry for a person spans three existing tables — no new migrations required:

| Table | Role |
|---|---|
| `blurry_locations` | Physical location: `country` (required), `city`, `street` |
| `addresses` | Labelled wrapper: `location_id`, `label`, `notes` |
| `relationships` | Links person → address via `to_type = 'address'`, with optional `since_id` / `until_id` (blurry_dates) |

Move-in/move-out dates are stored as `blurry_dates` with `year` and `month` set, `day` always null (UI collects year + month only). Latitude/longitude fields on `blurry_locations` are not exposed in the UI.

**Current address:** relationship where `until_id IS NULL`.
**Past address:** relationship where `until_id IS NOT NULL`.

---

## Store Layer

### New file: `internal/store/person_addresses.go`

**New type:**

```go
type PersonAddressRow struct {
    RelationshipID string
    SinceID        *string // blurry_date ID, nil if not set
    UntilID        *string // blurry_date ID, nil = current address
    Address        model.Address
    Location       model.BlurryLocation
}
```

**New function:**

```go
func ListPersonAddresses(ctx context.Context, db *sql.DB, personID string) ([]PersonAddressRow, error)
```

Single JOIN query across `relationships`, `addresses`, and `blurry_locations`, filtered to `person_id = ? AND to_type = 'address'`, ordered by `relationships.created_at`. Returns an empty slice (not nil) when no addresses exist.

All other store operations reuse existing functions:
- `CreateBlurryLocation`, `GetBlurryLocation`, `UpdateBlurryLocation`, `DeleteBlurryLocation`
- `CreateAddress`, `GetAddress`, `UpdateAddress`, `DeleteAddress`
- `GetBlurryDate`, `CreateBlurryDate`, `UpdateBlurryDate`, `DeleteBlurryDate`
- `CreateRelationship`, `DeleteRelationship`

---

## Routes

All routes added to the existing `protected` mux in `internal/web/server.go`:

```
POST   /people/{id}/addresses                    create address
GET    /people/{id}/addresses/{addr_id}          fetch card fragment (used by cancel)
GET    /people/{id}/addresses/{addr_id}/edit     fetch inline edit form fragment
POST   /people/{id}/addresses/{addr_id}/edit     save edit, return updated card
DELETE /people/{id}/addresses/{addr_id}          delete address
```

---

## Handlers

New file: `internal/web/addresses.go`

### `handleAddressCreate` — `POST /people/{id}/addresses`

1. Validate `country` is non-empty; return 422 with error fragment if missing.
2. Parse optional fields: `city`, `street`, `label`, `notes`, `from_year`, `from_month`, `to_year`, `to_month`.
3. Create records in sequence:
   - `store.CreateBlurryLocation` → location
   - `store.CreateAddress` → address (with `location_id`, `label`, `notes`)
   - If `from_year` or `from_month` provided: `store.CreateBlurryDate` → since blurry_date
   - If `to_year` or `to_month` provided: `store.CreateBlurryDate` → until blurry_date
   - `store.CreateRelationship` → relationship (`person_id`, `to_type="address"`, `to_id=address.ID`, optional `since_id`/`until_id`)
4. Build a `PersonAddressRow` from the created records.
5. Return `address_card.html` fragment (HTMX swaps `beforeend` into `#addresses-current` or `#addresses-past` depending on whether until is set).

### `handleAddressCard` — `GET /people/{id}/addresses/{addr_id}`

1. Fetch relationship, address, blurry_location, and since/until blurry_dates.
2. Return `address_card.html` fragment. Used by the Cancel button on the edit form.

### `handleAddressEditForm` — `GET /people/{id}/addresses/{addr_id}/edit`

1. Fetch relationship by querying for `person_id = id AND to_id = addr_id AND to_type = 'address'`. Return 404 if not found.
2. Fetch address, blurry_location, and (if set) since/until blurry_dates.
3. Return `address_edit_form.html` fragment.

### `handleAddressUpdate` — `POST /people/{id}/addresses/{addr_id}/edit`

1. Fetch existing relationship, address, location, and since/until blurry_dates (same as edit form).
2. Validate `country` non-empty; return 422 with error fragment if missing.
3. Update `blurry_location` fields in place via `store.UpdateBlurryLocation`.
4. Update `address` label and notes via `store.UpdateAddress`.
5. Handle since blurry_date:
   - If from fields provided and since exists: `store.UpdateBlurryDate`.
   - If from fields provided and no since: `store.CreateBlurryDate`, then update relationship (via delete + recreate, since relationships have no update function).
   - If from fields cleared and since exists: `store.DeleteBlurryDate`, recreate relationship without `since_id`.
6. Handle until blurry_date with the same create/update/delete logic.
7. Return updated `address_card.html` fragment.

### `handleAddressDelete` — `DELETE /people/{id}/addresses/{addr_id}`

1. Fetch relationship for `person_id = id AND to_id = addr_id AND to_type = 'address'`. Return 404 if not found.
2. Delete in sequence: relationship → (since blurry_date if set) → (until blurry_date if set) → address → blurry_location.
3. Return empty 200. HTMX removes `closest article` (the card wrapper).

---

## Templates

### New file: `assets/templates/addresses.html`

#### `address_card.html`

Wrapped in `<article>` with a data attribute for the addr_id. Displays:
- **Label** in bold (if set), followed by formatted location: `street, city, country` (omitting blank parts).
- Date range: `from MMM YYYY` or `MMM YYYY – MMM YYYY` or `since MMM YYYY` as appropriate. Omitted if neither since nor until is set.
- Notes paragraph (if set).
- Two buttons: **Edit** (`hx-get="/people/{personID}/addresses/{addrID}/edit"`, `hx-target="closest article"`, `hx-swap="outerHTML"`) and **Delete** (`hx-delete`, `hx-target="closest article"`, `hx-swap="outerHTML"`, `hx-confirm="Remove this address? This cannot be undone."`).

#### `address_edit_form.html`

Inline edit form replacing the card. Fields:

| Field | Type | Required |
|---|---|---|
| Country | text input | Yes |
| City | text input | No |
| Street | text input | No |
| Label | text input | No |
| Notes | textarea | No |
| From year | number input | No |
| From month | select 1–12 | No |
| To year | number input | No |
| To month | select 1–12 | No |

- **Save** button: `hx-post="/people/{personID}/addresses/{addrID}/edit"`, `hx-target="closest article"`, `hx-swap="outerHTML"`.
- **Cancel** button: `hx-get="/people/{personID}/addresses/{addrID}"`, `hx-target="closest article"`, `hx-swap="outerHTML"`. This calls the card-only endpoint to restore the display card without saving any changes.

### Changes to `people_detail.html`

New **Addresses** `<article>` section inserted after the birthday/notes block and before Contact Methods:

```
<article>
  <header>Addresses</header>

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

  <details id="add-address-accordion">
    <summary>Add address</summary>
    <form hx-post="/people/{{.Person.ID}}/addresses"
          hx-target="#addresses-current"
          hx-swap="beforeend"
          hx-on::after-request="this.reset()">
      <!-- country, city, street, label, notes, from year+month, to year+month -->
      <button type="submit">Add</button>
    </form>
  </details>
</article>
```

### Template data changes

`personDetailData` in `internal/web/people.go` gains two new fields:

```go
CurrentAddresses []store.PersonAddressRow
PastAddresses    []store.PersonAddressRow
```

`buildPersonDetailData` calls `store.ListPersonAddresses` and splits the result by `UntilID == nil`.

---

## Error Handling

- Missing `country` on create or update: HTTP 422, return a small error fragment the form can display.
- Address not found (wrong person or missing): HTTP 404, plain text response.
- DB errors in any handler: HTTP 500, log error, return plain text.
- Partial failures during multi-step create (e.g. blurry_location created but address insert fails): log and return 500. No explicit rollback — orphaned blurry_location rows are acceptable given the low volume of a PRM.

---

## Testing

Handler tests in `internal/web/addresses_test.go` (package `web_test`, `httptest` pattern matching existing `people_test.go`):

- Create address with all fields → card fragment returned, re-fetching person detail shows address
- Create address missing country → 422
- Edit form loads → returns form fragment with existing values populated
- Update address → card fragment returned with new values
- Update clears from-date → relationship updated, card shows no date
- Delete address → 200, address gone from subsequent person detail load
- Cancel edit restores card
- `ListPersonAddresses` store test with current + past addresses

No need to test `blurry_location` or `address` store functions directly — they are already tested in `internal/store/addresses_pets_work_test.go`.

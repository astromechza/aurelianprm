# Person TODO Flag — Design Spec

**Issue:** [astromechza/aurelianprm#6](https://github.com/astromechza/aurelianprm/issues/6)
**Date:** 2026-05-10

## Problem

No way to flag a person record as incomplete. Users misuse the `nickName` field as a workaround.

## Goal

Add a boolean `todo` flag to Person entities. Flagged persons:
- Show a ⚑ icon next to their name in the `/persons` list
- Appear at the top of the list (before unflagged persons)
- Can be toggled at create and edit time via a checkbox

## Scope

No changes to: person detail page, DAL queries, DB schema, search behaviour.

## Changes

### 1. JSON Schema — `internal/schema/entities/Person.json`

Add optional boolean property:

```json
"todo": { "type": "boolean" }
```

No DB migration needed. Existing rows without the key are treated as `false`.

### 2. View Models — `internal/web/viewmodels.go`

- `PersonData`: add `Todo bool` json tag `"todo"`
- `PersonListItem`: add `Todo bool`

### 3. Handlers — `internal/web/handlers_persons.go`

**Create + Update:** read `r.FormValue("todo")` — value `"on"` means true (standard HTML checkbox), absent means false. Omit the key from JSON when false (consistent with other optional fields).

**List:** after building `[]PersonListItem`, stable-sort so `Todo == true` items come first. Display-name order preserved within each group.

### 4. Templates

**`persons_create.html`** and **`partials/person_edit_form.html`:**

Add checkbox field:
```html
<label>
  <input type="checkbox" name="todo" {{if .PersonData.Todo}}checked{{end}}>
  Flag as TODO (needs revisiting)
</label>
```

Create form has no pre-existing data so `checked` is never set by default.

**`partials/persons_rows.html`:**

Prepend flag icon when `Todo == true`:
```html
<td>{{if .Todo}}⚑ {{end}}{{.Name}}{{if .NickName}} ({{.NickName}}){{end}}</td>
```

## Success Criteria

- Checkbox appears on create and edit forms
- Saving with checkbox checked persists `todo: true` in entity JSON
- Saving with checkbox unchecked omits `todo` key (not stored as `false`)
- Flagged persons appear above unflagged in `/persons` list, icon visible
- Existing persons without the field are unaffected

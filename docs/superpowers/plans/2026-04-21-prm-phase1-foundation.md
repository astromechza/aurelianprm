# PRM Phase 1 — Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish a graph-based SQLite entity store with goose migrations, JSONSchema-validated entity types, a relationship type registry, and a transaction-based DAL with graph-neighbour queries.

**Architecture:** Two tables — `entities` (typed nodes with ULID PKs and JSON data blobs) and `relationships` (typed edges with optional date range and note). Entity data is validated at DAL write time against per-type JSONSchema documents embedded in the binary. Relationship type constraints (symmetric flag, allowed subject/object types) live in a Go registry. All mutations go through `WithTx`.

**Tech Stack:** Go 1.26, `modernc.org/sqlite`, `github.com/pressly/goose/v3`, `github.com/oklog/ulid/v2`, `github.com/santhosh-tekuri/jsonschema/v6`, `github.com/stretchr/testify`

---

## Entity Type Catalogue

| Type | Inspired by | Fields | Required |
|------|-------------|--------|----------|
| `Person` | schema.org/Person, foaf:Person | `name`, `nickName`, `birthYear`, `birthMonth`, `birthDay`, `note` | `name` |
| `PostalAddress` | schema.org/PostalAddress | `label`, `country` (ISO 3166-1 alpha-2), `city`, `streetAddress` | *(none)* |
| `EmailAddress` | schema.org/ContactPoint | `email`, `label` | `email` |
| `PhoneNumber` | schema.org/ContactPoint | `telephone`, `label` | `telephone` |
| `WebSite` | schema.org/WebSite | `url`, `title`, `label` | `url` |
| `SocialNetwork` | custom | `platform`, `handle`, `url` | `platform` |
| `Pet` | schema.org/Animal | `name`, `species`, `birthDate`, `label` | `name` |
| `Career` | custom | `role`, `organization`, `label` | `role`, `organization` |

## Relationship Type Catalogue

| Type | Symmetric | Subject | Object |
|------|-----------|---------|--------|
| `hasAddress` | no | Person | PostalAddress |
| `hasEmail` | no | Person | EmailAddress |
| `hasPhone` | no | Person | PhoneNumber |
| `hasWebSite` | no | Person | WebSite |
| `hasSocialNetwork` | no | Person | SocialNetwork |
| `hasPet` | no | Person | Pet |
| `hasCareer` | no | Person | Career |
| `knows` | yes | Person | Person |
| `friendOf` | yes | Person | Person |
| `spouseOf` | yes | Person | Person |
| `siblingOf` | yes | Person | Person |
| `parentOf` | no | Person | Person |

---

## File Map

| File | Responsibility |
|------|---------------|
| `internal/db/db.go` | Open `*sql.DB`, pragmas, run goose migrations |
| `internal/db/migrations/00001_create_entities.sql` | entities table DDL |
| `internal/db/migrations/00002_create_relationships.sql` | relationships table DDL |
| `internal/db/migrations/00003_create_entities_fts.sql` | FTS5 virtual table + sync triggers |
| `internal/schema/schema.go` | Embed + compile JSONSchema files, `ValidateEntity` |
| `internal/schema/registry.go` | `RelationshipTypeDef` map, `ValidateRelationship` |
| `internal/schema/entities/Person.json` | JSONSchema for Person |
| `internal/schema/entities/PostalAddress.json` | JSONSchema for PostalAddress |
| `internal/schema/entities/EmailAddress.json` | JSONSchema for EmailAddress |
| `internal/schema/entities/PhoneNumber.json` | JSONSchema for PhoneNumber |
| `internal/schema/entities/WebSite.json` | JSONSchema for WebSite |
| `internal/schema/entities/SocialNetwork.json` | JSONSchema for SocialNetwork |
| `internal/schema/entities/Pet.json` | JSONSchema for Pet |
| `internal/schema/entities/Career.json` | JSONSchema for Career |
| `internal/dal/dal.go` | `DAL`, `DBTX` interface, `WithTx` helper |
| `internal/dal/entities.go` | Entity CRUD (`Entity` type, create/get/update/delete/list/search) |
| `internal/dal/relationships.go` | Relationship CRUD + `GetNeighbours`, `GetNeighboursByRelType`, `UpdateRelationship` |
| `internal/dal/dal_test.go` | Integration tests using in-memory SQLite |

---

### Task 1: Add dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add all new packages**

```bash
go get modernc.org/sqlite
go get github.com/pressly/goose/v3
go get github.com/oklog/ulid/v2
go get github.com/santhosh-tekuri/jsonschema/v6
go mod tidy
```

- [ ] **Step 2: Verify**

```bash
make verify
```

Expected: all steps pass.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add sqlite, goose, ulid, and jsonschema dependencies"
```

---

### Task 2: Database open + pragma + migration runner

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/db_test.go`
- Create: `internal/db/migrations/` (empty dir, placeholder for SQL files)

- [ ] **Step 1: Write failing test**

Create `internal/db/db_test.go`:

```go
package db_test

import (
	"testing"

	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/stretchr/testify/require"
)

func TestOpen_pragmas(t *testing.T) {
	conn, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	var journalMode string
	require.NoError(t, conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode))
	require.Equal(t, "wal", journalMode)

	var fkEnabled int
	require.NoError(t, conn.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled))
	require.Equal(t, 1, fkEnabled)
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/db/... -v -run TestOpen_pragmas
```

Expected: compile error — package `db` does not exist.

- [ ] **Step 3: Implement**

```bash
mkdir -p internal/db/migrations
```

Create `internal/db/db.go`:

```go
package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens a SQLite database at path (use ":memory:" for tests), applies
// required pragmas, and runs all pending goose migrations.
func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := conn.Exec(pragma); err != nil {
			conn.Close()
			return nil, fmt.Errorf("apply %q: %w", pragma, err)
		}
	}

	goose.SetLogger(goose.NopLogger())
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, migrationsFS)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("goose provider: %w", err)
	}
	if _, err := provider.Up(nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("goose up: %w", err)
	}
	return conn, nil
}
```

- [ ] **Step 4: Run test**

```bash
go test ./internal/db/... -v -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/db.go internal/db/db_test.go internal/db/migrations
git commit -m "feat: add db.Open with SQLite pragmas and goose migration runner"
```

---

### Task 3: Migration — entities table

**Files:**
- Create: `internal/db/migrations/00001_create_entities.sql`

- [ ] **Step 1: Write migration**

Create `internal/db/migrations/00001_create_entities.sql`:

```sql
-- +goose Up
CREATE TABLE entities (
    id           TEXT     PRIMARY KEY,            -- ULID
    type         TEXT     NOT NULL,
    display_name TEXT,
    data         TEXT     NOT NULL DEFAULT '{}',  -- JSON blob
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX entities_type         ON entities(type);
CREATE INDEX entities_display_name ON entities(display_name)
    WHERE display_name IS NOT NULL;

-- +goose Down
DROP TABLE entities;
```

- [ ] **Step 2: Write test**

Add to `internal/db/db_test.go`:

```go
func TestOpen_entitiesTable(t *testing.T) {
	conn, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	_, err = conn.Exec(
		`INSERT INTO entities (id, type, display_name, data) VALUES ('01JTEST00000000000000000001', 'Person', 'Alice', '{"name":"Alice"}')`,
	)
	require.NoError(t, err)

	var typ, name string
	require.NoError(t, conn.QueryRow(
		`SELECT type, display_name FROM entities WHERE id='01JTEST00000000000000000001'`,
	).Scan(&typ, &name))
	require.Equal(t, "Person", typ)
	require.Equal(t, "Alice", name)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/db/... -v -race
```

Expected: both tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/db/migrations/00001_create_entities.sql internal/db/db_test.go
git commit -m "feat: add entities table migration"
```

---

### Task 4: Migration — relationships table

**Files:**
- Create: `internal/db/migrations/00002_create_relationships.sql`

- [ ] **Step 1: Write migration**

Create `internal/db/migrations/00002_create_relationships.sql`:

```sql
-- +goose Up
CREATE TABLE relationships (
    id          TEXT     PRIMARY KEY,           -- ULID
    entity_a_id TEXT     NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    entity_b_id TEXT     NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    type        TEXT     NOT NULL,
    date_from   TEXT,                           -- partial ISO 8601: YYYY, YYYY-MM, or YYYY-MM-DD
    date_to     TEXT,                           -- partial ISO 8601: YYYY, YYYY-MM, or YYYY-MM-DD
    note        TEXT,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (entity_a_id != entity_b_id),
    UNIQUE (entity_a_id, entity_b_id, type)
);

CREATE INDEX relationships_entity_a ON relationships(entity_a_id);
CREATE INDEX relationships_entity_b ON relationships(entity_b_id);
CREATE INDEX relationships_type     ON relationships(type);

-- +goose Down
DROP TABLE relationships;
```

- [ ] **Step 2: Write test**

Add to `internal/db/db_test.go`:

```go
func TestOpen_relationshipsTable(t *testing.T) {
	conn, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	_, err = conn.Exec(`INSERT INTO entities (id, type, data) VALUES
		('01JTEST00000000000000000002', 'Person', '{}'),
		('01JTEST00000000000000000003', 'EmailAddress', '{}')`)
	require.NoError(t, err)

	_, err = conn.Exec(`INSERT INTO relationships (id, entity_a_id, entity_b_id, type)
		VALUES ('01JTEST00000000000000000004', '01JTEST00000000000000000002', '01JTEST00000000000000000003', 'hasEmail')`)
	require.NoError(t, err)

	// Self-relationship must be rejected by CHECK constraint.
	_, err = conn.Exec(`INSERT INTO relationships (id, entity_a_id, entity_b_id, type)
		VALUES ('01JTEST00000000000000000005', '01JTEST00000000000000000002', '01JTEST00000000000000000002', 'knows')`)
	require.Error(t, err)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/db/... -v -race
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/db/migrations/00002_create_relationships.sql internal/db/db_test.go
git commit -m "feat: add relationships table migration"
```

---

### Task 4b: Migration — FTS5 entity search

**Files:**
- Create: `internal/db/migrations/00003_create_entities_fts.sql`

- [ ] **Step 1: Write migration**

Create `internal/db/migrations/00003_create_entities_fts.sql`:

```sql
-- +goose Up
CREATE VIRTUAL TABLE entities_fts USING fts5(
    display_name,
    entity_id UNINDEXED,
    tokenize = 'porter unicode61'
);

-- Backfill for migrations running against an existing database.
INSERT INTO entities_fts(display_name, entity_id)
    SELECT display_name, id FROM entities WHERE display_name IS NOT NULL;

CREATE TRIGGER entities_fts_insert AFTER INSERT ON entities
    WHEN NEW.display_name IS NOT NULL
BEGIN
    INSERT INTO entities_fts(display_name, entity_id) VALUES (NEW.display_name, NEW.id);
END;

CREATE TRIGGER entities_fts_update AFTER UPDATE OF display_name ON entities
BEGIN
    DELETE FROM entities_fts WHERE entity_id = OLD.id;
    INSERT INTO entities_fts(display_name, entity_id)
        SELECT NEW.display_name, NEW.id WHERE NEW.display_name IS NOT NULL;
END;

CREATE TRIGGER entities_fts_delete AFTER DELETE ON entities
BEGIN
    DELETE FROM entities_fts WHERE entity_id = OLD.id;
END;

-- +goose Down
DROP TRIGGER IF EXISTS entities_fts_delete;
DROP TRIGGER IF EXISTS entities_fts_update;
DROP TRIGGER IF EXISTS entities_fts_insert;
DROP TABLE IF EXISTS entities_fts;
```

- [ ] **Step 2: Write test**

Add to `internal/db/db_test.go`:

```go
func TestOpen_entitiesFTSTable(t *testing.T) {
	conn, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	_, err = conn.Exec(`INSERT INTO entities (id, type, display_name, data)
		VALUES ('01JTEST00000000000000000010', 'Person', 'Alice Smith', '{"name":"Alice Smith"}')`)
	require.NoError(t, err)

	// Trigger should have populated FTS.
	var entityID string
	require.NoError(t, conn.QueryRow(
		`SELECT entity_id FROM entities_fts WHERE display_name MATCH 'alice'`,
	).Scan(&entityID))
	require.Equal(t, "01JTEST00000000000000000010", entityID)
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/db/... -v -race
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/db/migrations/00003_create_entities_fts.sql internal/db/db_test.go
git commit -m "feat: add FTS5 entities search index with porter stemmer"
```

---

### Task 5: Entity JSONSchema files

**Files:**
- Create: `internal/schema/entities/Person.json`
- Create: `internal/schema/entities/PostalAddress.json`
- Create: `internal/schema/entities/EmailAddress.json`
- Create: `internal/schema/entities/PhoneNumber.json`
- Create: `internal/schema/entities/WebSite.json`
- Create: `internal/schema/entities/SocialNetwork.json`
- Create: `internal/schema/entities/Pet.json`
- Create: `internal/schema/entities/Career.json`

- [ ] **Step 1: Create schema directory**

```bash
mkdir -p internal/schema/entities
```

- [ ] **Step 2: Write Person.json**

Create `internal/schema/entities/Person.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "Person",
  "title": "Person",
  "description": "A human being. Inspired by schema.org/Person and foaf:Person.",
  "type": "object",
  "properties": {
    "name":       { "type": "string", "minLength": 1, "description": "Full freeform name" },
    "nickName":   { "type": "string", "minLength": 1, "description": "Nickname or preferred name" },
    "birthYear":  { "type": "integer", "minimum": 1, "maximum": 9999 },
    "birthMonth": { "type": "integer", "minimum": 1, "maximum": 12 },
    "birthDay":   { "type": "integer", "minimum": 1, "maximum": 31 },
    "note":       { "type": "string" }
  },
  "required": ["name"],
  "additionalProperties": false
}
```

- [ ] **Step 3: Write PostalAddress.json**

Create `internal/schema/entities/PostalAddress.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "PostalAddress",
  "title": "PostalAddress",
  "description": "A postal mailing address. Inspired by schema.org/PostalAddress.",
  "type": "object",
  "properties": {
    "label":         { "type": "string", "description": "e.g. home, work, other" },
    "country":       { "type": "string", "minLength": 2, "maxLength": 2, "description": "ISO 3166-1 alpha-2 country code" },
    "city":          { "type": "string" },
    "streetAddress": { "type": "string" }
  },
  "additionalProperties": false
}
```

- [ ] **Step 4: Write EmailAddress.json**

Create `internal/schema/entities/EmailAddress.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "EmailAddress",
  "title": "EmailAddress",
  "description": "An email contact point. Inspired by schema.org/ContactPoint.",
  "type": "object",
  "properties": {
    "email": { "type": "string", "format": "email" },
    "label": { "type": "string", "description": "e.g. work, personal, other" }
  },
  "required": ["email"],
  "additionalProperties": false
}
```

- [ ] **Step 5: Write PhoneNumber.json**

Create `internal/schema/entities/PhoneNumber.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "PhoneNumber",
  "title": "PhoneNumber",
  "description": "A telephone contact point. Inspired by schema.org/ContactPoint.",
  "type": "object",
  "properties": {
    "telephone": { "type": "string", "minLength": 1, "pattern": "^\\+?[\\d\\s\\-().]+$" },
    "label":     { "type": "string", "description": "e.g. mobile, work, home, other" }
  },
  "required": ["telephone"],
  "additionalProperties": false
}
```

- [ ] **Step 6: Write WebSite.json**

Create `internal/schema/entities/WebSite.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "WebSite",
  "title": "WebSite",
  "description": "A website. Inspired by schema.org/WebSite.",
  "type": "object",
  "properties": {
    "url":   { "type": "string", "format": "uri" },
    "title": { "type": "string" },
    "label": { "type": "string", "description": "e.g. personal, work, blog" }
  },
  "required": ["url"],
  "additionalProperties": false
}
```

- [ ] **Step 7: Write SocialNetwork.json**

Create `internal/schema/entities/SocialNetwork.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "SocialNetwork",
  "title": "SocialNetwork",
  "description": "A social network profile.",
  "type": "object",
  "properties": {
    "platform": { "type": "string", "description": "e.g. LinkedIn, GitHub, Bluesky, Mastodon" },
    "handle":   { "type": "string", "description": "Username or handle on the platform" },
    "url":      { "type": "string", "format": "uri" }
  },
  "required": ["platform"],
  "additionalProperties": false
}
```

- [ ] **Step 8: Write Pet.json**

Create `internal/schema/entities/Pet.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "Pet",
  "title": "Pet",
  "description": "A domesticated animal. Inspired by schema.org/Animal.",
  "type": "object",
  "properties": {
    "name":      { "type": "string", "minLength": 1 },
    "species":   { "type": "string", "minLength": 1, "description": "e.g. dog, cat, rabbit" },
    "birthDate": { "type": "string", "format": "date" },
    "label":     { "type": "string" }
  },
  "required": ["name"],
  "additionalProperties": false
}
```

- [ ] **Step 9: Write Career.json**

Create `internal/schema/entities/Career.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "Career",
  "title": "Career",
  "description": "A role or position at an organization.",
  "type": "object",
  "properties": {
    "role":         { "type": "string", "minLength": 1, "description": "Job title or position" },
    "organization": { "type": "string", "minLength": 1, "description": "Name of the organization" },
    "label":        { "type": "string", "description": "e.g. current, former, contract" }
  },
  "required": ["role", "organization"],
  "additionalProperties": false
}
```

- [ ] **Step 10: Commit**

```bash
git add internal/schema/entities/
git commit -m "feat: add JSONSchema definitions for all entity types"
```

---

### Task 6: Schema package — validation and relationship registry

**Files:**
- Create: `internal/schema/schema.go`
- Create: `internal/schema/registry.go`
- Create: `internal/schema/schema_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/schema/schema_test.go`:

```go
package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/astromechza/aurelianprm/internal/schema"
	"github.com/stretchr/testify/require"
)

func TestValidateEntity_valid(t *testing.T) {
	data := json.RawMessage(`{"name":"Alice Smith"}`)
	require.NoError(t, schema.ValidateEntity("Person", data))
}

func TestValidateEntity_missingRequired(t *testing.T) {
	data := json.RawMessage(`{}`) // missing required name
	err := schema.ValidateEntity("Person", data)
	require.Error(t, err)
}

func TestValidateEntity_additionalProperty(t *testing.T) {
	data := json.RawMessage(`{"name":"Alice","unknownField":"oops"}`)
	err := schema.ValidateEntity("Person", data)
	require.Error(t, err)
}

func TestValidateEntity_unknownType(t *testing.T) {
	err := schema.ValidateEntity("Spaceship", json.RawMessage(`{}`))
	require.Error(t, err)
}

func TestValidateEntity_formatEmail(t *testing.T) {
	err := schema.ValidateEntity("EmailAddress", json.RawMessage(`{"email":"not-an-email"}`))
	require.Error(t, err)
}

func TestValidateEntity_formatURI(t *testing.T) {
	err := schema.ValidateEntity("WebSite", json.RawMessage(`{"url":"not a uri"}`))
	require.Error(t, err)
}

func TestValidateEntity_phonePattern(t *testing.T) {
	require.NoError(t, schema.ValidateEntity("PhoneNumber", json.RawMessage(`{"telephone":"+44 20 7946 0958"}`)))
	require.Error(t, schema.ValidateEntity("PhoneNumber", json.RawMessage(`{"telephone":"abc-not-a-phone"}`)))
}

func TestValidateEntity_minLength(t *testing.T) {
	err := schema.ValidateEntity("Person", json.RawMessage(`{"name":""}`))
	require.Error(t, err)
}

func TestKnownEntityTypes(t *testing.T) {
	types := schema.KnownEntityTypes()
	require.Contains(t, types, "Person")
	require.Contains(t, types, "Career")
	require.Len(t, types, 8)
}

func TestValidateRelationship_valid(t *testing.T) {
	require.NoError(t, schema.ValidateRelationship("hasEmail", "Person", "EmailAddress"))
}

func TestValidateRelationship_wrongObjectType(t *testing.T) {
	err := schema.ValidateRelationship("hasEmail", "Person", "Person")
	require.Error(t, err)
}

func TestValidateRelationship_unknownType(t *testing.T) {
	err := schema.ValidateRelationship("hasDragon", "Person", "Pet")
	require.Error(t, err)
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/schema/... -v
```

Expected: compile error — package `schema` does not exist.

- [ ] **Step 3: Implement schema.go**

Create `internal/schema/schema.go`:

```go
package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:generate go run . -- embed check
var entitySchemas map[string]*jsonschema.Schema

func init() {
	c := jsonschema.NewCompiler()
	c.AssertFormat = true // enforce format keywords: email, uri, date, etc.
	entitySchemas = make(map[string]*jsonschema.Schema)

	entries, err := fs.ReadDir(entitySchemasFS, "entities")
	if err != nil {
		panic(fmt.Sprintf("schema: read entities dir: %v", err))
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		data, err := entitySchemasFS.ReadFile("entities/" + e.Name())
		if err != nil {
			panic(fmt.Sprintf("schema: read %s: %v", e.Name(), err))
		}
		if err := c.AddResource("schema:"+name, bytes.NewReader(data)); err != nil {
			panic(fmt.Sprintf("schema: add resource %s: %v", name, err))
		}
		compiled, err := c.Compile("schema:" + name)
		if err != nil {
			panic(fmt.Sprintf("schema: compile %s: %v", name, err))
		}
		entitySchemas[name] = compiled
	}
}

// ValidateEntity validates data against the JSONSchema for the given entity type.
func ValidateEntity(entityType string, data json.RawMessage) error {
	s, ok := entitySchemas[entityType]
	if !ok {
		return fmt.Errorf("unknown entity type %q", entityType)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := s.Validate(v); err != nil {
		return fmt.Errorf("entity %q schema validation failed: %w", entityType, err)
	}
	return nil
}

// KnownEntityTypes returns all registered entity type names in sorted order.
func KnownEntityTypes() []string {
	types := make([]string, 0, len(entitySchemas))
	for t := range entitySchemas {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}
```

Add embed file reference. Create `internal/schema/embed.go`:

```go
package schema

import "embed"

//go:embed entities/*.json
var entitySchemasFS embed.FS
```

- [ ] **Step 4: Implement registry.go**

Create `internal/schema/registry.go`:

```go
package schema

import "fmt"

// RelationshipTypeDef defines constraints for a relationship type.
type RelationshipTypeDef struct {
	Symmetric           bool
	AllowedSubjectTypes []string // empty = any entity type allowed
	AllowedObjectTypes  []string // empty = any entity type allowed
}

// RelationshipTypes is the authoritative registry of valid relationship types.
var RelationshipTypes = map[string]RelationshipTypeDef{
	"hasAddress":       {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"PostalAddress"}},
	"hasEmail":         {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"EmailAddress"}},
	"hasPhone":         {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"PhoneNumber"}},
	"hasWebSite":       {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"WebSite"}},
	"hasSocialNetwork": {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"SocialNetwork"}},
	"hasPet":           {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Pet"}},
	"hasCareer":        {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Career"}},
	"knows":            {Symmetric: true, AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Person"}},
	"friendOf":         {Symmetric: true, AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Person"}},
	"spouseOf":         {Symmetric: true, AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Person"}},
	"siblingOf":        {Symmetric: true, AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Person"}},
	"parentOf":         {AllowedSubjectTypes: []string{"Person"}, AllowedObjectTypes: []string{"Person"}},
}

// ValidateRelationship checks the relationship type exists and that the subject
// and object entity types satisfy its constraints.
func ValidateRelationship(relType, subjectEntityType, objectEntityType string) error {
	def, ok := RelationshipTypes[relType]
	if !ok {
		return fmt.Errorf("unknown relationship type %q", relType)
	}
	if err := checkAllowed(subjectEntityType, def.AllowedSubjectTypes); err != nil {
		return fmt.Errorf("relationship %q: subject type %q not allowed", relType, subjectEntityType)
	}
	if err := checkAllowed(objectEntityType, def.AllowedObjectTypes); err != nil {
		return fmt.Errorf("relationship %q: object type %q not allowed", relType, objectEntityType)
	}
	return nil
}

func checkAllowed(t string, allowed []string) error {
	if len(allowed) == 0 {
		return nil
	}
	for _, a := range allowed {
		if a == t {
			return nil
		}
	}
	return fmt.Errorf("%q not in %v", t, allowed)
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/schema/... -v -race
```

Expected: all tests PASS.

- [ ] **Step 6: Full verify**

```bash
make verify
```

Expected: all steps pass.

- [ ] **Step 7: Commit**

```bash
git add internal/schema/
git commit -m "feat: add schema validation and relationship type registry"
```

---

### Task 7: DAL foundation — DBTX interface and WithTx

**Files:**
- Create: `internal/dal/dal.go`
- Create: `internal/dal/dal_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/dal/dal_test.go`:

```go
package dal_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/astromechza/aurelianprm/internal/dal"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/stretchr/testify/require"
)

func newTestDAL(t *testing.T) *dal.DAL {
	t.Helper()
	conn, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return dal.New(conn)
}

func TestWithTx_commit(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.Exec(ctx, `INSERT INTO entities (id, type, data) VALUES ('01TEST0000000000000000001A', 'Person', '{}')`)
		return err
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, d.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM entities`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestWithTx_rollbackOnError(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		if _, err := q.Exec(ctx, `INSERT INTO entities (id, type, data) VALUES ('01TEST0000000000000000001B', 'Person', '{}')`, ); err != nil {
			return err
		}
		return fmt.Errorf("simulated failure")
	})
	require.Error(t, err)

	var count int
	require.NoError(t, d.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM entities`).Scan(&count))
	require.Equal(t, 0, count)
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/dal/... -v
```

Expected: compile error — package `dal` does not exist.

- [ ] **Step 3: Implement dal.go**

Create `internal/dal/dal.go`:

```go
package dal

import (
	"context"
	"database/sql"
	"fmt"
)

// DBTX is satisfied by both *sql.DB and *sql.Tx.
type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// Queries wraps a DBTX and provides all data access methods.
type Queries struct {
	db DBTX
}

// Exec is a pass-through used in tests.
func (q *Queries) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return q.db.ExecContext(ctx, query, args...)
}

// DAL owns the database connection.
type DAL struct {
	db *sql.DB
}

// New creates a DAL from an open *sql.DB.
func New(db *sql.DB) *DAL {
	return &DAL{db: db}
}

// DB returns the underlying *sql.DB (used in tests).
func (d *DAL) DB() *sql.DB {
	return d.db
}

// WithTx runs fn inside a transaction. Commits on success, rolls back on error.
func (d *DAL) WithTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if err := fn(&Queries{db: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/dal/... -v -race
```

Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dal/dal.go internal/dal/dal_test.go
git commit -m "feat: add DAL foundation with DBTX interface and WithTx"
```

---

### Task 8: Entity CRUD

**Files:**
- Create: `internal/dal/entities.go`
- Modify: `internal/dal/dal_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/dal/dal_test.go`:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/astromechza/aurelianprm/internal/dal"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/stretchr/testify/require"
)

func TestCreateEntity_and_GetEntity(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var created dal.Entity
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		created, err = q.CreateEntity(ctx, dal.CreateEntityParams{
			Type:        "Person",
			DisplayName: ptr("Alice Smith"),
			Data:        json.RawMessage(`{"name":"Alice Smith"}`),
		})
		return err
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "Person", created.Type)
	require.Equal(t, ptr("Alice Smith"), created.DisplayName)

	var fetched dal.Entity
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		fetched, err = q.GetEntity(ctx, created.ID)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, fetched.ID)
}

func TestCreateEntity_validationFails(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person",
			Data: json.RawMessage(`{"unknownField":"oops"}`), // missing required name + extra property
		})
		return err
	})
	require.Error(t, err)
}

func TestUpdateEntity(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var created dal.Entity
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		created, err = q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person",
			Data: json.RawMessage(`{"name":"Bob"}`),
		})
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.UpdateEntity(ctx, dal.UpdateEntityParams{
			ID:          created.ID,
			DisplayName: ptr("Bob Jones"),
			Data:        json.RawMessage(`{"name":"Bob Jones"}`),
		})
	})
	require.NoError(t, err)

	var fetched dal.Entity
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		fetched, err = q.GetEntity(ctx, created.ID)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, ptr("Bob Jones"), fetched.DisplayName)
}

func TestDeleteEntity(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var id string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person",
			Data: json.RawMessage(`{"name":"Carol"}`),
		})
		id = e.ID
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.DeleteEntity(ctx, id)
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.GetEntity(ctx, id)
		return err
	})
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestListEntitiesByType(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		for _, name := range []string{"Dave", "Eve"} {
			if _, err := q.CreateEntity(ctx, dal.CreateEntityParams{
				Type: "Person",
				Data: json.RawMessage(`{"name":"` + name + `"}`),
			}); err != nil {
				return err
			}
		}
		_, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "EmailAddress",
			Data: json.RawMessage(`{"email":"dave@example.com"}`),
		})
		return err
	})
	require.NoError(t, err)

	var people []dal.Entity
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		people, err = q.ListEntitiesByType(ctx, "Person")
		return err
	})
	require.NoError(t, err)
	require.Len(t, people, 2)
}

func ptr(s string) *string { return &s }
```

Also add `"database/sql"` to the import block.

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/dal/... -v -race
```

Expected: compile errors — `Entity`, `CreateEntity`, etc. undefined.

- [ ] **Step 3: Implement entities.go**

Create `internal/dal/entities.go`:

```go
package dal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/astromechza/aurelianprm/internal/schema"
	"github.com/oklog/ulid/v2"
)

// Entity represents a row in the entities table.
type Entity struct {
	ID          string
	Type        string
	DisplayName *string
	Data        json.RawMessage
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateEntityParams holds fields for inserting a new entity.
type CreateEntityParams struct {
	Type        string
	DisplayName *string
	Data        json.RawMessage
}

// UpdateEntityParams holds fields for updating an existing entity.
type UpdateEntityParams struct {
	ID          string
	DisplayName *string
	Data        json.RawMessage
}

func newID() string {
	return ulid.Make().String()
}

func scanEntity(row interface{ Scan(...any) error }) (Entity, error) {
	var e Entity
	var data string
	err := row.Scan(&e.ID, &e.Type, &e.DisplayName, &data, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return Entity{}, err
	}
	e.Data = json.RawMessage(data)
	return e, nil
}

// CreateEntity validates data against the entity type schema and inserts a new entity.
func (q *Queries) CreateEntity(ctx context.Context, params CreateEntityParams) (Entity, error) {
	if params.DisplayName != nil && *params.DisplayName == "" {
		return Entity{}, fmt.Errorf("create entity: display_name must not be empty string; use nil to omit")
	}
	if err := schema.ValidateEntity(params.Type, params.Data); err != nil {
		return Entity{}, fmt.Errorf("create entity: %w", err)
	}
	const query = `
		INSERT INTO entities (id, type, display_name, data)
		VALUES (?, ?, ?, ?)
		RETURNING id, type, display_name, data, created_at, updated_at`
	row := q.db.QueryRowContext(ctx, query, newID(), params.Type, params.DisplayName, string(params.Data))
	e, err := scanEntity(row)
	if err != nil {
		return Entity{}, fmt.Errorf("create entity: %w", err)
	}
	return e, nil
}

// GetEntity fetches a single entity by ID. Returns sql.ErrNoRows if not found.
func (q *Queries) GetEntity(ctx context.Context, id string) (Entity, error) {
	const query = `
		SELECT id, type, display_name, data, created_at, updated_at
		FROM entities WHERE id = ?`
	e, err := scanEntity(q.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return Entity{}, err
	}
	return e, nil
}

// UpdateEntity validates data and updates display_name and data for an existing entity.
// The entity type cannot be changed.
func (q *Queries) UpdateEntity(ctx context.Context, params UpdateEntityParams) error {
	if params.DisplayName != nil && *params.DisplayName == "" {
		return fmt.Errorf("update entity: display_name must not be empty string; use nil to omit")
	}
	existing, err := q.GetEntity(ctx, params.ID)
	if err != nil {
		return fmt.Errorf("update entity: fetch existing: %w", err)
	}
	if err := schema.ValidateEntity(existing.Type, params.Data); err != nil {
		return fmt.Errorf("update entity: %w", err)
	}
	const query = `
		UPDATE entities
		SET display_name=?, data=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`
	res, err := q.db.ExecContext(ctx, query, params.DisplayName, string(params.Data), params.ID)
	if err != nil {
		return fmt.Errorf("update entity: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update entity: id %q not found", params.ID)
	}
	return nil
}

// DeleteEntity removes an entity by ID. Cascades to relationships.
func (q *Queries) DeleteEntity(ctx context.Context, id string) error {
	if _, err := q.db.ExecContext(ctx, `DELETE FROM entities WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete entity: %w", err)
	}
	return nil
}

// ListEntitiesByType returns all entities of a given type ordered by display_name, then id.
func (q *Queries) ListEntitiesByType(ctx context.Context, entityType string) ([]Entity, error) {
	const query = `
		SELECT id, type, display_name, data, created_at, updated_at
		FROM entities WHERE type=?
		ORDER BY display_name, id`
	rows, err := q.db.QueryContext(ctx, query, entityType)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/dal/... -v -race
```

Expected: all entity tests PASS.

- [ ] **Step 5: Write SearchEntities test**

Add to `internal/dal/dal_test.go`:

```go
func TestSearchEntities(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		for _, p := range []struct{ display, data string }{
			{"Alice Smith", `{"name":"Alice Smith"}`},
			{"Bob Jones", `{"name":"Bob Jones"}`},
			{"Alicia Keys", `{"name":"Alicia Keys"}`},
		} {
			if _, err := q.CreateEntity(ctx, dal.CreateEntityParams{
				Type:        "Person",
				DisplayName: ptr(p.display),
				Data:        json.RawMessage(p.data),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)

	var results []dal.Entity
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		results, err = q.SearchEntities(ctx, "alice")
		return err
	})
	require.NoError(t, err)
	require.Len(t, results, 2) // Alice Smith and Alicia Keys

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		results, err = q.SearchEntities(ctx, "jones")
		return err
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, ptr("Bob Jones"), results[0].DisplayName)
}

func TestSearchEntities_noDisplayName_notIndexed(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person",
			Data: json.RawMessage(`{"name":"Dave"}`),
		})
		return err
	})
	require.NoError(t, err)

	var results []dal.Entity
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		results, err = q.SearchEntities(ctx, "dave")
		return err
	})
	require.NoError(t, err)
	require.Empty(t, results) // no display_name set, not indexed
}
```

- [ ] **Step 6: Implement SearchEntities in entities.go**

Add to `internal/dal/entities.go` (also add `"strings"` to imports):

```go
// sanitizeFTSQuery escapes double-quotes and wraps input as a prefix phrase
// query safe for FTS5 MATCH expressions.
func sanitizeFTSQuery(q string) string {
	return `"` + strings.ReplaceAll(q, `"`, `""`) + `"*`
}

// SearchEntities performs a full-text prefix search over entity display names,
// returning results ordered by BM25 relevance rank.
// Returns nil, nil for a blank query.
func (q *Queries) SearchEntities(ctx context.Context, query string) ([]Entity, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	const stmt = `
		SELECT e.id, e.type, e.display_name, e.data, e.created_at, e.updated_at
		FROM entities_fts f
		JOIN entities e ON e.id = f.entity_id
		WHERE entities_fts MATCH ?
		ORDER BY rank`
	rows, err := q.db.QueryContext(ctx, stmt, sanitizeFTSQuery(query))
	if err != nil {
		return nil, fmt.Errorf("search entities: %w", err)
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}
```

- [ ] **Step 7: Run tests**

```bash
go test ./internal/dal/... -v -race
```

Expected: all tests including search tests PASS.

- [ ] **Step 8: Full verify**

```bash
make verify
```

Expected: all steps pass.

- [ ] **Step 9: Commit**

```bash
git add internal/dal/entities.go internal/dal/dal_test.go
git commit -m "feat: add Entity CRUD with JSONSchema validation and FTS search to DAL"
```

---

### Task 9: Relationship CRUD and graph neighbour queries

**Files:**
- Create: `internal/dal/relationships.go`
- Modify: `internal/dal/dal_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/dal/dal_test.go`:

```go
func TestCreateRelationship_and_GetNeighbors(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID, emailID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person",
			Data: json.RawMessage(`{"name":"Frank"}`),
		})
		if err != nil {
			return err
		}
		personID = p.ID
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "EmailAddress",
			Data: json.RawMessage(`{"email":"frank@example.com"}`),
		})
		emailID = e.ID
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: personID,
			EntityBID: emailID,
			Type:      "hasEmail",
			Note:      ptr("primary email"),
		})
		return err
	})
	require.NoError(t, err)

	var neighbours []dal.NeighbourResult
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		neighbours, err = q.GetNeighbours(ctx, personID)
		return err
	})
	require.NoError(t, err)
	require.Len(t, neighbours, 1)
	require.Equal(t, emailID, neighbours[0].Entity.ID)
	require.Equal(t, "hasEmail", neighbours[0].Relationship.Type)
	require.Equal(t, "outbound", neighbours[0].Direction)
}

func TestCreateRelationship_validationFails(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var idA, idB string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		a, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Greta"}`)})
		if err != nil {
			return err
		}
		idA = a.ID
		b, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Hank"}`)})
		idB = b.ID
		return err
	})
	require.NoError(t, err)

	// hasEmail requires object to be EmailAddress, not Person — must fail.
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: idA,
			EntityBID: idB,
			Type:      "hasEmail",
		})
		return err
	})
	require.Error(t, err)
}

func TestCreateRelationship_partialDates(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var idA, idB string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		a, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Pat"}`)})
		if err != nil {
			return err
		}
		idA = a.ID
		b, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Quinn"}`)})
		idB = b.ID
		return err
	})
	require.NoError(t, err)

	// Year-only and year-month both valid.
	for _, tc := range []struct{ from, to string }{
		{"2019", "2023"},
		{"2019-06", "2023-01"},
		{"2019-06-15", "2023-01-20"},
	} {
		err = d.WithTx(ctx, func(q *dal.Queries) error {
			_, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
				EntityAID: idA, EntityBID: idB, Type: "knows",
				DateFrom: &tc.from, DateTo: &tc.to,
			})
			return err
		})
		require.NoError(t, err, "expected valid for from=%s to=%s", tc.from, tc.to)
	}

	// Invalid formats must be rejected.
	for _, bad := range []string{"01-2019", "2019/06/15", "not-a-date", "20190615"} {
		bad := bad
		err = d.WithTx(ctx, func(q *dal.Queries) error {
			_, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
				EntityAID: idA, EntityBID: idB, Type: "knows",
				DateFrom: &bad,
			})
			return err
		})
		require.Error(t, err, "expected error for date_from=%q", bad)
	}
}

func TestCreateRelationship_symmetric_queryBothDirections(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var idA, idB string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		a, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Iris"}`)})
		if err != nil {
			return err
		}
		idA = a.ID
		b, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Jack"}`)})
		idB = b.ID
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: idA,
			EntityBID: idB,
			Type:      "knows",
		})
		return err
	})
	require.NoError(t, err)

	// GetNeighbours from idB should surface the relationship as "inbound".
	var neighbours []dal.NeighbourResult
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		neighbours, err = q.GetNeighbours(ctx, idB)
		return err
	})
	require.NoError(t, err)
	require.Len(t, neighbours, 1)
	require.Equal(t, idA, neighbours[0].Entity.ID)
	require.Equal(t, "inbound", neighbours[0].Direction)
}

func TestDeleteRelationship_cascadesWithEntity(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Kate"}`)})
		if err != nil {
			return err
		}
		personID = p.ID
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "EmailAddress", Data: json.RawMessage(`{"email":"kate@example.com"}`)})
		if err != nil {
			return err
		}
		_, err = q.CreateRelationship(ctx, dal.CreateRelationshipParams{EntityAID: personID, EntityBID: e.ID, Type: "hasEmail"})
		return err
	})
	require.NoError(t, err)

	// Deleting the person should cascade to the relationship.
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.DeleteEntity(ctx, personID)
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, d.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM relationships`).Scan(&count))
	require.Equal(t, 0, count)
}

func TestUpdateRelationship(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var relID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Leo"}`)})
		if err != nil {
			return err
		}
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "EmailAddress", Data: json.RawMessage(`{"email":"leo@example.com"}`)})
		if err != nil {
			return err
		}
		r, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: p.ID, EntityBID: e.ID, Type: "hasEmail",
		})
		relID = r.ID
		return err
	})
	require.NoError(t, err)

	dateFrom := "2020-03"
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.UpdateRelationship(ctx, dal.UpdateRelationshipParams{
			ID:       relID,
			DateFrom: &dateFrom,
			Note:     ptr("primary work email"),
		})
	})
	require.NoError(t, err)

	var note, df string
	require.NoError(t, d.DB().QueryRowContext(ctx,
		`SELECT note, date_from FROM relationships WHERE id=?`, relID,
	).Scan(&note, &df))
	require.Equal(t, "primary work email", note)
	require.Equal(t, "2020-03", df)
}

func TestUpdateRelationship_invalidDate(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var relID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Mia"}`)})
		if err != nil {
			return err
		}
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "EmailAddress", Data: json.RawMessage(`{"email":"mia@example.com"}`)})
		if err != nil {
			return err
		}
		r, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: p.ID, EntityBID: e.ID, Type: "hasEmail",
		})
		relID = r.ID
		return err
	})
	require.NoError(t, err)

	bad := "not-a-date"
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.UpdateRelationship(ctx, dal.UpdateRelationshipParams{
			ID:       relID,
			DateFrom: &bad,
		})
	})
	require.Error(t, err)
}

func TestGetNeighboursByRelType(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Nina"}`)})
		if err != nil {
			return err
		}
		personID = p.ID
		email, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "EmailAddress", Data: json.RawMessage(`{"email":"nina@example.com"}`)})
		if err != nil {
			return err
		}
		phone, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "PhoneNumber", Data: json.RawMessage(`{"telephone":"+1 555 000 1234"}`)})
		if err != nil {
			return err
		}
		if _, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{EntityAID: p.ID, EntityBID: email.ID, Type: "hasEmail"}); err != nil {
			return err
		}
		_, err = q.CreateRelationship(ctx, dal.CreateRelationshipParams{EntityAID: p.ID, EntityBID: phone.ID, Type: "hasPhone"})
		return err
	})
	require.NoError(t, err)

	var emails []dal.NeighbourResult
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		emails, err = q.GetNeighboursByRelType(ctx, personID, "hasEmail")
		return err
	})
	require.NoError(t, err)
	require.Len(t, emails, 1)
	require.Equal(t, "EmailAddress", emails[0].Entity.Type)
}
```

- [ ] **Step 2: Run to verify failure**

```bash
go test ./internal/dal/... -v -race
```

Expected: compile errors — `Relationship`, `NeighbourResult`, `CreateRelationship` undefined.

- [ ] **Step 3: Implement relationships.go**

Create `internal/dal/relationships.go`:

```go
package dal

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/astromechza/aurelianprm/internal/schema"
)

// Relationship represents a row in the relationships table.
type Relationship struct {
	ID        string
	EntityAID string
	EntityBID string
	Type      string
	DateFrom  *string // partial ISO 8601: "YYYY", "YYYY-MM", or "YYYY-MM-DD"
	DateTo    *string // partial ISO 8601: "YYYY", "YYYY-MM", or "YYYY-MM-DD"
	Note      *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// partialDateRE matches YYYY, YYYY-MM, or YYYY-MM-DD.
var partialDateRE = regexp.MustCompile(`^\d{4}(-\d{2}(-\d{2})?)?$`)

func validatePartialDate(field, value string) error {
	if !partialDateRE.MatchString(value) {
		return fmt.Errorf("%s %q must be YYYY, YYYY-MM, or YYYY-MM-DD", field, value)
	}
	return nil
}

// validateDateRange checks that from <= to using lexicographic order, which is
// correct for partial ISO 8601 strings of any precision.
func validateDateRange(from, to string) error {
	if from > to {
		return fmt.Errorf("date_from %q must not be after date_to %q", from, to)
	}
	return nil
}

// CreateRelationshipParams holds fields for inserting a new relationship.
type CreateRelationshipParams struct {
	EntityAID string
	EntityBID string
	Type      string
	DateFrom  *string // ISO 8601 date
	DateTo    *string // ISO 8601 date
	Note      *string
}

// NeighbourResult is returned by GetNeighbours and pairs an entity with the
// relationship that connects it to the query entity.
type NeighbourResult struct {
	Entity       Entity
	Relationship Relationship
	Direction    string // "outbound" (query entity is entity_a) or "inbound" (query entity is entity_b)
}

func scanRelationship(row interface{ Scan(...any) error }) (Relationship, error) {
	var r Relationship
	err := row.Scan(
		&r.ID, &r.EntityAID, &r.EntityBID, &r.Type,
		&r.DateFrom, &r.DateTo, &r.Note,
		&r.CreatedAt, &r.UpdatedAt,
	)
	return r, err
}

// CreateRelationship validates the relationship type constraints and inserts a new edge.
func (q *Queries) CreateRelationship(ctx context.Context, params CreateRelationshipParams) (Relationship, error) {
	entityA, err := q.GetEntity(ctx, params.EntityAID)
	if err != nil {
		return Relationship{}, fmt.Errorf("create relationship: fetch entity_a: %w", err)
	}
	entityB, err := q.GetEntity(ctx, params.EntityBID)
	if err != nil {
		return Relationship{}, fmt.Errorf("create relationship: fetch entity_b: %w", err)
	}
	if err := schema.ValidateRelationship(params.Type, entityA.Type, entityB.Type); err != nil {
		return Relationship{}, fmt.Errorf("create relationship: %w", err)
	}
	if params.DateFrom != nil {
		if err := validatePartialDate("date_from", *params.DateFrom); err != nil {
			return Relationship{}, fmt.Errorf("create relationship: %w", err)
		}
	}
	if params.DateTo != nil {
		if err := validatePartialDate("date_to", *params.DateTo); err != nil {
			return Relationship{}, fmt.Errorf("create relationship: %w", err)
		}
	}
	if params.DateFrom != nil && params.DateTo != nil {
		if err := validateDateRange(*params.DateFrom, *params.DateTo); err != nil {
			return Relationship{}, fmt.Errorf("create relationship: %w", err)
		}
	}

	// For symmetric relationship types, block reverse-direction duplicates that
	// the UNIQUE constraint alone cannot catch.
	def := schema.RelationshipTypes[params.Type]
	if def.Symmetric {
		var n int
		if err := q.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM relationships
			WHERE type=? AND (
				(entity_a_id=? AND entity_b_id=?) OR
				(entity_a_id=? AND entity_b_id=?)
			)`, params.Type,
			params.EntityAID, params.EntityBID,
			params.EntityBID, params.EntityAID,
		).Scan(&n); err != nil {
			return Relationship{}, fmt.Errorf("create relationship: duplicate check: %w", err)
		}
		if n > 0 {
			return Relationship{}, fmt.Errorf("create relationship: %q relationship already exists between these entities", params.Type)
		}
	}

	const query = `
		INSERT INTO relationships (id, entity_a_id, entity_b_id, type, date_from, date_to, note)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, entity_a_id, entity_b_id, type, date_from, date_to, note, created_at, updated_at`
	row := q.db.QueryRowContext(ctx, query,
		newID(), params.EntityAID, params.EntityBID, params.Type,
		params.DateFrom, params.DateTo, params.Note,
	)
	r, err := scanRelationship(row)
	if err != nil {
		return Relationship{}, fmt.Errorf("create relationship: %w", err)
	}
	return r, nil
}

// DeleteRelationship removes a relationship by ID.
func (q *Queries) DeleteRelationship(ctx context.Context, id string) error {
	if _, err := q.db.ExecContext(ctx, `DELETE FROM relationships WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete relationship: %w", err)
	}
	return nil
}

// GetNeighbours returns all entities directly connected to entityID via any
// relationship, along with the relationship and its direction relative to entityID.
func (q *Queries) GetNeighbours(ctx context.Context, entityID string) ([]NeighbourResult, error) {
	const query = `
		SELECT
			e.id, e.type, e.display_name, e.data, e.created_at, e.updated_at,
			r.id, r.entity_a_id, r.entity_b_id, r.type, r.date_from, r.date_to, r.note, r.created_at, r.updated_at,
			CASE WHEN r.entity_a_id = ? THEN 'outbound' ELSE 'inbound' END AS direction
		FROM relationships r
		JOIN entities e ON e.id = CASE
			WHEN r.entity_a_id = ? THEN r.entity_b_id
			ELSE r.entity_a_id
		END
		WHERE r.entity_a_id = ? OR r.entity_b_id = ?
		ORDER BY r.created_at`

	rows, err := q.db.QueryContext(ctx, query, entityID, entityID, entityID, entityID)
	if err != nil {
		return nil, fmt.Errorf("get neighbours: %w", err)
	}
	defer rows.Close()

	var results []NeighbourResult
	for rows.Next() {
		var nr NeighbourResult
		var entityData string
		err := rows.Scan(
			&nr.Entity.ID, &nr.Entity.Type, &nr.Entity.DisplayName, &entityData,
			&nr.Entity.CreatedAt, &nr.Entity.UpdatedAt,
			&nr.Relationship.ID, &nr.Relationship.EntityAID, &nr.Relationship.EntityBID,
			&nr.Relationship.Type, &nr.Relationship.DateFrom, &nr.Relationship.DateTo,
			&nr.Relationship.Note, &nr.Relationship.CreatedAt, &nr.Relationship.UpdatedAt,
			&nr.Direction,
		)
		if err != nil {
			return nil, fmt.Errorf("scan neighbour: %w", err)
		}
		nr.Entity.Data = []byte(entityData)
		results = append(results, nr)
	}
	return results, rows.Err()
}

// UpdateRelationshipParams holds editable fields for an existing relationship.
type UpdateRelationshipParams struct {
	ID       string
	DateFrom *string // partial ISO 8601 or nil to clear
	DateTo   *string // partial ISO 8601 or nil to clear
	Note     *string // nil to clear
}

// UpdateRelationship updates the date range and note on an existing relationship.
// The entity pair and type are immutable.
func (q *Queries) UpdateRelationship(ctx context.Context, params UpdateRelationshipParams) error {
	if params.DateFrom != nil {
		if err := validatePartialDate("date_from", *params.DateFrom); err != nil {
			return fmt.Errorf("update relationship: %w", err)
		}
	}
	if params.DateTo != nil {
		if err := validatePartialDate("date_to", *params.DateTo); err != nil {
			return fmt.Errorf("update relationship: %w", err)
		}
	}
	if params.DateFrom != nil && params.DateTo != nil {
		if err := validateDateRange(*params.DateFrom, *params.DateTo); err != nil {
			return fmt.Errorf("update relationship: %w", err)
		}
	}
	const stmt = `
		UPDATE relationships
		SET date_from=?, date_to=?, note=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`
	res, err := q.db.ExecContext(ctx, stmt, params.DateFrom, params.DateTo, params.Note, params.ID)
	if err != nil {
		return fmt.Errorf("update relationship: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update relationship: id %q not found", params.ID)
	}
	return nil
}

// GetNeighboursByRelType returns neighbours connected via a specific relationship type.
func (q *Queries) GetNeighboursByRelType(ctx context.Context, entityID, relType string) ([]NeighbourResult, error) {
	const stmt = `
		SELECT
			e.id, e.type, e.display_name, e.data, e.created_at, e.updated_at,
			r.id, r.entity_a_id, r.entity_b_id, r.type, r.date_from, r.date_to, r.note, r.created_at, r.updated_at,
			CASE WHEN r.entity_a_id = ? THEN 'outbound' ELSE 'inbound' END AS direction
		FROM relationships r
		JOIN entities e ON e.id = CASE
			WHEN r.entity_a_id = ? THEN r.entity_b_id
			ELSE r.entity_a_id
		END
		WHERE (r.entity_a_id = ? OR r.entity_b_id = ?) AND r.type = ?
		ORDER BY r.created_at`

	rows, err := q.db.QueryContext(ctx, stmt, entityID, entityID, entityID, entityID, relType)
	if err != nil {
		return nil, fmt.Errorf("get neighbours by rel type: %w", err)
	}
	defer rows.Close()

	var results []NeighbourResult
	for rows.Next() {
		var nr NeighbourResult
		var entityData string
		err := rows.Scan(
			&nr.Entity.ID, &nr.Entity.Type, &nr.Entity.DisplayName, &entityData,
			&nr.Entity.CreatedAt, &nr.Entity.UpdatedAt,
			&nr.Relationship.ID, &nr.Relationship.EntityAID, &nr.Relationship.EntityBID,
			&nr.Relationship.Type, &nr.Relationship.DateFrom, &nr.Relationship.DateTo,
			&nr.Relationship.Note, &nr.Relationship.CreatedAt, &nr.Relationship.UpdatedAt,
			&nr.Direction,
		)
		if err != nil {
			return nil, fmt.Errorf("scan neighbour: %w", err)
		}
		nr.Entity.Data = []byte(entityData)
		results = append(results, nr)
	}
	return results, rows.Err()
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/dal/... -v -race
```

Expected: all tests PASS.

- [ ] **Step 5: Final full verify**

```bash
make verify
```

Expected: all steps pass — fmt, tidy, test -race, lint all green.

- [ ] **Step 6: Commit**

```bash
git add internal/dal/relationships.go internal/dal/dal_test.go
git commit -m "feat: add Relationship CRUD and GetNeighbours graph query to DAL"
```

---

## Self-Review

**Spec coverage:**
- ✅ `entities` table: ULID PK, type, display_name, JSON data blob
- ✅ `relationships` table: bidirectional, date_from, date_to, note
- ✅ JSONSchema per entity type, embedded in binary, validated on write
- ✅ Relationship type registry: symmetric flag, subject/object type constraints
- ✅ Graph-style neighbour lookup with direction indicator
- ✅ 8 entity types: Person, PostalAddress, EmailAddress, PhoneNumber, WebSite, SocialNetwork, Pet, Career
- ✅ 12 relationship types as catalogued above
- ✅ `make verify` gate at every commit

**Type consistency:** `Entity`, `Relationship`, `NeighbourResult`, `CreateEntityParams`, `UpdateEntityParams`, `CreateRelationshipParams` used consistently. `scanEntity` and `scanRelationship` accept `interface{ Scan(...any) error }` — compatible with both `*sql.Row` and `*sql.Rows`.

**No placeholders:** All steps contain complete code, exact commands, and expected output.

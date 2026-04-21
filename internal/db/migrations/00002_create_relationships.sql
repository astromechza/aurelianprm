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

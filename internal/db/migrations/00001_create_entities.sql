-- +goose Up
CREATE TABLE entities (
    id           TEXT     PRIMARY KEY,
    type         TEXT     NOT NULL,
    display_name TEXT,
    data         TEXT     NOT NULL DEFAULT '{}',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX entities_type         ON entities(type);
CREATE INDEX entities_display_name ON entities(display_name)
    WHERE display_name IS NOT NULL;

-- +goose Down
DROP TABLE entities;

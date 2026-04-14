-- +goose Up

CREATE TABLE blurry_dates (
    id                   TEXT    PRIMARY KEY,
    year                 INTEGER,
    month                INTEGER,
    day                  INTEGER,
    reminder_enabled     INTEGER NOT NULL DEFAULT 1,
    reminder_days_before INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE blurry_locations (
    id        TEXT PRIMARY KEY,
    country   TEXT NOT NULL,
    city      TEXT,
    street    TEXT,
    latitude  REAL,
    longitude REAL
);

CREATE TABLE people (
    id          TEXT     PRIMARY KEY,
    name        TEXT     NOT NULL,
    nickname    TEXT,
    notes       TEXT,
    birthday_id TEXT     REFERENCES blurry_dates(id),
    created_at  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE contact_methods (
    id         TEXT     PRIMARY KEY,
    person_id  TEXT     NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    type       TEXT     NOT NULL CHECK (type IN ('email', 'phone', 'social', 'website')),
    value      TEXT     NOT NULL,
    label      TEXT,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE addresses (
    id          TEXT     PRIMARY KEY,
    location_id TEXT     NOT NULL REFERENCES blurry_locations(id) ON DELETE CASCADE,
    label       TEXT,
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE pets (
    id         TEXT     PRIMARY KEY,
    name       TEXT     NOT NULL,
    notes      TEXT,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE work (
    id         TEXT     PRIMARY KEY,
    name       TEXT     NOT NULL,
    website    TEXT,
    notes      TEXT,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE interactions (
    id          TEXT     PRIMARY KEY,
    person_id   TEXT     NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    occurred_at DATETIME NOT NULL,
    channel     TEXT,
    notes       TEXT,
    created_at  DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE relationships (
    id         TEXT     PRIMARY KEY,
    person_id  TEXT     NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    to_type    TEXT     NOT NULL CHECK (to_type IN ('person', 'address', 'pet', 'work')),
    to_id      TEXT     NOT NULL,
    label      TEXT     NOT NULL,
    since_id   TEXT     REFERENCES blurry_dates(id),
    until_id   TEXT     REFERENCES blurry_dates(id),
    notes      TEXT,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- +goose Down

DROP TABLE IF EXISTS relationships;
DROP TABLE IF EXISTS interactions;
DROP TABLE IF EXISTS work;
DROP TABLE IF EXISTS pets;
DROP TABLE IF EXISTS addresses;
DROP TABLE IF EXISTS contact_methods;
DROP TABLE IF EXISTS people;
DROP TABLE IF EXISTS blurry_locations;
DROP TABLE IF EXISTS blurry_dates;

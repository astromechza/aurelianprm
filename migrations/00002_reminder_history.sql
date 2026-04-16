-- +goose Up

CREATE TABLE reminder_history (
    id         TEXT PRIMARY KEY,
    sent_at    DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    status     TEXT NOT NULL CHECK (status IN ('success', 'failure')),
    error      TEXT,
    date_start TEXT NOT NULL,
    date_end   TEXT NOT NULL
);

-- +goose Down

DROP TABLE reminder_history;

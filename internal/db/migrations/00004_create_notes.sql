-- +goose Up
CREATE TABLE notes (
    id         TEXT     PRIMARY KEY,
    type       TEXT     NOT NULL CHECK(type IN ('LIFE_EVENT','CONVERSATION','HANGOUT')),
    date       TEXT     NOT NULL,
    content    TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX notes_date ON notes(date);

CREATE TABLE note_persons (
    note_id   TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    person_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    PRIMARY KEY (note_id, person_id)
);
CREATE INDEX note_persons_person ON note_persons(person_id);

-- +goose Down
DROP TABLE note_persons;
DROP TABLE notes;

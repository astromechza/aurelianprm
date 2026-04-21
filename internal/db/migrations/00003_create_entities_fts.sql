-- +goose Up
CREATE VIRTUAL TABLE entities_fts USING fts5(display_name, entity_id UNINDEXED, tokenize = 'porter unicode61');
INSERT INTO entities_fts(display_name, entity_id) SELECT display_name, id FROM entities WHERE display_name IS NOT NULL;
CREATE TRIGGER entities_fts_insert AFTER INSERT ON entities WHEN NEW.display_name IS NOT NULL BEGIN INSERT INTO entities_fts(display_name, entity_id) VALUES (NEW.display_name, NEW.id); END;
CREATE TRIGGER entities_fts_update AFTER UPDATE OF display_name ON entities BEGIN DELETE FROM entities_fts WHERE entity_id = OLD.id; INSERT INTO entities_fts(display_name, entity_id) SELECT NEW.display_name, NEW.id WHERE NEW.display_name IS NOT NULL; END;
CREATE TRIGGER entities_fts_delete AFTER DELETE ON entities BEGIN DELETE FROM entities_fts WHERE entity_id = OLD.id; END;

-- +goose Down
DROP TRIGGER IF EXISTS entities_fts_delete;
DROP TRIGGER IF EXISTS entities_fts_update;
DROP TRIGGER IF EXISTS entities_fts_insert;
DROP TABLE IF EXISTS entities_fts;

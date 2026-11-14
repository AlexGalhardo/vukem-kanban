CREATE TABLE boards (
	id          INTEGER PRIMARY KEY,
	name        TEXT    NOT NULL,
	description TEXT    NOT NULL DEFAULT '',
	color       TEXT    NOT NULL DEFAULT 'slate',
	archived_at INTEGER,
	created_at  INTEGER NOT NULL,
	updated_at  INTEGER NOT NULL
);

CREATE TABLE columns (
	id         INTEGER PRIMARY KEY,
	board_id   INTEGER NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
	name       TEXT    NOT NULL,
	position   INTEGER NOT NULL,
	wip_limit  INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL
);

CREATE INDEX columns_board_position ON columns (board_id, position);

CREATE TABLE cards (
	id          INTEGER PRIMARY KEY,
	column_id   INTEGER NOT NULL REFERENCES columns (id) ON DELETE CASCADE,
	title       TEXT    NOT NULL,
	description TEXT    NOT NULL DEFAULT '',
	priority    INTEGER NOT NULL DEFAULT 0,
	due_date    TEXT    NOT NULL DEFAULT '',
	position    INTEGER NOT NULL,
	created_at  INTEGER NOT NULL,
	updated_at  INTEGER NOT NULL
);

CREATE INDEX cards_column_position ON cards (column_id, position);

CREATE TABLE labels (
	id       INTEGER PRIMARY KEY,
	board_id INTEGER NOT NULL REFERENCES boards (id) ON DELETE CASCADE,
	name     TEXT    NOT NULL,
	color    TEXT    NOT NULL
);

CREATE INDEX labels_board ON labels (board_id);

CREATE TABLE card_labels (
	card_id  INTEGER NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
	label_id INTEGER NOT NULL REFERENCES labels (id) ON DELETE CASCADE,
	PRIMARY KEY (card_id, label_id)
);

CREATE INDEX card_labels_label ON card_labels (label_id);

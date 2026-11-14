-- revision increments on every card update; the card dialog sends the
-- revision it loaded so a stale tab cannot overwrite newer edits.
ALTER TABLE cards ADD COLUMN revision INTEGER NOT NULL DEFAULT 0;

package store

import (
	"context"
	"database/sql"
)

// orderedTable describes a table whose rows keep a contiguous 0..n-1
// "position" inside a group (cards inside a column, columns inside a board).
type orderedTable struct {
	shiftDown, shiftUp, count, place string
}

var (
	cardsTable = orderedTable{
		shiftDown: `UPDATE cards SET position = position - 1 WHERE column_id = ? AND position > ?`,
		shiftUp:   `UPDATE cards SET position = position + 1 WHERE column_id = ? AND position >= ? AND id <> ?`,
		count:     `SELECT COUNT(*) FROM cards WHERE column_id = ? AND id <> ?`,
		place:     `UPDATE cards SET column_id = ?, position = ? WHERE id = ?`,
	}
	columnsTable = orderedTable{
		shiftDown: `UPDATE columns SET position = position - 1 WHERE board_id = ? AND position > ?`,
		shiftUp:   `UPDATE columns SET position = position + 1 WHERE board_id = ? AND position >= ? AND id <> ?`,
		count:     `SELECT COUNT(*) FROM columns WHERE board_id = ? AND id <> ?`,
		place:     `UPDATE columns SET board_id = ?, position = ? WHERE id = ?`,
	}
)

// reposition moves row id from (fromGroup, fromPos) to (toGroup, toPos),
// clamping toPos to the target group's bounds and keeping both groups contiguous.
func reposition(ctx context.Context, tx *sql.Tx, t orderedTable, id, fromGroup int64, fromPos int, toGroup int64, toPos int) error {
	if _, err := tx.ExecContext(ctx, t.shiftDown, fromGroup, fromPos); err != nil {
		return err
	}
	var n int
	if err := tx.QueryRowContext(ctx, t.count, toGroup, id).Scan(&n); err != nil {
		return err
	}
	toPos = max(0, min(toPos, n))
	if _, err := tx.ExecContext(ctx, t.shiftUp, toGroup, toPos, id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, t.place, toGroup, toPos, id)
	return err
}

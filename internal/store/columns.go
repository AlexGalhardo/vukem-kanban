package store

import (
	"context"
	"database/sql"
	"errors"
)

type Column struct {
	ID       int64
	BoardID  int64
	Name     string
	Position int
	WIPLimit int
	Cards    []Card
}

// OverLimit reports whether the column holds more cards than its WIP limit.
func (c Column) OverLimit() bool { return c.WIPLimit > 0 && len(c.Cards) > c.WIPLimit }

type ColumnInput struct {
	Name     string
	WIPLimit int
}

func (in *ColumnInput) normalize() error {
	in.Name = singleLine(in.Name)
	v := ValidationError{}
	checkLen(v, "name", in.Name, 1, 40)
	if in.WIPLimit < 0 || in.WIPLimit > 999 {
		v["wip_limit"] = "deve estar entre 0 e 999"
	}
	if len(v) > 0 {
		return v
	}
	return nil
}

func (s *Store) GetColumn(ctx context.Context, id int64) (Column, error) {
	var c Column
	err := s.db.QueryRowContext(ctx,
		`SELECT id, board_id, name, position, wip_limit FROM columns WHERE id = ?`, id).
		Scan(&c.ID, &c.BoardID, &c.Name, &c.Position, &c.WIPLimit)
	return c, orNotFound(err)
}

// CreateColumn appends a column to the end of the board.
func (s *Store) CreateColumn(ctx context.Context, boardID int64, in ColumnInput) (Column, error) {
	if err := in.normalize(); err != nil {
		return Column{}, err
	}
	var id int64
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM boards WHERE id = ?)`, boardID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO columns (board_id, name, position, wip_limit, created_at)
			 VALUES (?, ?, (SELECT COUNT(*) FROM columns WHERE board_id = ?), ?, ?)`,
			boardID, in.Name, boardID, in.WIPLimit, s.now().Unix())
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return Column{}, err
	}
	return s.GetColumn(ctx, id)
}

func (s *Store) UpdateColumn(ctx context.Context, id int64, in ColumnInput) (Column, error) {
	if err := in.normalize(); err != nil {
		return Column{}, err
	}
	if err := affectedOrNotFound(s.db.ExecContext(ctx,
		`UPDATE columns SET name = ?, wip_limit = ? WHERE id = ?`, in.Name, in.WIPLimit, id)); err != nil {
		return Column{}, err
	}
	return s.GetColumn(ctx, id)
}

// DeleteColumn removes a column. A column with cards is only deleted when
// moveTo names another column of the same board, which receives the cards.
func (s *Store) DeleteColumn(ctx context.Context, id, moveTo int64) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		var boardID int64
		var pos, count int
		err := tx.QueryRowContext(ctx,
			`SELECT board_id, position, (SELECT COUNT(*) FROM cards WHERE column_id = columns.id) FROM columns WHERE id = ?`, id).
			Scan(&boardID, &pos, &count)
		if err != nil {
			return orNotFound(err)
		}
		if count > 0 {
			if moveTo == 0 || moveTo == id {
				return ErrColumnNotEmpty
			}
			var target int64
			if err := tx.QueryRowContext(ctx, `SELECT board_id FROM columns WHERE id = ?`, moveTo).Scan(&target); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrColumnNotEmpty
				}
				return err
			}
			if target != boardID {
				return ErrColumnNotEmpty
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE cards SET column_id = ?, position = position + (SELECT COUNT(*) FROM cards WHERE column_id = ?)
				 WHERE column_id = ?`, moveTo, moveTo, id); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM columns WHERE id = ?`, id); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE columns SET position = position - 1 WHERE board_id = ? AND position > ?`, boardID, pos)
		return err
	})
}

// MoveColumn reorders a column inside its board.
func (s *Store) MoveColumn(ctx context.Context, id int64, position int) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		var boardID int64
		var from int
		if err := tx.QueryRowContext(ctx, `SELECT board_id, position FROM columns WHERE id = ?`, id).
			Scan(&boardID, &from); err != nil {
			return orNotFound(err)
		}
		return reposition(ctx, tx, columnsTable, id, boardID, from, boardID, position)
	})
}

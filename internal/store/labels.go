package store

import (
	"context"
	"database/sql"
)

type Label struct {
	ID      int64
	BoardID int64
	Name    string
	Color   string
}

type LabelInput struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func (in *LabelInput) normalize() error {
	in.Name = clampText(in.Name)
	v := ValidationError{}
	checkLen(v, "name", in.Name, 1, 30)
	if !validColor(in.Color) {
		v["color"] = "cor inválida"
	}
	if len(v) > 0 {
		return v
	}
	return nil
}

func (s *Store) GetLabel(ctx context.Context, id int64) (Label, error) {
	var l Label
	err := s.db.QueryRowContext(ctx, `SELECT id, board_id, name, color FROM labels WHERE id = ?`, id).
		Scan(&l.ID, &l.BoardID, &l.Name, &l.Color)
	return l, orNotFound(err)
}

func (s *Store) ListLabels(ctx context.Context, boardID int64) ([]Label, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, board_id, name, color FROM labels WHERE board_id = ? ORDER BY name COLLATE NOCASE, id`, boardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.BoardID, &l.Name, &l.Color); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) CreateLabel(ctx context.Context, boardID int64, in LabelInput) (Label, error) {
	if err := in.normalize(); err != nil {
		return Label{}, err
	}
	if _, err := s.GetBoard(ctx, boardID); err != nil {
		return Label{}, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO labels (board_id, name, color) VALUES (?, ?, ?)`, boardID, in.Name, in.Color)
	if err != nil {
		return Label{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Label{}, err
	}
	return s.GetLabel(ctx, id)
}

func (s *Store) UpdateLabel(ctx context.Context, id int64, in LabelInput) (Label, error) {
	if err := in.normalize(); err != nil {
		return Label{}, err
	}
	if err := affectedOrNotFound(s.db.ExecContext(ctx,
		`UPDATE labels SET name = ?, color = ? WHERE id = ?`, in.Name, in.Color, id)); err != nil {
		return Label{}, err
	}
	return s.GetLabel(ctx, id)
}

func (s *Store) DeleteLabel(ctx context.Context, id int64) error {
	return affectedOrNotFound(s.db.ExecContext(ctx, `DELETE FROM labels WHERE id = ?`, id))
}

// ToggleCardLabel attaches the label to the card, or detaches it if already
// attached. Both must belong to the same board.
func (s *Store) ToggleCardLabel(ctx context.Context, cardID, labelID int64) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		var sameBoard bool
		err := tx.QueryRowContext(ctx,
			`SELECT col.board_id = l.board_id FROM cards c JOIN columns col ON col.id = c.column_id, labels l
			 WHERE c.id = ? AND l.id = ?`, cardID, labelID).Scan(&sameBoard)
		if err != nil {
			return orNotFound(err)
		}
		if !sameBoard {
			return ValidationError{"label_id": "label de outro projeto"}
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM card_labels WHERE card_id = ? AND label_id = ?`, cardID, labelID)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n > 0 {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO card_labels (card_id, label_id) VALUES (?, ?)`, cardID, labelID)
		return err
	})
}

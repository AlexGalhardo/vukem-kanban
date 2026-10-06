package store

import (
	"context"
	"database/sql"
	"time"
)

// DefaultColumns are created with every new board.
var DefaultColumns = []string{"Backlog", "A fazer", "Em progresso", "Revisão", "Feito"}

type Board struct {
	ID          int64
	Name        string
	Description string
	Color       string
	Archived    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type BoardInput struct {
	Name        string
	Description string
	Color       string
}

func (in *BoardInput) normalize() error {
	in.Name = singleLine(in.Name)
	in.Description = clampText(in.Description)
	if in.Color == "" {
		in.Color = "slate"
	}
	v := ValidationError{}
	checkLen(v, "name", in.Name, 1, 80)
	checkLen(v, "description", in.Description, 0, 2000)
	if !validColor(in.Color) {
		v["color"] = "cor inválida"
	}
	if len(v) > 0 {
		return v
	}
	return nil
}

const boardCols = `id, name, description, color, archived_at IS NOT NULL, created_at, updated_at`

func scanBoard(row interface{ Scan(...any) error }) (Board, error) {
	var b Board
	var created, updated int64
	err := row.Scan(&b.ID, &b.Name, &b.Description, &b.Color, &b.Archived, &created, &updated)
	b.CreatedAt, b.UpdatedAt = time.Unix(created, 0), time.Unix(updated, 0)
	return b, err
}

// CreateBoard inserts a board together with DefaultColumns in one transaction.
func (s *Store) CreateBoard(ctx context.Context, in BoardInput) (Board, error) {
	if err := in.normalize(); err != nil {
		return Board{}, err
	}
	var id int64
	now := s.now().Unix()
	err := s.tx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO boards (name, description, color, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			in.Name, in.Description, in.Color, now, now)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		for i, name := range DefaultColumns {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO columns (board_id, name, position, created_at) VALUES (?, ?, ?, ?)`,
				id, name, i, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Board{}, err
	}
	return s.GetBoard(ctx, id)
}

func (s *Store) GetBoard(ctx context.Context, id int64) (Board, error) {
	b, err := scanBoard(s.db.QueryRowContext(ctx, `SELECT `+boardCols+` FROM boards WHERE id = ?`, id))
	return b, orNotFound(err)
}

// ListBoards returns active or archived boards ordered by name.
func (s *Store) ListBoards(ctx context.Context, archived bool) ([]Board, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+boardCols+` FROM boards WHERE (archived_at IS NOT NULL) = ? ORDER BY name COLLATE NOCASE, id`, archived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Board
	for rows.Next() {
		b, err := scanBoard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) UpdateBoard(ctx context.Context, id int64, in BoardInput) (Board, error) {
	if err := in.normalize(); err != nil {
		return Board{}, err
	}
	err := affectedOrNotFound(s.db.ExecContext(ctx,
		`UPDATE boards SET name = ?, description = ?, color = ?, updated_at = ? WHERE id = ?`,
		in.Name, in.Description, in.Color, s.now().Unix(), id))
	if err != nil {
		return Board{}, err
	}
	return s.GetBoard(ctx, id)
}

// SetBoardArchived archives or restores a board.
func (s *Store) SetBoardArchived(ctx context.Context, id int64, archived bool) error {
	var at any
	if archived {
		at = s.now().Unix()
	}
	return affectedOrNotFound(s.db.ExecContext(ctx,
		`UPDATE boards SET archived_at = ?, updated_at = ? WHERE id = ?`, at, s.now().Unix(), id))
}

// DeleteBoard removes a board and, through ON DELETE CASCADE, everything in it.
func (s *Store) DeleteBoard(ctx context.Context, id int64) error {
	return affectedOrNotFound(s.db.ExecContext(ctx, `DELETE FROM boards WHERE id = ?`, id))
}

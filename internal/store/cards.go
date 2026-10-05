package store

import (
	"context"
	"database/sql"
	"time"
)

// Priorities follow Linear: 0 none, 1 urgent, 2 high, 3 medium, 4 low.
var Priorities = []string{"Sem prioridade", "Urgente", "Alta", "Média", "Baixa"}

const dateLayout = "2006-01-02"

type Card struct {
	ID          int64
	BoardID     int64
	ColumnID    int64
	Title       string
	Description string
	Priority    int
	DueDate     string // YYYY-MM-DD or ""
	Position    int
	Labels      []Label
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Overdue reports whether the due date is before today (YYYY-MM-DD).
func (c Card) Overdue(today string) bool { return c.DueDate != "" && c.DueDate < today }

// HasLabel reports whether the card carries the label.
func (c Card) HasLabel(id int64) bool {
	for _, l := range c.Labels {
		if l.ID == id {
			return true
		}
	}
	return false
}

type CardInput struct {
	Title       string
	Description string
	Priority    int
	DueDate     string
}

func (in *CardInput) normalize() error {
	in.Title = clampText(in.Title)
	in.Description = clampText(in.Description)
	in.DueDate = clampText(in.DueDate)
	v := ValidationError{}
	checkLen(v, "title", in.Title, 1, 200)
	checkLen(v, "description", in.Description, 0, 10000)
	if in.Priority < 0 || in.Priority >= len(Priorities) {
		v["priority"] = "prioridade inválida"
	}
	if in.DueDate != "" {
		if _, err := time.Parse(dateLayout, in.DueDate); err != nil {
			v["due_date"] = "data inválida (use AAAA-MM-DD)"
		}
	}
	if len(v) > 0 {
		return v
	}
	return nil
}

const cardCols = `c.id, col.board_id, c.column_id, c.title, c.description, c.priority, c.due_date, c.position, c.created_at, c.updated_at`

func scanCard(row interface{ Scan(...any) error }) (Card, error) {
	var c Card
	var created, updated int64
	err := row.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Title, &c.Description, &c.Priority, &c.DueDate, &c.Position, &created, &updated)
	c.CreatedAt, c.UpdatedAt = time.Unix(created, 0), time.Unix(updated, 0)
	return c, err
}

// GetCard returns a card with its labels.
func (s *Store) GetCard(ctx context.Context, id int64) (Card, error) {
	c, err := scanCard(s.db.QueryRowContext(ctx,
		`SELECT `+cardCols+` FROM cards c JOIN columns col ON col.id = c.column_id WHERE c.id = ?`, id))
	if err != nil {
		return Card{}, orNotFound(err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT l.id, l.board_id, l.name, l.color FROM card_labels cl JOIN labels l ON l.id = cl.label_id
		 WHERE cl.card_id = ? ORDER BY l.name COLLATE NOCASE`, id)
	if err != nil {
		return Card{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.ID, &l.BoardID, &l.Name, &l.Color); err != nil {
			return Card{}, err
		}
		c.Labels = append(c.Labels, l)
	}
	return c, rows.Err()
}

// CreateCard appends a card with the given title to the end of a column.
func (s *Store) CreateCard(ctx context.Context, columnID int64, title string) (Card, error) {
	in := CardInput{Title: title}
	if err := in.normalize(); err != nil {
		return Card{}, err
	}
	var id int64
	now := s.now().Unix()
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM columns WHERE id = ?)`, columnID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO cards (column_id, title, position, created_at, updated_at)
			 VALUES (?, ?, (SELECT COUNT(*) FROM cards WHERE column_id = ?), ?, ?)`,
			columnID, in.Title, columnID, now, now)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return Card{}, err
	}
	return s.GetCard(ctx, id)
}

func (s *Store) UpdateCard(ctx context.Context, id int64, in CardInput) (Card, error) {
	if err := in.normalize(); err != nil {
		return Card{}, err
	}
	if err := affectedOrNotFound(s.db.ExecContext(ctx,
		`UPDATE cards SET title = ?, description = ?, priority = ?, due_date = ?, updated_at = ? WHERE id = ?`,
		in.Title, in.Description, in.Priority, in.DueDate, s.now().Unix(), id)); err != nil {
		return Card{}, err
	}
	return s.GetCard(ctx, id)
}

// RenameCard changes only the title (inline edit).
func (s *Store) RenameCard(ctx context.Context, id int64, title string) (Card, error) {
	c, err := s.GetCard(ctx, id)
	if err != nil {
		return Card{}, err
	}
	return s.UpdateCard(ctx, id, CardInput{Title: title, Description: c.Description, Priority: c.Priority, DueDate: c.DueDate})
}

func (s *Store) DeleteCard(ctx context.Context, id int64) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		var col int64
		var pos int
		if err := tx.QueryRowContext(ctx, `SELECT column_id, position FROM cards WHERE id = ?`, id).Scan(&col, &pos); err != nil {
			return orNotFound(err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM cards WHERE id = ?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, cardsTable.shiftDown, col, pos)
		return err
	})
}

// MoveCard places a card at position inside columnID. The target column must
// belong to the same board; out of range positions are clamped.
func (s *Store) MoveCard(ctx context.Context, id, columnID int64, position int) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		var fromCol, fromBoard int64
		var fromPos int
		if err := tx.QueryRowContext(ctx,
			`SELECT c.column_id, c.position, col.board_id FROM cards c JOIN columns col ON col.id = c.column_id WHERE c.id = ?`, id).
			Scan(&fromCol, &fromPos, &fromBoard); err != nil {
			return orNotFound(err)
		}
		var toBoard int64
		if err := tx.QueryRowContext(ctx, `SELECT board_id FROM columns WHERE id = ?`, columnID).Scan(&toBoard); err != nil {
			return orNotFound(err)
		}
		if toBoard != fromBoard {
			return ValidationError{"column_id": "coluna de outro projeto"}
		}
		if err := reposition(ctx, tx, cardsTable, id, fromCol, fromPos, columnID, position); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE cards SET updated_at = ? WHERE id = ?`, s.now().Unix(), id)
		return err
	})
}

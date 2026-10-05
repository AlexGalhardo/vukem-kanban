package store

import (
	"context"
	"strings"
)

// Filter narrows the cards shown on a board. Zero value shows everything.
type Filter struct {
	Query       string
	LabelID     int64
	Priority    int
	HasPriority bool
	Overdue     bool
	Today       string // YYYY-MM-DD, required when Overdue is set
}

// Active reports whether any filter is applied.
func (f Filter) Active() bool {
	return f.Query != "" || f.LabelID != 0 || f.HasPriority || f.Overdue
}

// BoardView is everything needed to render one board.
type BoardView struct {
	Board   Board
	Columns []Column
	Labels  []Label
	Filter  Filter
	Total   int // cards on the board, ignoring the filter
	Shown   int // cards matching the filter
}

// LoadBoard reads a board with its columns, matching cards and labels using a
// fixed number of queries regardless of board size (no N+1).
func (s *Store) LoadBoard(ctx context.Context, id int64, f Filter) (BoardView, error) {
	b, err := s.GetBoard(ctx, id)
	if err != nil {
		return BoardView{}, err
	}
	v := BoardView{Board: b, Filter: f}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, board_id, name, position, wip_limit FROM columns WHERE board_id = ? ORDER BY position`, id)
	if err != nil {
		return BoardView{}, err
	}
	defer rows.Close()
	index := map[int64]int{}
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.ID, &c.BoardID, &c.Name, &c.Position, &c.WIPLimit); err != nil {
			return BoardView{}, err
		}
		index[c.ID] = len(v.Columns)
		v.Columns = append(v.Columns, c)
	}
	if err := rows.Err(); err != nil {
		return BoardView{}, err
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM cards c JOIN columns col ON col.id = c.column_id WHERE col.board_id = ?`, id).
		Scan(&v.Total); err != nil {
		return BoardView{}, err
	}

	where, args := []string{"col.board_id = ?"}, []any{id}
	if f.Query != "" {
		like := "%" + escapeLike(f.Query) + "%"
		where = append(where, `(c.title LIKE ? ESCAPE '\' OR c.description LIKE ? ESCAPE '\')`)
		args = append(args, like, like)
	}
	if f.HasPriority {
		where = append(where, "c.priority = ?")
		args = append(args, f.Priority)
	}
	if f.Overdue {
		where = append(where, "c.due_date <> '' AND c.due_date < ?")
		args = append(args, f.Today)
	}
	if f.LabelID != 0 {
		where = append(where, "EXISTS (SELECT 1 FROM card_labels cl WHERE cl.card_id = c.id AND cl.label_id = ?)")
		args = append(args, f.LabelID)
	}
	// Every fragment above is a constant; user input only travels as args.
	query := `SELECT ` + cardCols + ` FROM cards c JOIN columns col ON col.id = c.column_id WHERE ` + //nolint:gosec // constant fragments only
		strings.Join(where, " AND ") + ` ORDER BY c.column_id, c.position`
	cardRows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return BoardView{}, err
	}
	defer cardRows.Close()
	cardAt := map[int64][2]int{}
	for cardRows.Next() {
		c, err := scanCard(cardRows)
		if err != nil {
			return BoardView{}, err
		}
		ci := index[c.ColumnID]
		cardAt[c.ID] = [2]int{ci, len(v.Columns[ci].Cards)}
		v.Columns[ci].Cards = append(v.Columns[ci].Cards, c)
		v.Shown++
	}
	if err := cardRows.Err(); err != nil {
		return BoardView{}, err
	}

	if v.Labels, err = s.ListLabels(ctx, id); err != nil {
		return BoardView{}, err
	}
	byID := map[int64]Label{}
	for _, l := range v.Labels {
		byID[l.ID] = l
	}
	lrows, err := s.db.QueryContext(ctx,
		`SELECT cl.card_id, cl.label_id FROM card_labels cl JOIN labels l ON l.id = cl.label_id
		 WHERE l.board_id = ? ORDER BY l.name COLLATE NOCASE`, id)
	if err != nil {
		return BoardView{}, err
	}
	defer lrows.Close()
	for lrows.Next() {
		var cardID, labelID int64
		if err := lrows.Scan(&cardID, &labelID); err != nil {
			return BoardView{}, err
		}
		if at, ok := cardAt[cardID]; ok {
			card := &v.Columns[at[0]].Cards[at[1]]
			card.Labels = append(card.Labels, byID[labelID])
		}
	}
	return v, lrows.Err()
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

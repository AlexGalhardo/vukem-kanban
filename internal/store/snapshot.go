package store

import (
	"context"
	"database/sql"
	"fmt"
)

// SnapshotVersion is bumped whenever the JSON layout changes incompatibly.
const SnapshotVersion = 1

// Snapshot is the portable JSON export of every board. Nesting replaces IDs,
// so importing never needs to remap keys; card labels point into the board's
// Labels slice by index.
type Snapshot struct {
	Version int             `json:"version"`
	Boards  []BoardSnapshot `json:"boards"`
}

type BoardSnapshot struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Color       string           `json:"color"`
	Archived    bool             `json:"archived"`
	Labels      []LabelInput     `json:"labels"`
	Columns     []ColumnSnapshot `json:"columns"`
}

type ColumnSnapshot struct {
	Name     string         `json:"name"`
	WIPLimit int            `json:"wip_limit"`
	Cards    []CardSnapshot `json:"cards"`
}

type CardSnapshot struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	DueDate     string `json:"due_date"`
	Labels      []int  `json:"labels"`
}

// Export returns every board, active and archived, ordered by id.
func (s *Store) Export(ctx context.Context) (Snapshot, error) {
	snap := Snapshot{Version: SnapshotVersion, Boards: []BoardSnapshot{}}
	ids, err := s.boardIDs(ctx)
	if err != nil {
		return snap, err
	}
	for _, id := range ids {
		v, err := s.LoadBoard(ctx, id, Filter{})
		if err != nil {
			return snap, err
		}
		b := BoardSnapshot{
			Name: v.Board.Name, Description: v.Board.Description, Color: v.Board.Color, Archived: v.Board.Archived,
			Labels: []LabelInput{}, Columns: []ColumnSnapshot{},
		}
		labelIdx := map[int64]int{}
		for i, l := range v.Labels {
			labelIdx[l.ID] = i
			b.Labels = append(b.Labels, LabelInput{Name: l.Name, Color: l.Color})
		}
		for _, col := range v.Columns {
			cs := ColumnSnapshot{Name: col.Name, WIPLimit: col.WIPLimit, Cards: []CardSnapshot{}}
			for _, c := range col.Cards {
				card := CardSnapshot{Title: c.Title, Description: c.Description, Priority: c.Priority, DueDate: c.DueDate, Labels: []int{}}
				for _, l := range c.Labels {
					card.Labels = append(card.Labels, labelIdx[l.ID])
				}
				cs.Cards = append(cs.Cards, card)
			}
			b.Columns = append(b.Columns, cs)
		}
		snap.Boards = append(snap.Boards, b)
	}
	return snap, nil
}

// Import validates the whole snapshot and writes it in a single transaction.
// With replace, existing boards are deleted first; otherwise boards are added.
func (s *Store) Import(ctx context.Context, snap Snapshot, replace bool) error {
	if snap.Version != SnapshotVersion {
		return ValidationError{"file": fmt.Sprintf("versão %d não suportada", snap.Version)}
	}
	for bi := range snap.Boards {
		b := &snap.Boards[bi]
		in := BoardInput{Name: b.Name, Description: b.Description, Color: b.Color}
		if err := in.normalize(); err != nil {
			return prefixed(fmt.Sprintf("projeto %d", bi+1), err)
		}
		b.Name, b.Description, b.Color = in.Name, in.Description, in.Color
		for li := range b.Labels {
			if err := b.Labels[li].normalize(); err != nil {
				return prefixed(fmt.Sprintf("projeto %q, label %d", b.Name, li+1), err)
			}
		}
		for ci := range b.Columns {
			col := &b.Columns[ci]
			cin := ColumnInput{Name: col.Name, WIPLimit: col.WIPLimit}
			if err := cin.normalize(); err != nil {
				return prefixed(fmt.Sprintf("projeto %q, coluna %d", b.Name, ci+1), err)
			}
			col.Name = cin.Name
			for ki := range col.Cards {
				c := &col.Cards[ki]
				kin := CardInput{Title: c.Title, Description: c.Description, Priority: c.Priority, DueDate: c.DueDate}
				if err := kin.normalize(); err != nil {
					return prefixed(fmt.Sprintf("projeto %q, card %d", b.Name, ki+1), err)
				}
				c.Title, c.Description, c.DueDate = kin.Title, kin.Description, kin.DueDate
				for _, li := range c.Labels {
					if li < 0 || li >= len(b.Labels) {
						return ValidationError{"file": fmt.Sprintf("projeto %q: card %q aponta para label inexistente", b.Name, c.Title)}
					}
				}
			}
		}
	}

	now := s.now().Unix()
	return s.tx(ctx, func(tx *sql.Tx) error {
		if replace {
			if _, err := tx.ExecContext(ctx, `DELETE FROM boards`); err != nil {
				return err
			}
		}
		for _, b := range snap.Boards {
			var archivedAt any
			if b.Archived {
				archivedAt = now
			}
			boardID, err := insertID(ctx, tx,
				`INSERT INTO boards (name, description, color, archived_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
				b.Name, b.Description, b.Color, archivedAt, now, now)
			if err != nil {
				return err
			}
			labelIDs := make([]int64, len(b.Labels))
			for i, l := range b.Labels {
				if labelIDs[i], err = insertID(ctx, tx,
					`INSERT INTO labels (board_id, name, color) VALUES (?, ?, ?)`, boardID, l.Name, l.Color); err != nil {
					return err
				}
			}
			for pos, col := range b.Columns {
				colID, err := insertID(ctx, tx,
					`INSERT INTO columns (board_id, name, position, wip_limit, created_at) VALUES (?, ?, ?, ?, ?)`,
					boardID, col.Name, pos, col.WIPLimit, now)
				if err != nil {
					return err
				}
				for cpos, c := range col.Cards {
					cardID, err := insertID(ctx, tx,
						`INSERT INTO cards (column_id, title, description, priority, due_date, position, created_at, updated_at)
						 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
						colID, c.Title, c.Description, c.Priority, c.DueDate, cpos, now, now)
					if err != nil {
						return err
					}
					for _, li := range c.Labels {
						if _, err := tx.ExecContext(ctx,
							`INSERT OR IGNORE INTO card_labels (card_id, label_id) VALUES (?, ?)`, cardID, labelIDs[li]); err != nil {
							return err
						}
					}
				}
			}
		}
		return nil
	})
}

func (s *Store) boardIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM boards ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func insertID(ctx context.Context, tx *sql.Tx, query string, args ...any) (int64, error) {
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func prefixed(where string, err error) error {
	if v, ok := err.(ValidationError); ok { //nolint:errorlint // normalize returns the concrete type, never wrapped
		return ValidationError{"file": fmt.Sprintf("%s: %s", where, v.Error())}
	}
	return err
}

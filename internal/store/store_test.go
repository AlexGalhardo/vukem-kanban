package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// tempDir is t.TempDir with a retrying cleanup: on Windows SQLite's WAL/SHM
// files can stay locked for a moment after Close, which made t.TempDir's
// cleanup fail intermittently with "directory not empty".
func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "kanban-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for range 40 {
			if os.RemoveAll(dir) == nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Logf("could not remove %s", dir)
	})
	return dir
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(tempDir(t), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustBoard(t *testing.T, s *Store, name string) BoardView {
	t.Helper()
	b, err := s.CreateBoard(context.Background(), BoardInput{Name: name})
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	v, err := s.LoadBoard(context.Background(), b.ID, Filter{})
	if err != nil {
		t.Fatalf("load board: %v", err)
	}
	return v
}

func mustCard(t *testing.T, s *Store, colID int64, title string) Card {
	t.Helper()
	c, err := s.CreateCard(context.Background(), colID, title)
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	return c
}

// titles returns card titles per column, the observable order of a board.
func titles(t *testing.T, s *Store, boardID int64) [][]string {
	t.Helper()
	v, err := s.LoadBoard(context.Background(), boardID, Filter{})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	out := make([][]string, len(v.Columns))
	for i, c := range v.Columns {
		out[i] = []string{}
		for j, card := range c.Cards {
			if card.Position != j {
				t.Fatalf("column %q: card %q has position %d, want %d", c.Name, card.Title, card.Position, j)
			}
			out[i] = append(out[i], card.Title)
		}
	}
	return out
}

func TestMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(tempDir(t), "m.db")
	for i := range 2 {
		s, err := Open(context.Background(), path)
		if err != nil {
			t.Fatalf("open #%d: %v", i, err)
		}
		var fk int
		if err := s.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
			t.Fatalf("foreign_keys = %d, err %v", fk, err)
		}
		var mode string
		if err := s.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
			t.Fatalf("journal_mode = %q, err %v", mode, err)
		}
		_ = s.Close()
	}
}

func TestPendingMigrationBacksUpExistingDatabase(t *testing.T) {
	path := filepath.Join(tempDir(t), "old.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateBoard(context.Background(), BoardInput{Name: "Keep me"}); err != nil {
		t.Fatal(err)
	}
	// Pretend a newer migration exists by forgetting that 0001 was applied
	// after creating a sentinel version, so 0001 is "pending" on reopen.
	if _, err := s.db.Exec(`INSERT INTO schema_migrations VALUES ('0000_sentinel.sql', 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM schema_migrations WHERE version = '0001_init.sql'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DROP TABLE card_labels; DROP TABLE labels; DROP TABLE cards; DROP TABLE columns; DROP TABLE boards`); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	s, err = Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	_ = s.Close()
	if _, err := os.Stat(path + ".pre-0001_init.bak"); err != nil {
		t.Fatalf("backup not written: %v", err)
	}
}

func TestCreateBoardAddsDefaultColumns(t *testing.T) {
	s := newStore(t)
	v := mustBoard(t, s, "  Kanban  ")
	if v.Board.Name != "Kanban" || v.Board.Color != "slate" {
		t.Fatalf("board = %+v", v.Board)
	}
	var names []string
	for _, c := range v.Columns {
		names = append(names, c.Name)
	}
	if !reflect.DeepEqual(names, DefaultColumns) {
		t.Fatalf("columns = %v", names)
	}
}

func TestBoardValidation(t *testing.T) {
	s := newStore(t)
	cases := []struct {
		name  string
		in    BoardInput
		field string
	}{
		{"empty name", BoardInput{Name: "   "}, "name"},
		{"long name", BoardInput{Name: fmt.Sprintf("%081d", 0)}, "name"},
		{"bad color", BoardInput{Name: "x", Color: "#fff"}, "color"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.CreateBoard(context.Background(), tc.in)
			var v ValidationError
			if !errors.As(err, &v) || v[tc.field] == "" {
				t.Fatalf("err = %v, want validation on %s", err, tc.field)
			}
		})
	}
}

func TestBoardLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	b := mustBoard(t, s, "b").Board
	if _, err := s.UpdateBoard(ctx, b.ID, BoardInput{Name: "renamed", Description: "d", Color: "green"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBoardArchived(ctx, b.ID, true); err != nil {
		t.Fatal(err)
	}
	active, _ := s.ListBoards(ctx, false)
	archived, _ := s.ListBoards(ctx, true)
	if len(active) != 0 || len(archived) != 1 || archived[0].Name != "renamed" || !archived[0].Archived {
		t.Fatalf("active=%v archived=%v", active, archived)
	}
	if err := s.SetBoardArchived(ctx, b.ID, false); err != nil {
		t.Fatal(err)
	}
	if active, _ = s.ListBoards(ctx, false); len(active) != 1 {
		t.Fatalf("restore failed: %v", active)
	}
	if err := s.DeleteBoard(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetBoard(ctx, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted = %v", err)
	}
	if err := s.DeleteBoard(ctx, b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice = %v", err)
	}
}

func TestDeleteBoardCascades(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	v := mustBoard(t, s, "b")
	c := mustCard(t, s, v.Columns[0].ID, "card")
	l, err := s.CreateLabel(ctx, v.Board.ID, LabelInput{Name: "bug", Color: "red"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ToggleCardLabel(ctx, c.ID, l.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteBoard(ctx, v.Board.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"columns", "cards", "labels", "card_labels"} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s rows = %d, err %v", table, n, err)
		}
	}
}

func TestCardCRUD(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	v := mustBoard(t, s, "b")
	c := mustCard(t, s, v.Columns[0].ID, "first")
	if c.BoardID != v.Board.ID || c.Position != 0 {
		t.Fatalf("card = %+v", c)
	}
	c, err := s.UpdateCard(ctx, c.ID, CardInput{Title: "t", Description: "line1\nline2", Priority: 2, DueDate: "2026-01-31"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Priority != 2 || c.DueDate != "2026-01-31" || c.Description != "line1\nline2" {
		t.Fatalf("updated = %+v", c)
	}
	c, err = s.RenameCard(ctx, c.ID, "renamed")
	if err != nil || c.Title != "renamed" || c.Priority != 2 {
		t.Fatalf("rename = %+v, %v", c, err)
	}
	if !c.Overdue("2026-02-01") || c.Overdue("2026-01-31") {
		t.Fatal("overdue logic")
	}
	for _, bad := range []CardInput{{Title: ""}, {Title: "x", Priority: 5}, {Title: "x", DueDate: "31/01/2026"}} {
		if _, err := s.UpdateCard(ctx, c.ID, bad); err == nil {
			t.Fatalf("expected validation error for %+v", bad)
		}
	}
	if _, err := s.CreateCard(ctx, 9999, "orphan"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("create in missing column = %v", err)
	}
	mustCard(t, s, v.Columns[0].ID, "second")
	if err := s.DeleteCard(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if got := titles(t, s, v.Board.ID)[0]; !reflect.DeepEqual(got, []string{"second"}) {
		t.Fatalf("after delete = %v", got)
	}
}

func TestMoveCard(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	v := mustBoard(t, s, "b")
	a, b := v.Columns[0].ID, v.Columns[1].ID
	ids := map[string]int64{}
	for _, n := range []string{"1", "2", "3"} {
		ids[n] = mustCard(t, s, a, n).ID
	}
	ids["x"] = mustCard(t, s, b, "x").ID

	steps := []struct {
		card string
		col  int64
		pos  int
		want [][]string
	}{
		{"3", a, 0, [][]string{{"3", "1", "2"}, {"x"}}},
		{"3", a, 2, [][]string{{"1", "2", "3"}, {"x"}}},
		{"1", b, 0, [][]string{{"2", "3"}, {"1", "x"}}},
		{"2", b, 99, [][]string{{"3"}, {"1", "x", "2"}}},
		{"x", a, -5, [][]string{{"x", "3"}, {"1", "2"}}},
	}
	for i, st := range steps {
		if err := s.MoveCard(ctx, ids[st.card], st.col, st.pos); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		got := titles(t, s, v.Board.ID)[:2]
		if !reflect.DeepEqual(got, st.want) {
			t.Fatalf("step %d: got %v, want %v", i, got, st.want)
		}
	}

	other := mustBoard(t, s, "other")
	var verr ValidationError
	if err := s.MoveCard(ctx, ids["x"], other.Columns[0].ID, 0); !errors.As(err, &verr) {
		t.Fatalf("cross-board move = %v", err)
	}
}

func TestColumns(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	v := mustBoard(t, s, "b")
	col, err := s.CreateColumn(ctx, v.Board.ID, ColumnInput{Name: "Deploy", WIPLimit: 2})
	if err != nil || col.Position != len(DefaultColumns) {
		t.Fatalf("create column = %+v, %v", col, err)
	}
	if col, err = s.UpdateColumn(ctx, col.ID, ColumnInput{Name: "Prod", WIPLimit: 1}); err != nil || col.Name != "Prod" {
		t.Fatalf("update = %+v, %v", col, err)
	}
	if err := s.MoveColumn(ctx, col.ID, 0); err != nil {
		t.Fatal(err)
	}
	v, _ = s.LoadBoard(ctx, v.Board.ID, Filter{})
	if v.Columns[0].Name != "Prod" || v.Columns[1].Name != "Backlog" {
		t.Fatalf("order after move: %v, %v", v.Columns[0].Name, v.Columns[1].Name)
	}
	for i, c := range v.Columns {
		if c.Position != i {
			t.Fatalf("column %q position %d want %d", c.Name, c.Position, i)
		}
	}

	mustCard(t, s, col.ID, "c1")
	mustCard(t, s, col.ID, "c2")
	v, _ = s.LoadBoard(ctx, v.Board.ID, Filter{})
	if !v.Columns[0].OverLimit() {
		t.Fatal("expected over WIP limit")
	}
	if err := s.DeleteColumn(ctx, col.ID, 0); !errors.Is(err, ErrColumnNotEmpty) {
		t.Fatalf("delete non-empty = %v", err)
	}
	backlog := v.Columns[1].ID
	mustCard(t, s, backlog, "b1")
	if err := s.DeleteColumn(ctx, col.ID, backlog); err != nil {
		t.Fatal(err)
	}
	got := titles(t, s, v.Board.ID)
	if len(got) != len(DefaultColumns) || !reflect.DeepEqual(got[0], []string{"b1", "c1", "c2"}) {
		t.Fatalf("after delete with move: %v", got)
	}
	empty := v.Columns[len(v.Columns)-1].ID
	if err := s.DeleteColumn(ctx, empty, 0); err != nil {
		t.Fatalf("delete empty: %v", err)
	}
}

func TestLabelsAndFilter(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	v := mustBoard(t, s, "b")
	col := v.Columns[0].ID
	bug, _ := s.CreateLabel(ctx, v.Board.ID, LabelInput{Name: "bug", Color: "red"})
	feat, _ := s.CreateLabel(ctx, v.Board.ID, LabelInput{Name: "feat", Color: "green"})

	login := mustCard(t, s, col, "Fix login")
	_, _ = s.UpdateCard(ctx, login.ID, CardInput{Title: "Fix login", Priority: 1, DueDate: "2026-01-01"})
	search := mustCard(t, s, col, "Add search")
	_, _ = s.UpdateCard(ctx, search.ID, CardInput{Title: "Add search", Description: "100% full text"})
	mustCard(t, s, col, "Docs")

	if err := s.ToggleCardLabel(ctx, login.ID, bug.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ToggleCardLabel(ctx, search.ID, feat.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ToggleCardLabel(ctx, search.ID, bug.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ToggleCardLabel(ctx, search.ID, bug.ID); err != nil { // detach again
		t.Fatal(err)
	}
	c, _ := s.GetCard(ctx, search.ID)
	if len(c.Labels) != 1 || !c.HasLabel(feat.ID) {
		t.Fatalf("labels = %+v", c.Labels)
	}

	cases := []struct {
		name string
		f    Filter
		want []string
	}{
		{"none", Filter{}, []string{"Fix login", "Add search", "Docs"}},
		{"query case-insensitive", Filter{Query: "LOGIN"}, []string{"Fix login"}},
		{"query in description", Filter{Query: "full text"}, []string{"Add search"}},
		{"percent is literal", Filter{Query: "100%"}, []string{"Add search"}},
		{"underscore is literal", Filter{Query: "_"}, nil},
		{"label", Filter{LabelID: bug.ID}, []string{"Fix login"}},
		{"priority none", Filter{HasPriority: true, Priority: 0}, []string{"Add search", "Docs"}},
		{"overdue", Filter{Overdue: true, Today: "2026-06-01"}, []string{"Fix login"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := s.LoadBoard(ctx, v.Board.ID, tc.f)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, c := range v.Columns[0].Cards {
				got = append(got, c.Title)
			}
			if !reflect.DeepEqual(got, tc.want) || v.Total != 3 || v.Shown != len(tc.want) {
				t.Fatalf("got %v (total %d shown %d), want %v", got, v.Total, v.Shown, tc.want)
			}
			if tc.f.Active() == (tc.name == "none") {
				t.Fatalf("Active() = %v", tc.f.Active())
			}
		})
	}

	if _, err := s.UpdateLabel(ctx, bug.ID, LabelInput{Name: "defect", Color: "orange"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLabel(ctx, bug.ID); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.GetCard(ctx, login.ID); len(c.Labels) != 0 {
		t.Fatalf("label not detached on delete: %+v", c.Labels)
	}
	other := mustBoard(t, s, "other")
	foreign, _ := s.CreateLabel(ctx, other.Board.ID, LabelInput{Name: "x", Color: "sky"})
	var verr ValidationError
	if err := s.ToggleCardLabel(ctx, login.ID, foreign.ID); !errors.As(err, &verr) {
		t.Fatalf("cross-board label = %v", err)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	v := mustBoard(t, s, "Alpha")
	l, _ := s.CreateLabel(ctx, v.Board.ID, LabelInput{Name: "bug", Color: "red"})
	c := mustCard(t, s, v.Columns[2].ID, "card")
	_, _ = s.UpdateCard(ctx, c.ID, CardInput{Title: "card", Description: "d", Priority: 3, DueDate: "2026-05-05"})
	_ = s.ToggleCardLabel(ctx, c.ID, l.ID)
	archived := mustBoard(t, s, "Old")
	_ = s.SetBoardArchived(ctx, archived.Board.ID, true)

	first, err := s.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}

	empty := newStore(t)
	if err := empty.Import(ctx, first, true); err != nil {
		t.Fatal(err)
	}
	second, err := empty.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("round trip differs:\n%+v\n%+v", first, second)
	}

	if err := empty.Import(ctx, first, false); err != nil {
		t.Fatal(err)
	}
	if third, _ := empty.Export(ctx); len(third.Boards) != 4 {
		t.Fatalf("merge import boards = %d", len(third.Boards))
	}
	if err := empty.Import(ctx, first, true); err != nil {
		t.Fatal(err)
	}
	if fourth, _ := empty.Export(ctx); len(fourth.Boards) != 2 {
		t.Fatalf("replace import boards = %d", len(fourth.Boards))
	}

	bad := first
	bad.Boards = append([]BoardSnapshot(nil), first.Boards...)
	bad.Boards[0].Columns = []ColumnSnapshot{{Name: "c", Cards: []CardSnapshot{{Title: "t", Labels: []int{7}}}}}
	var verr ValidationError
	if err := empty.Import(ctx, bad, true); !errors.As(err, &verr) {
		t.Fatalf("bad label index = %v", err)
	}
	if err := empty.Import(ctx, Snapshot{Version: 99}, true); !errors.As(err, &verr) {
		t.Fatalf("bad version = %v", err)
	}
	if after, _ := empty.Export(ctx); len(after.Boards) != 2 {
		t.Fatal("failed import must not change data")
	}
}

func TestBackup(t *testing.T) {
	s := newStore(t)
	mustBoard(t, s, "b")
	dest := filepath.Join(tempDir(t), "copy.db")
	if err := s.Backup(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
	c, err := Open(context.Background(), dest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if bs, _ := c.ListBoards(context.Background(), false); len(bs) != 1 {
		t.Fatalf("backup boards = %d", len(bs))
	}
}

func TestSearchFoldsUnicodeCase(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	v := mustBoard(t, s, "b")
	mustCard(t, s, v.Columns[0].ID, "BUG ÇÃO")
	mustCard(t, s, v.Columns[0].ID, "outro")
	for _, q := range []string{"ção", "ÇÃO", "bug"} {
		got, err := s.LoadBoard(ctx, v.Board.ID, Filter{Query: q})
		if err != nil || got.Shown != 1 {
			t.Fatalf("query %q shown = %d, err %v", q, got.Shown, err)
		}
	}
}

func TestCardTitleIsSingleLine(t *testing.T) {
	s := newStore(t)
	v := mustBoard(t, s, "b")
	c := mustCard(t, s, v.Columns[0].ID, "  linha 1\nlinha\t2  ")
	if c.Title != "linha 1 linha 2" {
		t.Fatalf("title = %q", c.Title)
	}
}

// TestMoveCardBeforeWithHiddenCards reproduces a drop while a filter hides
// cards: the client only knows the visible neighbour it dropped above.
func TestMoveCardBeforeWithHiddenCards(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	v := mustBoard(t, s, "b")
	col := v.Columns[0].ID
	ids := map[string]int64{}
	for _, n := range []string{"x1", "hid", "x2", "x3"} {
		ids[n] = mustCard(t, s, col, n).ID
	}
	if err := s.MoveCardBefore(ctx, ids["x1"], col, ids["x3"]); err != nil {
		t.Fatal(err)
	}
	if got := titles(t, s, v.Board.ID)[0]; !reflect.DeepEqual(got, []string{"hid", "x2", "x1", "x3"}) {
		t.Fatalf("order = %v", got)
	}
	if err := s.MoveCardBefore(ctx, ids["x3"], col, ids["hid"]); err != nil {
		t.Fatal(err)
	}
	if got := titles(t, s, v.Board.ID)[0]; !reflect.DeepEqual(got, []string{"x3", "hid", "x2", "x1"}) {
		t.Fatalf("order = %v", got)
	}
	other := v.Columns[1].ID
	if err := s.MoveCardBefore(ctx, ids["hid"], other, 0); err != nil {
		t.Fatal(err)
	}
	var verr ValidationError
	if err := s.MoveCardBefore(ctx, ids["x2"], other, ids["x1"]); !errors.As(err, &verr) {
		t.Fatalf("anchor in another column = %v", err)
	}
	if err := s.MoveCardBefore(ctx, ids["x2"], col, 9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing anchor = %v", err)
	}
}

func TestValidationMessage(t *testing.T) {
	v := ValidationError{"title": "obrigatório", "due_date": "data inválida"}
	if got := v.Message(); got != "Entrega: data inválida; Título: obrigatório" {
		t.Fatalf("message = %q", got)
	}
}

func TestNotFoundPaths(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	checks := map[string]error{}
	_, checks["UpdateBoard"] = s.UpdateBoard(ctx, 99, BoardInput{Name: "x"})
	checks["SetBoardArchived"] = s.SetBoardArchived(ctx, 99, true)
	_, checks["CreateColumn"] = s.CreateColumn(ctx, 99, ColumnInput{Name: "x"})
	_, checks["UpdateColumn"] = s.UpdateColumn(ctx, 99, ColumnInput{Name: "x"})
	checks["DeleteColumn"] = s.DeleteColumn(ctx, 99, 0)
	checks["MoveColumn"] = s.MoveColumn(ctx, 99, 0)
	_, checks["GetCard"] = s.GetCard(ctx, 99)
	_, checks["UpdateCard"] = s.UpdateCard(ctx, 99, CardInput{Title: "x"})
	_, checks["RenameCard"] = s.RenameCard(ctx, 99, "x")
	checks["DeleteCard"] = s.DeleteCard(ctx, 99)
	checks["MoveCard"] = s.MoveCard(ctx, 99, 1, 0)
	_, checks["CreateLabel"] = s.CreateLabel(ctx, 99, LabelInput{Name: "x", Color: "red"})
	_, checks["UpdateLabel"] = s.UpdateLabel(ctx, 99, LabelInput{Name: "x", Color: "red"})
	checks["DeleteLabel"] = s.DeleteLabel(ctx, 99)
	checks["ToggleCardLabel"] = s.ToggleCardLabel(ctx, 99, 99)
	_, checks["LoadBoard"] = s.LoadBoard(ctx, 99, Filter{})
	for name, err := range checks {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
	var verr ValidationError
	if _, err := s.CreateColumn(ctx, 1, ColumnInput{Name: "", WIPLimit: -1}); !errors.As(err, &verr) || len(verr) != 2 {
		t.Fatalf("column validation = %v", err)
	}
	if _, err := s.CreateLabel(ctx, 1, LabelInput{Name: "x", Color: "nope"}); !errors.As(err, &verr) {
		t.Fatalf("label validation = %v", err)
	}
}

// TestOpensDatabaseFromV040Alpha1 guards data compatibility: a database
// written by the released v0.4.0-alpha.1 binary must open, migrate and keep
// every record. Exports from that version (labels used "Name"/"Color" keys)
// must still import.
func TestOpensDatabaseFromV040Alpha1(t *testing.T) {
	ctx := context.Background()
	src, err := os.ReadFile(filepath.Join("testdata", "v0.4.0-alpha.1.db"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tempDir(t), "legacy.db")
	if err := os.WriteFile(path, src, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer func() { _ = s.Close() }()

	raw, err := os.ReadFile(filepath.Join("testdata", "v0.4.0-alpha.1.export.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want Snapshot
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy data changed:\n got %+v\nwant %+v", got, want)
	}
	if want.Boards[0].Labels[0].Name != "bug" {
		t.Fatalf("legacy label keys not decoded: %+v", want.Boards[0].Labels)
	}

	fresh := newStore(t)
	if err := fresh.Import(ctx, want, true); err != nil {
		t.Fatalf("import legacy export: %v", err)
	}
}

func TestUpdateCardDetectsStaleRevision(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	v := mustBoard(t, s, "b")
	c := mustCard(t, s, v.Columns[0].ID, "card")
	loaded := c.Revision

	updated, err := s.UpdateCardAt(ctx, c.ID, CardInput{Title: "card", Description: "important text"}, loaded)
	if err != nil || updated.Revision != loaded+1 {
		t.Fatalf("first save = %+v, %v", updated, err)
	}
	// A second tab still holding the old revision must not overwrite it.
	if _, err := s.UpdateCardAt(ctx, c.ID, CardInput{Title: "stale"}, loaded); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale save = %v, want ErrConflict", err)
	}
	if got, _ := s.GetCard(ctx, c.ID); got.Description != "important text" || got.Title != "card" {
		t.Fatalf("stale save changed data: %+v", got)
	}
	if _, err := s.UpdateCardAt(ctx, 9999, CardInput{Title: "x"}, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing card = %v", err)
	}
}

func TestInvisibleAndInvalidTextIsRejected(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	v := mustBoard(t, s, "b")
	for _, title := range []string{"\x00", string(rune(0x200b)) + string(rune(0x200b)), string(rune(0x202e)), " \t\n"} {
		var verr ValidationError
		if _, err := s.CreateCard(ctx, v.Columns[0].ID, title); !errors.As(err, &verr) {
			t.Errorf("title %q accepted: %v", title, err)
		}
	}
	c, err := s.CreateCard(ctx, v.Columns[0].ID, "ok"+string([]byte{0xff})+string(rune(0x200b))+"título")
	if err != nil || c.Title != "oktítulo" {
		t.Fatalf("cleaned title = %q, %v", c.Title, err)
	}
	b, _ := s.CreateBoard(ctx, BoardInput{Name: "a\nb\x00"})
	if b.Name != "a b" {
		t.Fatalf("board name = %q", b.Name)
	}
}

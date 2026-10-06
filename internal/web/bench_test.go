package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/trello-golang-htmx/internal/store"
)

// seedBoard creates a board with n cards spread over the default columns.
func seedBoard(tb testing.TB, a *testApp, n int) store.BoardView {
	tb.Helper()
	ctx := context.Background()
	b, err := a.store.CreateBoard(ctx, store.BoardInput{Name: "Perf"})
	if err != nil {
		tb.Fatal(err)
	}
	v, _ := a.store.LoadBoard(ctx, b.ID, store.Filter{})
	l, _ := a.store.CreateLabel(ctx, b.ID, store.LabelInput{Name: "bug", Color: "red"})
	for i := range n {
		c, err := a.store.CreateCard(ctx, v.Columns[i%len(v.Columns)].ID, fmt.Sprintf("Card %d com um título razoável", i))
		if err != nil {
			tb.Fatal(err)
		}
		if i%3 == 0 {
			_ = a.store.ToggleCardLabel(ctx, c.ID, l.ID)
		}
	}
	v, _ = a.store.LoadBoard(ctx, b.ID, store.Filter{})
	return v
}

func newBenchApp(b *testing.B) *testApp {
	b.Helper()
	st, err := store.Open(context.Background(), filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = st.Close() })
	srv, err := New(st, slog.New(slog.DiscardHandler), "bench")
	if err != nil {
		b.Fatal(err)
	}
	return &testApp{srv: srv, store: st}
}

// BenchmarkRenderBoard500 is the R2 budget: a 500-card board must render
// server-side well under 200 ms.
func BenchmarkRenderBoard500(b *testing.B) {
	a := newBenchApp(b)
	v := seedBoard(b, a, 500)
	path := fmt.Sprintf("/boards/%d", v.Board.ID)
	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		a.srv.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			b.Fatal(w.Code)
		}
	}
}

// BenchmarkMoveCard500 is the R2 budget for a drag: well under 50 ms.
func BenchmarkMoveCard500(b *testing.B) {
	a := newBenchApp(b)
	v := seedBoard(b, a, 500)
	card := v.Columns[0].Cards[0].ID
	cols := []int64{v.Columns[1].ID, v.Columns[0].ID}
	b.ResetTimer()
	i := 0
	for b.Loop() {
		form := url.Values{"column_id": {fmt.Sprint(cols[i%2])}}
		r := httptest.NewRequest("POST", fmt.Sprintf("/cards/%d/move", card), strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		a.srv.ServeHTTP(w, r)
		if w.Code != 200 {
			b.Fatal(w.Code, w.Body.String())
		}
		i++
	}
}

// TestPerformanceBudgets enforces the R2 budgets with generous headroom so
// the test stays stable on slow machines while still catching regressions
// such as an N+1 query (which would be orders of magnitude slower).
func TestPerformanceBudgets(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("performance budgets need a normal build (no -short, no -race)")
	}
	a := newApp(t)
	v := seedBoard(t, a, 500)
	measure := func(n int, fn func()) time.Duration {
		fn() // warm up template and SQLite caches
		start := time.Now()
		for range n {
			fn()
		}
		return time.Since(start) / time.Duration(n)
	}
	board := measure(5, func() {
		if w := a.do("GET", fmt.Sprintf("/boards/%d", v.Board.ID), nil); w.Code != 200 {
			t.Fatal(w.Code)
		}
	})
	card := v.Columns[0].Cards[0].ID
	i := 0
	move := measure(10, func() {
		w := a.do("POST", fmt.Sprintf("/cards/%d/move", card), url.Values{"column_id": {fmt.Sprint(v.Columns[i%2].ID)}}, hx)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		i++
	})
	t.Logf("500 cards: render board %v, move card %v", board, move)
	if board > 200*time.Millisecond {
		t.Errorf("render board = %v, budget 200ms", board)
	}
	if move > 50*time.Millisecond {
		t.Errorf("move card = %v, budget 50ms", move)
	}

	w := a.do("POST", fmt.Sprintf("/cards/%d/move", card), url.Values{"column_id": {fmt.Sprint(v.Columns[1].ID)}}, hx)
	full := a.do("GET", fmt.Sprintf("/boards/%d", v.Board.ID), nil, hx)
	if w.Body.Len()*2 > full.Body.Len() {
		t.Errorf("move response %d bytes should be well below the board's %d", w.Body.Len(), full.Body.Len())
	}
}

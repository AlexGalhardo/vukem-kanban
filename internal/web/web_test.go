package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/trello-golang-htmx/internal/store"
)

type testApp struct {
	t     *testing.T
	srv   *Server
	store *store.Store
	logs  *bytes.Buffer
}

func newApp(t *testing.T) *testApp {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "web.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	logs := &bytes.Buffer{}
	srv, err := New(st, slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})), "test")
	if err != nil {
		t.Fatal(err)
	}
	srv.now = func() time.Time { return time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC) }
	return &testApp{t: t, srv: srv, store: st, logs: logs}
}

type reqOpt func(*http.Request)

func hx(r *http.Request) { r.Header.Set("HX-Request", "true") }

func currentURL(u string) reqOpt {
	return func(r *http.Request) { r.Header.Set("HX-Current-URL", "http://localhost"+u) }
}

func (a *testApp) do(method, target string, form url.Values, opts ...reqOpt) *httptest.ResponseRecorder {
	a.t.Helper()
	var body io.Reader
	if form != nil && method != http.MethodGet && method != http.MethodDelete {
		body = strings.NewReader(form.Encode())
	} else if form != nil {
		target += "?" + form.Encode()
	}
	r := httptest.NewRequest(method, target, body)
	if body != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, o := range opts {
		o(r)
	}
	w := httptest.NewRecorder()
	a.srv.ServeHTTP(w, r)
	return w
}

func (a *testApp) board(name string) store.BoardView {
	a.t.Helper()
	b, err := a.store.CreateBoard(context.Background(), store.BoardInput{Name: name})
	if err != nil {
		a.t.Fatal(err)
	}
	v, err := a.store.LoadBoard(context.Background(), b.ID, store.Filter{})
	if err != nil {
		a.t.Fatal(err)
	}
	return v
}

func expect(t *testing.T, w *httptest.ResponseRecorder, status int, contains ...string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d; body:\n%s", w.Code, status, w.Body.String())
	}
	for _, c := range contains {
		if !strings.Contains(w.Body.String(), c) {
			t.Fatalf("body missing %q:\n%s", c, w.Body.String())
		}
	}
}

func TestHealthzAndStatic(t *testing.T) {
	a := newApp(t)
	expect(t, a.do("GET", "/healthz", nil), 200, "ok")
	for _, f := range []string{"/static/htmx.min.js", "/static/app.js", "/static/theme.js", "/static/favicon.svg"} {
		w := a.do("GET", f, nil)
		if w.Code != 200 || !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("%s: %d %q", f, w.Code, w.Header().Get("Cache-Control"))
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	a := newApp(t)
	w := a.do("GET", "/", nil)
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "X-Request-Id"} {
		if w.Header().Get(h) == "" {
			t.Fatalf("missing header %s", h)
		}
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'self'") {
		t.Fatal("CSP must restrict scripts to self")
	}
}

func TestCrossOriginWritesAreRejected(t *testing.T) {
	a := newApp(t)
	w := a.do("POST", "/boards", url.Values{"name": {"x"}}, func(r *http.Request) {
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		r.Header.Set("Origin", "https://evil.example")
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-site POST = %d, want 403", w.Code)
	}
	if bs, _ := a.store.ListBoards(context.Background(), false); len(bs) != 0 {
		t.Fatal("board created by cross-site request")
	}
}

func TestDashboardEmptyAndRedirects(t *testing.T) {
	a := newApp(t)
	expect(t, a.do("GET", "/", nil), 200, `data-testid="empty-state"`, "<title>Kanban</title>")

	first := a.board("Alpha")
	second := a.board("Beta")
	w := a.do("GET", "/", nil)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != fmt.Sprintf("/boards/%d", first.Board.ID) {
		t.Fatalf("redirect = %d %s", w.Code, w.Header().Get("Location"))
	}
	w = a.do("GET", "/", nil, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: lastBoardCookie, Value: fmt.Sprint(second.Board.ID)})
	})
	if w.Header().Get("Location") != fmt.Sprintf("/boards/%d", second.Board.ID) {
		t.Fatalf("cookie redirect = %s", w.Header().Get("Location"))
	}
}

func TestShowBoardFullAndPartial(t *testing.T) {
	a := newApp(t)
	v := a.board("Kanban <script>alert(1)</script>")
	path := fmt.Sprintf("/boards/%d", v.Board.ID)

	full := a.do("GET", path, nil)
	expect(t, full, 200, "<!doctype html>", `id="columns"`, "Backlog", "Feito", "&lt;script&gt;alert(1)&lt;/script&gt;")
	if strings.Contains(full.Body.String(), "<script>alert(1)") {
		t.Fatal("board name not escaped")
	}
	if !strings.Contains(full.Header().Get("Set-Cookie"), lastBoardCookie) {
		t.Fatal("last board cookie not set")
	}

	part := a.do("GET", path, nil, hx)
	expect(t, part, 200, `id="columns"`, `hx-swap-oob="true"`)
	if strings.Contains(part.Body.String(), "<!doctype html>") {
		t.Fatal("partial request returned full layout")
	}
	columnsOf := func(s string) string { return regexp.MustCompile(`(?s)<div id="columns".*`).FindString(s) }
	if columnsOf(full.Body.String()) == "" || !strings.HasPrefix(columnsOf(part.Body.String()), `<div id="columns"`) {
		t.Fatal("columns fragment missing")
	}

	// htmx 4 "full" requests (hx-select, history restore) get the layout.
	fullType := a.do("GET", path, nil, hx, func(r *http.Request) { r.Header.Set("HX-Request-Type", "full") })
	expect(t, fullType, 200, "<!doctype html>")

	expect(t, a.do("GET", "/boards/999", nil), 404, "Projeto não encontrado")
	expect(t, a.do("GET", "/boards/abc", nil), 400)
	w := a.do("GET", "/boards/999", nil, hx)
	if w.Code != 404 || w.Header().Get("HX-Reswap") != "none" || !strings.Contains(w.Header().Get("HX-Trigger"), "toast") {
		t.Fatalf("htmx 404 = %d reswap=%q trigger=%q", w.Code, w.Header().Get("HX-Reswap"), w.Header().Get("HX-Trigger"))
	}
}

func TestCreateBoard(t *testing.T) {
	a := newApp(t)
	w := a.do("POST", "/boards", url.Values{"name": {""}}, hx)
	expect(t, w, http.StatusUnprocessableEntity, `data-testid="new-board-dialog"`, "obrigatório")

	w = a.do("POST", "/boards", url.Values{"name": {"Novo"}, "color": {"green"}}, hx)
	expect(t, w, http.StatusCreated, "Novo", `id="sidebar"`, `hx-swap-oob="innerHTML"`)
	// #columns lives inside the main swap: marking it OOB would make htmx
	// pull it out before #main exists and the board would render empty.
	if regexp.MustCompile(`id="columns"[^>]*hx-swap-oob`).MatchString(w.Body.String()) {
		t.Fatal("columns must not be out-of-band in the main swap")
	}
	if w.Header().Get("HX-Push-Url") == "" || w.Header().Get("HX-Retarget") != "#main" {
		t.Fatalf("headers = %v", w.Header())
	}
	bs, _ := a.store.ListBoards(context.Background(), false)
	if len(bs) != 1 || bs[0].Color != "green" {
		t.Fatalf("boards = %+v", bs)
	}

	w = a.do("POST", "/boards", url.Values{"name": {"Sem JS"}})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("non-htmx create = %d", w.Code)
	}
}

func TestBoardSettingsArchiveDelete(t *testing.T) {
	a := newApp(t)
	v := a.board("Proj")
	id := v.Board.ID
	expect(t, a.do("GET", fmt.Sprintf("/boards/%d/settings", id), nil, hx), 200, `data-testid="board-settings"`)
	expect(t, a.do("PUT", fmt.Sprintf("/boards/%d", id), url.Values{"name": {""}}, hx), 422, "obrigatório")
	expect(t, a.do("PUT", fmt.Sprintf("/boards/%d", id), url.Values{"name": {"Renamed"}, "color": {"sky"}}, hx), 200, "Renamed")

	w := a.do("POST", fmt.Sprintf("/boards/%d/archive", id), nil, hx)
	if w.Code != 204 || w.Header().Get("HX-Redirect") != "/" {
		t.Fatalf("archive = %d %v", w.Code, w.Header())
	}
	expect(t, a.do("GET", fmt.Sprintf("/boards/%d", id), nil), 200, "Arquivado", `data-testid="archived-list"`)
	w = a.do("POST", fmt.Sprintf("/boards/%d/restore", id), nil, hx)
	if w.Header().Get("HX-Redirect") != fmt.Sprintf("/boards/%d", id) {
		t.Fatalf("restore redirect = %q", w.Header().Get("HX-Redirect"))
	}

	expect(t, a.do("DELETE", fmt.Sprintf("/boards/%d", id), url.Values{"confirm_name": {"wrong"}}, hx), 422, "digite o nome exato")
	w = a.do("DELETE", fmt.Sprintf("/boards/%d", id), url.Values{"confirm_name": {"Renamed"}}, hx)
	if w.Code != 204 || w.Header().Get("HX-Redirect") != "/" {
		t.Fatalf("delete = %d", w.Code)
	}
	if _, err := a.store.GetBoard(context.Background(), id); err == nil {
		t.Fatal("board still exists")
	}
}

func TestCardFlow(t *testing.T) {
	a := newApp(t)
	v := a.board("Flow")
	col := v.Columns[0].ID

	w := a.do("POST", fmt.Sprintf("/columns/%d/cards", col), url.Values{"title": {"Primeiro <b>"}}, hx)
	expect(t, w, http.StatusCreated, `data-testid="card"`, "Primeiro &lt;b&gt;", fmt.Sprintf(`id="count-%d" hx-swap-oob="true"`, col), "FLO-")
	cards := a.mustCards(v.Board.ID)
	id := cards[0].ID

	w = a.do("POST", fmt.Sprintf("/columns/%d/cards", col), url.Values{"title": {""}}, hx)
	if w.Code != 400 || w.Header().Get("HX-Reswap") != "none" {
		t.Fatalf("empty title = %d %v", w.Code, w.Header())
	}

	expect(t, a.do("GET", fmt.Sprintf("/cards/%d", id), nil, hx), 200, `data-testid="card-dialog"`, "Primeiro &lt;b&gt;")
	expect(t, a.do("GET", fmt.Sprintf("/cards/%d/title", id), nil, hx), 200, "data-submit-on-enter")
	expect(t, a.do("PUT", fmt.Sprintf("/cards/%d/title", id), url.Values{"title": {"Renomeado"}}, hx), 200, "Renomeado")
	expect(t, a.do("GET", fmt.Sprintf("/cards/%d/card", id), nil, hx), 200, "Renomeado")

	bad := url.Values{"title": {"x"}, "priority": {"1"}, "due_date": {"amanhã"}}
	expect(t, a.do("PUT", fmt.Sprintf("/cards/%d", id), bad, hx), 422, "data inválida", `value="amanhã"`)
	good := url.Values{"title": {"Final"}, "description": {"desc"}, "priority": {"2"}, "due_date": {"2026-06-01"}}
	w = a.do("PUT", fmt.Sprintf("/cards/%d", id), good, hx)
	expect(t, w, 200, "Final", "01/06/2026", `hx-swap-oob="true"`, `id="modal" hx-swap-oob="innerHTML"`)
	c, _ := a.store.GetCard(context.Background(), id)
	if c.Priority != 2 || c.DueDate != "2026-06-01" || c.Description != "desc" {
		t.Fatalf("card = %+v", c)
	}
	// Due date before the fixed "today" (2026-06-15) renders as overdue.
	if !strings.Contains(w.Body.String(), "(vencido)") {
		t.Fatal("overdue marker missing")
	}

	w = a.do("POST", fmt.Sprintf("/cards/%d/move", id), url.Values{"column_id": {fmt.Sprint(v.Columns[2].ID)}, "position": {"0"}}, hx,
		currentURL(fmt.Sprintf("/boards/%d", v.Board.ID)))
	expect(t, w, 200, `id="columns"`)
	if c, _ = a.store.GetCard(context.Background(), id); c.ColumnID != v.Columns[2].ID {
		t.Fatalf("moved to %d", c.ColumnID)
	}
	expect(t, a.do("POST", fmt.Sprintf("/cards/%d/move", id), url.Values{"column_id": {"x"}}, hx), 400)
	expect(t, a.do("POST", fmt.Sprintf("/cards/%d/move", id), url.Values{"column_id": {fmt.Sprint(v.Columns[0].ID)}, "position": {"end"}}, hx), 200)

	w = a.do("DELETE", fmt.Sprintf("/cards/%d", id), nil, hx)
	expect(t, w, 200, `hx-swap-oob="delete"`, fmt.Sprintf(`id="count-%d"`, v.Columns[0].ID))
	if strings.Contains(regexp.MustCompile(`id="count-\d+"[^>]*>\s*(\d+)`).FindStringSubmatch(w.Body.String())[1], "1") {
		t.Fatal("count must drop to 0 after delete")
	}
	expect(t, a.do("DELETE", fmt.Sprintf("/cards/%d", id), nil, hx), 404)
}

func (a *testApp) mustCards(boardID int64) []store.Card {
	a.t.Helper()
	v, err := a.store.LoadBoard(context.Background(), boardID, store.Filter{})
	if err != nil {
		a.t.Fatal(err)
	}
	var out []store.Card
	for _, c := range v.Columns {
		out = append(out, c.Cards...)
	}
	return out
}

func TestColumnsAndFilterPreserved(t *testing.T) {
	a := newApp(t)
	v := a.board("Cols")
	board := fmt.Sprintf("/boards/%d", v.Board.ID)
	ctx := context.Background()
	_, _ = a.store.CreateCard(ctx, v.Columns[0].ID, "alpha task")
	_, _ = a.store.CreateCard(ctx, v.Columns[0].ID, "beta task")

	// Mutations re-render with the filter from HX-Current-URL.
	w := a.do("POST", board+"/columns", url.Values{"name": {"Deploy"}}, hx, currentURL(board+"?q=alpha"))
	expect(t, w, 200, "Deploy", "alpha task", `data-testid="filter-summary"`)
	if strings.Contains(w.Body.String(), "beta task") {
		t.Fatal("filter not applied after mutation")
	}

	col := v.Columns[1].ID
	expect(t, a.do("PUT", fmt.Sprintf("/columns/%d", col), url.Values{"name": {"Todo"}, "wip_limit": {"1"}}, hx), 200, "Todo", "/ 1")
	expect(t, a.do("POST", fmt.Sprintf("/columns/%d/move", col), url.Values{"position": {"0"}}, hx), 200)
	w = a.do("DELETE", fmt.Sprintf("/columns/%d", v.Columns[0].ID), nil, hx)
	if w.Code != http.StatusConflict {
		t.Fatalf("delete non-empty = %d", w.Code)
	}
	expect(t, a.do("DELETE", fmt.Sprintf("/columns/%d", v.Columns[0].ID), url.Values{"move_to": {fmt.Sprint(col)}}, hx), 200)
	expect(t, a.do("PUT", "/columns/999", url.Values{"name": {"x"}}, hx), 404)

	expect(t, a.do("GET", board, url.Values{"q": {"beta"}, "priority": {"0"}, "overdue": {"1"}}), 200, "Mostrando")
}

func TestLabels(t *testing.T) {
	a := newApp(t)
	v := a.board("Lab")
	ctx := context.Background()
	c, _ := a.store.CreateCard(ctx, v.Columns[0].ID, "card")

	expect(t, a.do("GET", fmt.Sprintf("/boards/%d/labels", v.Board.ID), nil, hx), 200, `data-testid="labels-dialog"`, "Nenhuma label")
	expect(t, a.do("POST", fmt.Sprintf("/boards/%d/labels", v.Board.ID), url.Values{"name": {""}, "color": {"red"}}, hx), 422, "obrigatório")
	expect(t, a.do("POST", fmt.Sprintf("/boards/%d/labels", v.Board.ID), url.Values{"name": {"bug"}, "color": {"red"}}, hx), 200, "bug", `id="columns"`, `data-board-id="1" hx-swap-oob="true"`, `id="filter-label"`)
	labels, _ := a.store.ListLabels(ctx, v.Board.ID)
	l := labels[0]

	expect(t, a.do("POST", fmt.Sprintf("/cards/%d/labels/%d", c.ID, l.ID), nil, hx), 200, `aria-pressed="true"`, "bug")
	expect(t, a.do("PUT", fmt.Sprintf("/labels/%d", l.ID), url.Values{"name": {"defeito"}, "color": {"orange"}}, hx), 200, "defeito")
	expect(t, a.do("DELETE", fmt.Sprintf("/labels/%d", l.ID), nil, hx), 200, "Nenhuma label")
	expect(t, a.do("DELETE", fmt.Sprintf("/labels/%d", l.ID), nil, hx), 404)
}

func TestExportImportBackup(t *testing.T) {
	a := newApp(t)
	v := a.board("Exp")
	_, _ = a.store.CreateCard(context.Background(), v.Columns[0].ID, "c1")

	w := a.do("GET", "/export", nil)
	expect(t, w, 200, `"version": 1`, "Exp", "c1")
	if !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("export must be a download")
	}
	exported := w.Body.Bytes()

	expect(t, a.do("GET", "/import", nil, hx), 200, `data-testid="import-dialog"`)
	upload := func(content []byte, mode string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", "x.json")
		_, _ = fw.Write(content)
		_ = mw.WriteField("mode", mode)
		_ = mw.Close()
		r := httptest.NewRequest("POST", "/import", &buf)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		a.srv.ServeHTTP(rec, r)
		return rec
	}
	expect(t, upload([]byte("{nope"), "merge"), 422, "JSON inválido")
	expect(t, upload([]byte(`{"version":1,"boards":[{"name":""}]}`), "merge"), 422, "projeto 1")
	w = upload(exported, "merge")
	if w.Code != 204 || w.Header().Get("HX-Redirect") != "/" {
		t.Fatalf("import = %d %s", w.Code, w.Body.String())
	}
	if bs, _ := a.store.ListBoards(context.Background(), false); len(bs) != 2 {
		t.Fatalf("boards after merge = %d", len(bs))
	}
	_ = upload(exported, "replace")
	var snap store.Snapshot
	_ = json.Unmarshal(a.do("GET", "/export", nil).Body.Bytes(), &snap)
	if len(snap.Boards) != 1 {
		t.Fatalf("boards after replace = %d", len(snap.Boards))
	}

	w = a.do("POST", "/backup", nil, hx)
	expect(t, w, 200, "kanban-20260615-120000.db")
	if trig := w.Header().Get("HX-Trigger"); !strings.Contains(trig, "Backup salvo") || strings.ContainsFunc(trig, func(r rune) bool { return r > 127 }) {
		t.Fatal("backup toast missing")
	}
}

func TestBodyLimit(t *testing.T) {
	a := newApp(t)
	big := url.Values{"name": {strings.Repeat("x", maxFormBytes+10)}}
	w := a.do("POST", "/boards", big, hx)
	if w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("oversized body = %d", w.Code)
	}
}

func TestRequestLogging(t *testing.T) {
	a := newApp(t)
	a.do("GET", "/boards/1", nil, hx)
	var entry map[string]any
	for _, line := range strings.Split(strings.TrimSpace(a.logs.String()), "\n") {
		_ = json.Unmarshal([]byte(line), &entry)
		if entry["msg"] == "request" {
			break
		}
	}
	for _, k := range []string{"request_id", "method", "path", "status", "duration_ms"} {
		if _, ok := entry[k]; !ok {
			t.Fatalf("log entry missing %q: %v", k, entry)
		}
	}
	if entry["status"] != float64(404) {
		t.Fatalf("logged status = %v", entry["status"])
	}
}

func TestPanicRecovery(t *testing.T) {
	a := newApp(t)
	h := a.srv.middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	if w.Code != 500 || !strings.Contains(a.logs.String(), `"panic":"boom"`) {
		t.Fatalf("panic = %d logs=%s", w.Code, a.logs.String())
	}
}

func TestHelpers(t *testing.T) {
	if prefix("trello golang") != "TRE" || prefix("a1") != "A1" || prefix("!!") != "KAN" {
		t.Fatal("prefix")
	}
	if initial("ágil") != "Á" || initial("") != "?" {
		t.Fatal("initial")
	}
	kinds := []string{}
	for i := range 5 {
		kinds = append(kinds, statusKind(i, 5))
	}
	if strings.Join(kinds, ",") != "backlog,todo,progress,review,done" {
		t.Fatalf("status kinds = %v", kinds)
	}
}

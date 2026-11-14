package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AlexGalhardo/vukem/web"

	"github.com/AlexGalhardo/vukem-kanban/internal/store"
)

const lastBoardCookie = "last_board"

// MaxImportBytes caps the JSON import upload.
const MaxImportBytes = 20 << 20

// Kanban serves the boards, columns, cards and labels UI.
type Kanban struct {
	store   *store.Store
	log     *slog.Logger
	version string
	now     func() time.Time
	// backupMu serializes backups so concurrent clicks cannot pick the same
	// file name between the existence check and VACUUM INTO.
	backupMu sync.Mutex
}

// NewKanban builds the controller over st.
func NewKanban(st *store.Store, log *slog.Logger, version string, now func() time.Time) *Kanban {
	return &Kanban{store: st, log: log, version: version, now: now}
}

// form reads the request body up front, so a body over the cap answers 413
// instead of a handler seeing a truncated or empty form.
func (k *Kanban) form(h web.HandlerFunc) web.HandlerFunc {
	return func(c *web.Ctx) error {
		if _, err := c.Input(); err != nil {
			return k.fail(c, err)
		}
		return h(c)
	}
}

// Healthz is the liveness probe the desktop shell and the smoke test poll.
func (k *Kanban) Healthz(c *web.Ctx) error {
	return c.Blob(http.StatusOK, "text/plain; charset=utf-8", []byte("ok"))
}

func (k *Kanban) newPage(ctx context.Context) (page, error) {
	p := page{Version: k.version, Today: k.today(), Colors: store.Colors, Priorities: store.Priorities}
	var err error
	if p.Boards, err = k.store.ListBoards(ctx, false); err != nil {
		return p, err
	}
	p.Archived, err = k.store.ListBoards(ctx, true)
	return p, err
}

// boardPage loads the page with board id rendered using filter f.
func (k *Kanban) boardPage(ctx context.Context, id int64, f store.Filter) (page, error) {
	p, err := k.newPage(ctx)
	if err != nil {
		return p, err
	}
	v, err := k.store.LoadBoard(ctx, id, f)
	if err != nil {
		return p, err
	}
	p.View = &v
	p.Prefix = prefix(v.Board.Name)
	return p, nil
}

func (k *Kanban) notFound(c *web.Ctx) error {
	return k.notFoundStatus(c, http.StatusNotFound)
}

func (k *Kanban) notFoundStatus(c *web.Ctx, status int) error {
	p, err := k.newPage(c.Context())
	if err != nil {
		http.Error(c.Writer(), "erro interno", http.StatusInternalServerError)
		return nil
	}
	p.NotFound = true
	return k.render(c, status, p, "layout")
}

// fallback answers unknown routes: a full page for GET, plain text otherwise.
func (k *Kanban) fallback(c *web.Ctx) error {
	r := c.Request()
	if r.Method == http.MethodGet && r.Header.Get("HX-Request") != "true" {
		return k.notFound(c)
	}
	http.Error(c.Writer(), "Não encontrado.", http.StatusNotFound)
	return nil
}

// dashboard is the landing page: it opens the last visited board, falling
// back to the first active one, or shows the empty state.
func (k *Kanban) dashboard(c *web.Ctx) error {
	w, r := c.Writer(), c.Request()
	p, err := k.newPage(r.Context())
	if err != nil {
		return k.fail(c, err)
	}
	if c, err := r.Cookie(lastBoardCookie); err == nil {
		if id, err := strconv.ParseInt(c.Value, 10, 64); err == nil {
			for _, b := range p.Boards {
				if b.ID == id {
					http.Redirect(w, r, fmt.Sprintf("/boards/%d", id), http.StatusSeeOther)
					return nil
				}
			}
		}
	}
	if len(p.Boards) > 0 {
		http.Redirect(w, r, fmt.Sprintf("/boards/%d", p.Boards[0].ID), http.StatusSeeOther)
		return nil
	}
	return k.render(c, http.StatusOK, p, "layout")
}

func (k *Kanban) showBoard(c *web.Ctx) error {
	w, r := c.Writer(), c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	p, err := k.boardPage(r.Context(), id, k.filterFrom(r.URL.Query()))
	if errors.Is(err, store.ErrNotFound) && r.Header.Get("HX-Request") == "true" {
		// The board was deleted (e.g. in another tab): reload its URL, which
		// shows the "not found" page instead of keeping a ghost board.
		return redirect(c, fmt.Sprintf("/boards/%d", id))
	}
	if err != nil {
		return k.fail(c, err)
	}
	rememberBoard(w, id)
	if isHX(r) {
		p.OOB = true
		return k.render(c, http.StatusOK, p, "main", "sidebar", "title-tag")
	}
	return k.render(c, http.StatusOK, p, "layout")
}

// rememberBoard stores the board the landing page should reopen.
func rememberBoard(w http.ResponseWriter, id int64) {
	//nolint:gosec // plain HTTP on 127.0.0.1: Secure would make the browser drop the cookie
	http.SetCookie(w, &http.Cookie{
		Name: lastBoardCookie, Value: strconv.FormatInt(id, 10), Path: "/",
		MaxAge: 365 * 24 * 3600, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
}

func boardInput(r *http.Request) store.BoardInput {
	return store.BoardInput{Name: r.FormValue("name"), Description: r.FormValue("description"), Color: r.FormValue("color")}
}

// formError re-renders a form fragment with 422 when err is a validation
// error and reports whether it did.
func (k *Kanban) formError(c *web.Ctx, err error, p page, name string) bool {
	var verr store.ValidationError
	if !errors.As(err, &verr) {
		return false
	}
	p.Errors, p.Form = verr, c.Request().Form
	_ = k.render(c, http.StatusUnprocessableEntity, p, name)
	return true
}

func (k *Kanban) newBoardDialog(c *web.Ctx) error {
	r := c.Request()
	p, err := k.newPage(r.Context())
	if err != nil {
		return k.fail(c, err)
	}
	return k.render(c, http.StatusOK, p, "new-board-dialog")
}

func (k *Kanban) createBoard(c *web.Ctx) error {
	w, r := c.Writer(), c.Request()
	b, err := k.store.CreateBoard(r.Context(), boardInput(r))
	if err != nil {
		p, perr := k.newPage(r.Context())
		if perr == nil && k.formError(c, err, p, "new-board-dialog") {
			return nil
		}
		return k.fail(c, err)
	}
	boardURL := fmt.Sprintf("/boards/%d", b.ID)
	rememberBoard(w, b.ID)
	if !isHX(r) {
		http.Redirect(w, r, boardURL, http.StatusSeeOther) //nolint:gosec // path built from an integer id, not user input
		return nil
	}
	p, err := k.boardPage(r.Context(), b.ID, store.Filter{Today: k.today()})
	if err != nil {
		return k.fail(c, err)
	}
	p.OOB = true
	w.Header().Set("HX-Push-Url", boardURL)
	w.Header().Set("HX-Retarget", "#main")
	w.Header().Set("HX-Reswap", "innerHTML")
	toast(w, "success", "Projeto criado.")
	return k.render(c, http.StatusCreated, p, "main", "sidebar", "modal-clear", "title-tag")
}

func (k *Kanban) boardSettings(c *web.Ctx) error {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	p, err := k.boardPage(r.Context(), id, store.Filter{Today: k.today()})
	if err != nil {
		return k.fail(c, err)
	}
	return k.render(c, http.StatusOK, p, "board-settings")
}

func (k *Kanban) updateBoard(c *web.Ctx) error {
	w, r := c.Writer(), c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	if _, err := k.store.UpdateBoard(r.Context(), id, boardInput(r)); err != nil {
		p, perr := k.boardPage(r.Context(), id, store.Filter{Today: k.today()})
		if perr == nil && k.formError(c, err, p, "board-settings") {
			return nil
		}
		return k.fail(c, err)
	}
	p, err := k.boardPage(r.Context(), id, k.currentFilter(r))
	if err != nil {
		return k.fail(c, err)
	}
	p.OOB = true
	w.Header().Set("HX-Retarget", "#main")
	w.Header().Set("HX-Reswap", "innerHTML")
	toast(w, "success", "Projeto atualizado.")
	return k.render(c, http.StatusOK, p, "main", "sidebar", "modal-clear", "title-tag")
}

func (k *Kanban) archiveBoard(archived bool) web.HandlerFunc {
	return func(c *web.Ctx) error {
		r := c.Request()
		id, err := pathID(r, "id")
		if err != nil {
			return k.fail(c, err)
		}
		if err := k.store.SetBoardArchived(r.Context(), id, archived); err != nil {
			return k.fail(c, err)
		}
		dest := fmt.Sprintf("/boards/%d", id)
		if archived {
			dest = "/"
		}
		return redirect(c, dest)
	}
}

// redirect sends the browser to a local path, with a full page load for htmx
// requests. Callers only pass paths built by this package.
func redirect(c *web.Ctx, path string) error {
	w, r := c.Writer(), c.Request()
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", path)
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	http.Redirect(w, r, path, http.StatusSeeOther) //nolint:gosec // see doc comment
	return nil
}

func (k *Kanban) deleteBoard(c *web.Ctx) error {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	b, err := k.store.GetBoard(r.Context(), id)
	if err != nil {
		return k.fail(c, err)
	}
	if strings.TrimSpace(r.FormValue("confirm_name")) != b.Name {
		p, err := k.boardPage(r.Context(), id, store.Filter{Today: k.today()})
		if err != nil {
			return k.fail(c, err)
		}
		k.formError(c, store.ValidationError{"confirm_name": "digite o nome exato do projeto"}, p, "board-settings")
		return nil
	}
	if err := k.store.DeleteBoard(r.Context(), id); err != nil {
		return k.fail(c, err)
	}
	return redirect(c, "/")
}

// renderColumns re-renders the whole board body after a structural change,
// keeping the filter the user had applied.
func (k *Kanban) renderColumns(c *web.Ctx, boardID int64, extra ...string) error {
	r := c.Request()
	p, err := k.boardPage(r.Context(), boardID, k.currentFilter(r))
	if err != nil {
		return k.fail(c, err)
	}
	return k.render(c, http.StatusOK, p, append([]string{"columns"}, extra...)...)
}

func (k *Kanban) createColumn(c *web.Ctx) error {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	wip, err := wipLimit(r)
	if err != nil {
		return k.fail(c, err)
	}
	if _, err := k.store.CreateColumn(r.Context(), id, store.ColumnInput{Name: r.FormValue("name"), WIPLimit: wip}); err != nil {
		return k.fail(c, err)
	}
	return k.renderColumns(c, id)
}

// wipLimit parses the optional WIP limit; text like "abc" or "5.5" is an
// error instead of silently clearing the limit.
func wipLimit(r *http.Request) (int, error) {
	v := strings.TrimSpace(r.FormValue("wip_limit"))
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, store.ValidationError{"wip_limit": "use um número inteiro"}
	}
	return n, nil
}

func (k *Kanban) columnFromPath(c *web.Ctx) (store.Column, bool) {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		_ = k.fail(c, err)
		return store.Column{}, false
	}
	col, err := k.store.GetColumn(r.Context(), id)
	if err != nil {
		_ = k.fail(c, err)
		return store.Column{}, false
	}
	return col, true
}

func (k *Kanban) updateColumn(c *web.Ctx) error {
	r := c.Request()
	col, ok := k.columnFromPath(c)
	if !ok {
		return nil
	}
	wip, err := wipLimit(r)
	if err != nil {
		return k.fail(c, err)
	}
	if _, err := k.store.UpdateColumn(r.Context(), col.ID, store.ColumnInput{Name: r.FormValue("name"), WIPLimit: wip}); err != nil {
		return k.fail(c, err)
	}
	return k.renderColumns(c, col.BoardID)
}

func (k *Kanban) deleteColumn(c *web.Ctx) error {
	r := c.Request()
	col, ok := k.columnFromPath(c)
	if !ok {
		return nil
	}
	moveTo, _ := strconv.ParseInt(r.FormValue("move_to"), 10, 64)
	if err := k.store.DeleteColumn(r.Context(), col.ID, moveTo); err != nil {
		return k.fail(c, err)
	}
	return k.renderColumns(c, col.BoardID)
}

func (k *Kanban) moveColumn(c *web.Ctx) error {
	r := c.Request()
	col, ok := k.columnFromPath(c)
	if !ok {
		return nil
	}
	if err := k.store.MoveColumn(r.Context(), col.ID, formInt(r, "position")); err != nil {
		return k.fail(c, err)
	}
	return k.renderColumns(c, col.BoardID)
}

// cardPage loads a card plus its board view (for labels, columns and the
// column header counters).
func (k *Kanban) cardPage(ctx context.Context, cardID int64) (page, error) {
	c, err := k.store.GetCard(ctx, cardID)
	if err != nil {
		return page{}, err
	}
	p, err := k.boardPage(ctx, c.BoardID, store.Filter{Today: k.today()})
	if err != nil {
		return page{}, err
	}
	p.Card = &c
	for i := range p.View.Columns {
		if p.View.Columns[i].ID == c.ColumnID {
			p.Column = &p.View.Columns[i]
		}
	}
	return p, nil
}

func (k *Kanban) createCard(c *web.Ctx) error {
	w, r := c.Writer(), c.Request()
	colID, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	card, err := k.store.CreateCard(r.Context(), colID, r.FormValue("title"))
	if err != nil {
		return k.fail(c, err)
	}
	if f := k.currentFilter(r); f.Active() {
		// With a filter on, re-render the board so counts and the summary
		// stay right; say so when the new card is hidden by the filter.
		p, err := k.boardPage(r.Context(), card.BoardID, f)
		if err != nil {
			return k.fail(c, err)
		}
		if !p.View.Has(card.ID) {
			toast(w, "info", "Card criado, mas oculto pelo filtro atual.")
		}
		w.Header().Set("HX-Retarget", "#columns")
		w.Header().Set("HX-Reswap", "outerHTML")
		return k.render(c, http.StatusCreated, p, "columns")
	}
	p, err := k.cardPage(r.Context(), card.ID)
	if err != nil {
		return k.fail(c, err)
	}
	return k.render(c, http.StatusCreated, p, "card", "column-header-oob")
}

func (k *Kanban) cardFromPath(c *web.Ctx) (page, bool) {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		_ = k.fail(c, err)
		return page{}, false
	}
	p, err := k.cardPage(r.Context(), id)
	if err != nil {
		_ = k.fail(c, err)
		return page{}, false
	}
	return p, true
}

func (k *Kanban) cardDetail(c *web.Ctx) error {
	if p, ok := k.cardFromPath(c); ok {
		return k.render(c, http.StatusOK, p, "card-dialog")
	}
	return nil
}

func (k *Kanban) cardFragment(c *web.Ctx) error {
	if p, ok := k.cardFromPath(c); ok {
		return k.render(c, http.StatusOK, p, "card")
	}
	return nil
}

func (k *Kanban) cardTitleForm(c *web.Ctx) error {
	if p, ok := k.cardFromPath(c); ok {
		return k.render(c, http.StatusOK, p, "card-title-form")
	}
	return nil
}

func (k *Kanban) renameCard(c *web.Ctx) error {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	if _, err := k.store.RenameCard(r.Context(), id, r.FormValue("title")); err != nil {
		return k.fail(c, err)
	}
	p, ok := k.cardFromPath(c)
	if !ok {
		return nil
	}
	return k.renderWithFilteredBoard(c, p, "card")
}

// renderWithFilteredBoard renders names and, when a filter is active, also
// the whole board out of band: an edited card may no longer match it.
func (k *Kanban) renderWithFilteredBoard(c *web.Ctx, p page, names ...string) error {
	r := c.Request()
	if f := k.currentFilter(r); f.Active() {
		bp, err := k.boardPage(r.Context(), p.Card.BoardID, f)
		if err != nil {
			return k.fail(c, err)
		}
		p.View, p.ColumnsOOB = bp.View, true
		names = append(names, "columns")
	}
	return k.render(c, http.StatusOK, p, names...)
}

func (k *Kanban) updateCard(c *web.Ctx) error {
	w, r := c.Writer(), c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	in := store.CardInput{
		Title: r.FormValue("title"), Description: r.FormValue("description"),
		Priority: formInt(r, "priority"), DueDate: r.FormValue("due_date"),
	}
	revision := int64(-1)
	if v := r.FormValue("revision"); v != "" {
		if revision, err = strconv.ParseInt(v, 10, 64); err != nil {
			return k.fail(c, errBadRequest)
		}
	}
	if _, err := k.store.UpdateCardAt(r.Context(), id, in, revision); err != nil {
		p, perr := k.cardPage(r.Context(), id)
		if perr == nil && k.formError(c, err, p, "card-dialog") {
			return nil
		}
		return k.fail(c, err)
	}
	p, ok := k.cardFromPath(c)
	if !ok {
		return nil
	}
	p.OOB = true
	toast(w, "success", "Card salvo.")
	return k.renderWithFilteredBoard(c, p, "card", "modal-clear")
}

func (k *Kanban) deleteCard(c *web.Ctx) error {
	w, r := c.Writer(), c.Request()
	p, ok := k.cardFromPath(c)
	if !ok {
		return nil
	}
	if err := k.store.DeleteCard(r.Context(), p.Card.ID); err != nil {
		return k.fail(c, err)
	}
	// Recount the column header without the deleted card.
	if col := p.Column; col != nil {
		for i, card := range col.Cards {
			if card.ID == p.Card.ID {
				col.Cards = append(col.Cards[:i:i], col.Cards[i+1:]...)
				break
			}
		}
	}
	toast(w, "success", "Card excluído.")
	if f := k.currentFilter(r); f.Active() {
		bp, err := k.boardPage(r.Context(), p.Card.BoardID, f)
		if err != nil {
			return k.fail(c, err)
		}
		bp.ColumnsOOB = true
		return k.render(c, http.StatusOK, bp, "columns", "modal-clear")
	}
	return k.render(c, http.StatusOK, p, "card-delete-oob", "column-header-oob", "modal-clear")
}

func (k *Kanban) moveCard(c *web.Ctx) error {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	colID, err := strconv.ParseInt(r.FormValue("column_id"), 10, 64)
	if err != nil {
		return k.fail(c, errBadRequest)
	}
	before, _ := strconv.ParseInt(r.FormValue("before_id"), 10, 64)
	from, err := k.store.GetCard(r.Context(), id)
	if err != nil {
		return k.fail(c, err)
	}
	if err := k.store.MoveCardBefore(r.Context(), id, colID, before); err != nil {
		return k.fail(c, err)
	}
	// Only the source and target columns changed; re-rendering just those
	// keeps a drag on a 2000-card board from shipping the whole board back.
	p, err := k.boardPage(r.Context(), from.BoardID, k.currentFilter(r))
	if err != nil {
		return k.fail(c, err)
	}
	return k.renderColumnsOOB(c, p, from.ColumnID, colID)
}

// renderColumnsOOB renders the given columns as out-of-band replacements.
func (k *Kanban) renderColumnsOOB(c *web.Ctx, p page, ids ...int64) error {
	var parts []fragment
	n := len(p.View.Columns)
	seen := map[int64]bool{}
	for i, col := range p.View.Columns {
		for _, id := range ids {
			if col.ID == id && !seen[id] {
				seen[id] = true
				parts = append(parts, fragment{"column", map[string]any{
					"Column": col, "Index": i, "Count": n, "Columns": p.View.Columns,
					"Today": p.Today, "Prefix": p.Prefix, "OOB": true,
				}})
			}
		}
	}
	return k.renderParts(c, http.StatusOK, parts...)
}

func (k *Kanban) toggleCardLabel(c *web.Ctx) error {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	labelID, err := pathID(r, "label")
	if err != nil {
		return k.fail(c, err)
	}
	if err := k.store.ToggleCardLabel(r.Context(), id, labelID); err != nil {
		return k.fail(c, err)
	}
	p, ok := k.cardFromPath(c)
	if !ok {
		return nil
	}
	p.OOB = true
	return k.renderWithFilteredBoard(c, p, "card-labels", "card")
}

func (k *Kanban) labelsDialog(c *web.Ctx) error {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	p, err := k.boardPage(r.Context(), id, k.currentFilter(r))
	if err != nil {
		return k.fail(c, err)
	}
	return k.render(c, http.StatusOK, p, "labels-dialog")
}

// renderLabels shows the label manager and refreshes the board out of band,
// since label changes alter the chips on the cards.
func (k *Kanban) renderLabels(c *web.Ctx, boardID int64, status int, verr store.ValidationError) error {
	w, r := c.Writer(), c.Request()
	f := k.currentFilter(r)
	p, err := k.boardPage(r.Context(), boardID, f)
	if err != nil {
		return k.fail(c, err)
	}
	if f.LabelID != 0 && !hasLabel(p.View.Labels, f.LabelID) {
		f.LabelID = 0
		if p, err = k.boardPage(r.Context(), boardID, f); err != nil {
			return k.fail(c, err)
		}
		if u, err := url.Parse(r.Header.Get("HX-Current-URL")); err == nil {
			q := u.Query()
			q.Del("label")
			u.RawQuery = q.Encode()
			w.Header().Set("HX-Replace-Url", u.RequestURI())
		}
	}
	p.Errors, p.Form, p.ColumnsOOB = verr, r.Form, true
	return k.render(c, status, p, "labels-dialog", "columns", "filter-label")
}

func hasLabel(labels []store.Label, id int64) bool {
	for _, l := range labels {
		if l.ID == id {
			return true
		}
	}
	return false
}

func labelInput(r *http.Request) store.LabelInput {
	return store.LabelInput{Name: r.FormValue("name"), Color: r.FormValue("color")}
}

func (k *Kanban) createLabel(c *web.Ctx) error {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	if _, err := k.store.CreateLabel(r.Context(), id, labelInput(r)); err != nil {
		var verr store.ValidationError
		if errors.As(err, &verr) {
			return k.renderLabels(c, id, http.StatusUnprocessableEntity, verr)
		}
		return k.fail(c, err)
	}
	r.Form = nil
	return k.renderLabels(c, id, http.StatusOK, nil)
}

func (k *Kanban) updateLabel(c *web.Ctx) error {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	l, err := k.store.UpdateLabel(r.Context(), id, labelInput(r))
	if err != nil {
		return k.fail(c, err)
	}
	return k.renderLabels(c, l.BoardID, http.StatusOK, nil)
}

func (k *Kanban) deleteLabel(c *web.Ctx) error {
	r := c.Request()
	id, err := pathID(r, "id")
	if err != nil {
		return k.fail(c, err)
	}
	l, err := k.store.GetLabel(r.Context(), id)
	if err != nil {
		return k.fail(c, err)
	}
	if err := k.store.DeleteLabel(r.Context(), id); err != nil {
		return k.fail(c, err)
	}
	return k.renderLabels(c, l.BoardID, http.StatusOK, nil)
}

// markdownPreview renders the description being edited (no persistence).
func (k *Kanban) markdownPreview(c *web.Ctx) error {
	r := c.Request()
	return k.render(c, http.StatusOK, map[string]string{"Src": r.FormValue("description")}, "markdown-preview")
}

func (k *Kanban) export(c *web.Ctx) error {
	w, r := c.Writer(), c.Request()
	snap, err := k.store.Export(r.Context())
	if err != nil {
		return k.fail(c, err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="kanban-%s.json"`, k.now().Format("20060102-150405")))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(snap)
	return nil
}

func (k *Kanban) importDialog(c *web.Ctx) error {
	r := c.Request()
	p, err := k.newPage(r.Context())
	if err != nil {
		return k.fail(c, err)
	}
	return k.render(c, http.StatusOK, p, "import-dialog")
}

func (k *Kanban) importData(c *web.Ctx) error {
	r := c.Request()
	importErr := func(msg string) {
		p, err := k.newPage(r.Context())
		if err != nil {
			_ = k.fail(c, err)
			return
		}
		k.formError(c, store.ValidationError{"file": msg}, p, "import-dialog")
	}
	// The body is already capped by http.MaxBytesReader in the middleware.
	if err := r.ParseMultipartForm(MaxImportBytes); err != nil { //nolint:gosec // see above
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			importErr("arquivo maior que 20 MB")
			return nil
		}
		importErr("envie um arquivo JSON")
		return nil
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		importErr("selecione um arquivo")
		return nil
	}
	defer func() { _ = f.Close() }()
	var snap store.Snapshot
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&snap); err != nil {
		k.log.Info("import rejected", "err", err)
		importErr("JSON inválido. Use um arquivo gerado por “Exportar dados”.")
		return nil
	}
	if dec.More() {
		importErr("JSON inválido: há conteúdo extra depois dos dados.")
		return nil
	}
	replace := r.FormValue("mode") == "replace"
	if replace {
		if _, err := k.writeBackup(r); err != nil {
			return k.fail(c, err)
		}
	}
	if err := k.store.Import(r.Context(), snap, replace); err != nil {
		var verr store.ValidationError
		if errors.As(err, &verr) {
			importErr(verr["file"])
			return nil
		}
		return k.fail(c, err)
	}
	return redirect(c, "/")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// backup writes a consistent copy of the database to a backups/ folder next
// to it and returns the file path.
func (k *Kanban) backup(c *web.Ctx) error {
	w, r := c.Writer(), c.Request()
	dest, err := k.writeBackup(r)
	if err != nil {
		return k.fail(c, err)
	}
	toast(w, "success", "Backup salvo em "+dest)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(dest)) //nolint:gosec // text/plain + nosniff: never rendered as HTML
	return nil
}

func (k *Kanban) writeBackup(r *http.Request) (string, error) {
	k.backupMu.Lock()
	defer k.backupMu.Unlock()
	dir := filepath.Join(filepath.Dir(k.store.Path()), "backups")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	// Millisecond resolution plus a counter: two clicks in the same second
	// used to collide ("output file already exists").
	stamp := k.now().Format("20060102-150405.000")
	dest := filepath.Join(dir, "kanban-"+strings.Replace(stamp, ".", "-", 1)+".db")
	for i := 2; fileExists(dest); i++ {
		dest = filepath.Join(dir, fmt.Sprintf("kanban-%s-%d.db", strings.Replace(stamp, ".", "-", 1), i))
	}
	if err := k.store.Backup(r.Context(), dest); err != nil {
		return "", err
	}
	return dest, nil
}

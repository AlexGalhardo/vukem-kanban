package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AlexGalhardo/trello-golang-htmx/internal/store"
)

const lastBoardCookie = "last_board"

func (s *Server) newPage(ctx context.Context) (page, error) {
	p := page{Version: s.version, Today: s.today(), Colors: store.Colors, Priorities: store.Priorities}
	var err error
	if p.Boards, err = s.store.ListBoards(ctx, false); err != nil {
		return p, err
	}
	p.Archived, err = s.store.ListBoards(ctx, true)
	return p, err
}

// boardPage loads the page with board id rendered using filter f.
func (s *Server) boardPage(ctx context.Context, id int64, f store.Filter) (page, error) {
	p, err := s.newPage(ctx)
	if err != nil {
		return p, err
	}
	v, err := s.store.LoadBoard(ctx, id, f)
	if err != nil {
		return p, err
	}
	p.View = &v
	p.Prefix = prefix(v.Board.Name)
	return p, nil
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	p, err := s.newPage(r.Context())
	if err != nil {
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}
	p.NotFound = true
	s.render(w, http.StatusNotFound, p, "layout")
}

// dashboard is the landing page: it opens the last visited board, falling
// back to the first active one, or shows the empty state.
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	p, err := s.newPage(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if c, err := r.Cookie(lastBoardCookie); err == nil {
		if id, err := strconv.ParseInt(c.Value, 10, 64); err == nil {
			for _, b := range p.Boards {
				if b.ID == id {
					http.Redirect(w, r, fmt.Sprintf("/boards/%d", id), http.StatusSeeOther)
					return
				}
			}
		}
	}
	if len(p.Boards) > 0 {
		http.Redirect(w, r, fmt.Sprintf("/boards/%d", p.Boards[0].ID), http.StatusSeeOther)
		return
	}
	s.render(w, http.StatusOK, p, "layout")
}

func (s *Server) showBoard(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	p, err := s.boardPage(r.Context(), id, s.filterFrom(r.URL.Query()))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	rememberBoard(w, id)
	if isHX(r) {
		p.OOB = true
		s.render(w, http.StatusOK, p, "main", "sidebar")
		return
	}
	s.render(w, http.StatusOK, p, "layout")
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
func (s *Server) formError(w http.ResponseWriter, r *http.Request, err error, p page, name string) bool {
	var verr store.ValidationError
	if !errors.As(err, &verr) {
		return false
	}
	p.Errors, p.Form = verr, r.Form
	s.render(w, http.StatusUnprocessableEntity, p, name)
	return true
}

func (s *Server) newBoardDialog(w http.ResponseWriter, r *http.Request) {
	p, err := s.newPage(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, http.StatusOK, p, "new-board-dialog")
}

func (s *Server) createBoard(w http.ResponseWriter, r *http.Request) {
	b, err := s.store.CreateBoard(r.Context(), boardInput(r))
	if err != nil {
		p, perr := s.newPage(r.Context())
		if perr == nil && s.formError(w, r, err, p, "new-board-dialog") {
			return
		}
		s.fail(w, r, err)
		return
	}
	boardURL := fmt.Sprintf("/boards/%d", b.ID)
	rememberBoard(w, b.ID)
	if !isHX(r) {
		http.Redirect(w, r, boardURL, http.StatusSeeOther) //nolint:gosec // path built from an integer id, not user input
		return
	}
	p, err := s.boardPage(r.Context(), b.ID, store.Filter{Today: s.today()})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	p.OOB = true
	w.Header().Set("HX-Push-Url", boardURL)
	w.Header().Set("HX-Retarget", "#main")
	w.Header().Set("HX-Reswap", "innerHTML")
	toast(w, "success", "Projeto criado.")
	s.render(w, http.StatusCreated, p, "main", "sidebar", "modal-clear")
}

func (s *Server) boardSettings(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	p, err := s.boardPage(r.Context(), id, store.Filter{Today: s.today()})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, http.StatusOK, p, "board-settings")
}

func (s *Server) updateBoard(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if _, err := s.store.UpdateBoard(r.Context(), id, boardInput(r)); err != nil {
		p, perr := s.boardPage(r.Context(), id, store.Filter{Today: s.today()})
		if perr == nil && s.formError(w, r, err, p, "board-settings") {
			return
		}
		s.fail(w, r, err)
		return
	}
	p, err := s.boardPage(r.Context(), id, s.currentFilter(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	p.OOB = true
	w.Header().Set("HX-Retarget", "#main")
	w.Header().Set("HX-Reswap", "innerHTML")
	toast(w, "success", "Projeto atualizado.")
	s.render(w, http.StatusOK, p, "main", "sidebar", "modal-clear")
}

func (s *Server) archiveBoard(archived bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r, "id")
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if err := s.store.SetBoardArchived(r.Context(), id, archived); err != nil {
			s.fail(w, r, err)
			return
		}
		dest := fmt.Sprintf("/boards/%d", id)
		if archived {
			dest = "/"
		}
		redirect(w, r, dest)
	}
}

// redirect sends the browser to a local path, with a full page load for htmx
// requests. Callers only pass paths built by this package.
func redirect(w http.ResponseWriter, r *http.Request, path string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", path)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, path, http.StatusSeeOther) //nolint:gosec // see doc comment
}

func (s *Server) deleteBoard(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	b, err := s.store.GetBoard(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if strings.TrimSpace(r.FormValue("confirm_name")) != b.Name {
		p, err := s.boardPage(r.Context(), id, store.Filter{Today: s.today()})
		if err != nil {
			s.fail(w, r, err)
			return
		}
		s.formError(w, r, store.ValidationError{"confirm_name": "digite o nome exato do projeto"}, p, "board-settings")
		return
	}
	if err := s.store.DeleteBoard(r.Context(), id); err != nil {
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/")
}

// renderColumns re-renders the whole board body after a structural change,
// keeping the filter the user had applied.
func (s *Server) renderColumns(w http.ResponseWriter, r *http.Request, boardID int64, extra ...string) {
	p, err := s.boardPage(r.Context(), boardID, s.currentFilter(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, http.StatusOK, p, append([]string{"columns"}, extra...)...)
}

func (s *Server) createColumn(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if _, err := s.store.CreateColumn(r.Context(), id, store.ColumnInput{Name: r.FormValue("name"), WIPLimit: formInt(r, "wip_limit")}); err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderColumns(w, r, id)
}

func (s *Server) columnFromPath(w http.ResponseWriter, r *http.Request) (store.Column, bool) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return store.Column{}, false
	}
	c, err := s.store.GetColumn(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return store.Column{}, false
	}
	return c, true
}

func (s *Server) updateColumn(w http.ResponseWriter, r *http.Request) {
	c, ok := s.columnFromPath(w, r)
	if !ok {
		return
	}
	if _, err := s.store.UpdateColumn(r.Context(), c.ID, store.ColumnInput{Name: r.FormValue("name"), WIPLimit: formInt(r, "wip_limit")}); err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderColumns(w, r, c.BoardID)
}

func (s *Server) deleteColumn(w http.ResponseWriter, r *http.Request) {
	c, ok := s.columnFromPath(w, r)
	if !ok {
		return
	}
	moveTo, _ := strconv.ParseInt(r.FormValue("move_to"), 10, 64)
	if err := s.store.DeleteColumn(r.Context(), c.ID, moveTo); err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderColumns(w, r, c.BoardID)
}

func (s *Server) moveColumn(w http.ResponseWriter, r *http.Request) {
	c, ok := s.columnFromPath(w, r)
	if !ok {
		return
	}
	if err := s.store.MoveColumn(r.Context(), c.ID, formInt(r, "position")); err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderColumns(w, r, c.BoardID)
}

// cardPage loads a card plus its board view (for labels, columns and the
// column header counters).
func (s *Server) cardPage(ctx context.Context, cardID int64) (page, error) {
	c, err := s.store.GetCard(ctx, cardID)
	if err != nil {
		return page{}, err
	}
	p, err := s.boardPage(ctx, c.BoardID, store.Filter{Today: s.today()})
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

func (s *Server) createCard(w http.ResponseWriter, r *http.Request) {
	colID, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	c, err := s.store.CreateCard(r.Context(), colID, r.FormValue("title"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	p, err := s.cardPage(r.Context(), c.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, http.StatusCreated, p, "card", "column-header-oob")
}

func (s *Server) cardFromPath(w http.ResponseWriter, r *http.Request) (page, bool) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return page{}, false
	}
	p, err := s.cardPage(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return page{}, false
	}
	return p, true
}

func (s *Server) cardDetail(w http.ResponseWriter, r *http.Request) {
	if p, ok := s.cardFromPath(w, r); ok {
		s.render(w, http.StatusOK, p, "card-dialog")
	}
}

func (s *Server) cardFragment(w http.ResponseWriter, r *http.Request) {
	if p, ok := s.cardFromPath(w, r); ok {
		s.render(w, http.StatusOK, p, "card")
	}
}

func (s *Server) cardTitleForm(w http.ResponseWriter, r *http.Request) {
	if p, ok := s.cardFromPath(w, r); ok {
		s.render(w, http.StatusOK, p, "card-title-form")
	}
}

func (s *Server) renameCard(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if _, err := s.store.RenameCard(r.Context(), id, r.FormValue("title")); err != nil {
		s.fail(w, r, err)
		return
	}
	if p, ok := s.cardFromPath(w, r); ok {
		s.render(w, http.StatusOK, p, "card")
	}
}

func (s *Server) updateCard(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	in := store.CardInput{
		Title: r.FormValue("title"), Description: r.FormValue("description"),
		Priority: formInt(r, "priority"), DueDate: r.FormValue("due_date"),
	}
	if _, err := s.store.UpdateCard(r.Context(), id, in); err != nil {
		p, perr := s.cardPage(r.Context(), id)
		if perr == nil && s.formError(w, r, err, p, "card-dialog") {
			return
		}
		s.fail(w, r, err)
		return
	}
	p, ok := s.cardFromPath(w, r)
	if !ok {
		return
	}
	p.OOB = true
	toast(w, "success", "Card salvo.")
	s.render(w, http.StatusOK, p, "card", "modal-clear")
}

func (s *Server) deleteCard(w http.ResponseWriter, r *http.Request) {
	p, ok := s.cardFromPath(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteCard(r.Context(), p.Card.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	// Recount the column header without the deleted card.
	if c := p.Column; c != nil {
		for i, card := range c.Cards {
			if card.ID == p.Card.ID {
				c.Cards = append(c.Cards[:i:i], c.Cards[i+1:]...)
				break
			}
		}
	}
	toast(w, "success", "Card excluído.")
	s.render(w, http.StatusOK, p, "card-delete-oob", "column-header-oob", "modal-clear")
}

func (s *Server) moveCard(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	colID, err := strconv.ParseInt(r.FormValue("column_id"), 10, 64)
	if err != nil {
		s.fail(w, r, errBadRequest)
		return
	}
	pos := formInt(r, "position")
	if r.FormValue("position") == "end" {
		pos = 1 << 30 // clamped to the end by the store
	}
	if err := s.store.MoveCard(r.Context(), id, colID, pos); err != nil {
		s.fail(w, r, err)
		return
	}
	col, err := s.store.GetColumn(r.Context(), colID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderColumns(w, r, col.BoardID)
}

func (s *Server) toggleCardLabel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	labelID, err := pathID(r, "label")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.store.ToggleCardLabel(r.Context(), id, labelID); err != nil {
		s.fail(w, r, err)
		return
	}
	p, ok := s.cardFromPath(w, r)
	if !ok {
		return
	}
	p.OOB = true
	s.render(w, http.StatusOK, p, "card-labels", "card")
}

func (s *Server) labelsDialog(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderLabels(w, r, id, http.StatusOK, nil)
}

// renderLabels shows the label manager and refreshes the board out of band,
// since label changes alter the chips on the cards.
func (s *Server) renderLabels(w http.ResponseWriter, r *http.Request, boardID int64, status int, verr store.ValidationError) {
	p, err := s.boardPage(r.Context(), boardID, s.currentFilter(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	p.Errors, p.Form, p.ColumnsOOB = verr, r.Form, true
	s.render(w, status, p, "labels-dialog", "columns", "filter-label")
}

func labelInput(r *http.Request) store.LabelInput {
	return store.LabelInput{Name: r.FormValue("name"), Color: r.FormValue("color")}
}

func (s *Server) createLabel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if _, err := s.store.CreateLabel(r.Context(), id, labelInput(r)); err != nil {
		var verr store.ValidationError
		if errors.As(err, &verr) {
			s.renderLabels(w, r, id, http.StatusUnprocessableEntity, verr)
			return
		}
		s.fail(w, r, err)
		return
	}
	r.Form = nil
	s.renderLabels(w, r, id, http.StatusOK, nil)
}

func (s *Server) updateLabel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	l, err := s.store.UpdateLabel(r.Context(), id, labelInput(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderLabels(w, r, l.BoardID, http.StatusOK, nil)
}

func (s *Server) deleteLabel(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	l, err := s.store.GetLabel(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.store.DeleteLabel(r.Context(), id); err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderLabels(w, r, l.BoardID, http.StatusOK, nil)
}

func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	snap, err := s.store.Export(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="kanban-%s.json"`, s.now().Format("20060102-150405")))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(snap)
}

func (s *Server) importDialog(w http.ResponseWriter, r *http.Request) {
	p, err := s.newPage(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, http.StatusOK, p, "import-dialog")
}

func (s *Server) importData(w http.ResponseWriter, r *http.Request) {
	importErr := func(msg string) {
		p, err := s.newPage(r.Context())
		if err != nil {
			s.fail(w, r, err)
			return
		}
		s.formError(w, r, store.ValidationError{"file": msg}, p, "import-dialog")
	}
	// The body is already capped by http.MaxBytesReader in the middleware.
	if err := r.ParseMultipartForm(maxImportBytes); err != nil { //nolint:gosec // see above
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			importErr("arquivo maior que 20 MB")
			return
		}
		importErr("envie um arquivo JSON")
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		importErr("selecione um arquivo")
		return
	}
	defer func() { _ = f.Close() }()
	var snap store.Snapshot
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&snap); err != nil {
		importErr("JSON inválido: " + err.Error())
		return
	}
	if err := s.store.Import(r.Context(), snap, r.FormValue("mode") == "replace"); err != nil {
		var verr store.ValidationError
		if errors.As(err, &verr) {
			importErr(verr["file"])
			return
		}
		s.fail(w, r, err)
		return
	}
	redirect(w, r, "/")
}

// backup writes a consistent copy of the database to a backups/ folder next
// to it and returns the file path.
func (s *Server) backup(w http.ResponseWriter, r *http.Request) {
	dir := filepath.Join(filepath.Dir(s.store.Path()), "backups")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		s.fail(w, r, err)
		return
	}
	dest := filepath.Join(dir, "kanban-"+s.now().Format("20060102-150405")+".db")
	if err := s.store.Backup(r.Context(), dest); err != nil {
		s.fail(w, r, err)
		return
	}
	toast(w, "success", "Backup salvo em "+dest)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(dest))
}

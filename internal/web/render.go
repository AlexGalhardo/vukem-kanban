package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/AlexGalhardo/trello-golang-htmx/internal/store"
)

// colorClasses maps the closed store.Colors palette to literal Tailwind
// classes. They must stay literal so the Tailwind scanner finds them.
var colorClasses = map[string][2]string{
	// dot/background, chip (bg + text)
	"slate":  {"bg-slate-500", "bg-slate-500/15 text-slate-700 dark:text-slate-300"},
	"red":    {"bg-red-500", "bg-red-500/15 text-red-700 dark:text-red-300"},
	"orange": {"bg-orange-500", "bg-orange-500/15 text-orange-700 dark:text-orange-300"},
	"amber":  {"bg-amber-500", "bg-amber-500/15 text-amber-700 dark:text-amber-300"},
	"green":  {"bg-green-500", "bg-green-500/15 text-green-700 dark:text-green-300"},
	"teal":   {"bg-teal-500", "bg-teal-500/15 text-teal-700 dark:text-teal-300"},
	"sky":    {"bg-sky-500", "bg-sky-500/15 text-sky-700 dark:text-sky-300"},
	"indigo": {"bg-indigo-500", "bg-indigo-500/15 text-indigo-700 dark:text-indigo-300"},
	"violet": {"bg-violet-500", "bg-violet-500/15 text-violet-700 dark:text-violet-300"},
	"pink":   {"bg-pink-500", "bg-pink-500/15 text-pink-700 dark:text-pink-300"},
}

var priorityClasses = []string{
	"text-zinc-400",
	"text-red-600 dark:text-red-400",
	"text-orange-600 dark:text-orange-400",
	"text-amber-600 dark:text-amber-400",
	"text-sky-600 dark:text-sky-400",
}

var funcs = template.FuncMap{
	"dot":  func(c string) string { return colorClasses[c][0] },
	"chip": func(c string) string { return colorClasses[c][1] },
	"priorityName": func(p int) string {
		if p < 0 || p >= len(store.Priorities) {
			return ""
		}
		return store.Priorities[p]
	},
	"priorityClass": func(p int) string {
		if p < 0 || p >= len(priorityClasses) {
			return ""
		}
		return priorityClasses[p]
	},
	// dict builds a map so templates can pass several values to a sub-template.
	"dict": func(kv ...any) (map[string]any, error) {
		if len(kv)%2 != 0 {
			return nil, errors.New("dict: odd number of arguments")
		}
		m := make(map[string]any, len(kv)/2)
		for i := 0; i < len(kv); i += 2 {
			k, ok := kv[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict: key %v is not a string", kv[i])
			}
			m[k] = kv[i+1]
		}
		return m, nil
	},
	"add":        func(a, b int) int { return a + b },
	"initial":    initial,
	"statusKind": statusKind,
	// pie returns the stroke-dasharray length (of a 2πr≈15.7 circle) that
	// fills the in-progress status icon proportionally to the column index.
	"pie": func(i, n int) string {
		if n < 2 {
			return "0"
		}
		return strconv.FormatFloat(15.7*float64(i)/float64(n-1), 'f', 2, 64)
	},
	// val returns the submitted form value when re-rendering after an error,
	// otherwise the stored fallback.
	"val": func(form url.Values, name string, fallback any) string {
		if form != nil {
			if _, ok := form[name]; ok {
				return form.Get(name)
			}
		}
		return fmt.Sprint(fallback)
	},
	"fmtDate": func(d string) string {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			return d
		}
		return t.Format("02/01/2006")
	},
	"last": func(i, n int) bool {
		return i == n-1
	},
}

// page is the single data type for the layout and every fragment. Fragments
// read only the fields they need; keeping one type avoids a struct per template.
type page struct {
	Version    string
	Boards     []store.Board
	Archived   []store.Board
	View       *store.BoardView
	Card       *store.Card
	Column     *store.Column
	Today      string
	Colors     []string
	Priorities []string
	NotFound   bool
	Prefix     string                // short board key shown before card ids, e.g. "KAN-12"
	OOB        bool                  // render the fragment as an hx-swap-oob update
	Errors     store.ValidationError // form errors keyed by field
	Form       url.Values            // submitted values to refill a form after an error
}

// initial is the first letter of a name, used as the board avatar.
func initial(name string) string {
	for _, r := range name {
		return strings.ToUpper(string(r))
	}
	return "?"
}

// prefix derives a 3-letter issue key from the board name ("Trello Clone" -> "TRE").
func prefix(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			if b.Len() >= 3 {
				break
			}
		}
	}
	if b.Len() == 0 {
		return "KAN"
	}
	return b.String()
}

// statusKind picks the Linear-style status icon for column i of n: first is
// backlog, last is done, the one before last is review, the rest progress.
func statusKind(i, n int) string {
	switch {
	case i == 0 && n > 1:
		return "backlog"
	case i == n-1 && n > 1:
		return "done"
	case i == 1 && n > 3:
		return "todo"
	case i == n-2 && n > 3:
		return "review"
	default:
		return "progress"
	}
}

// isHX reports whether htmx wants a fragment. htmx 4 marks body-level swaps,
// hx-select requests and history restores as "full": those get the layout.
func isHX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true" &&
		r.Header.Get("HX-Request-Type") != "full" &&
		r.Header.Get("HX-History-Restore-Request") != "true"
}

// render executes the named templates into a buffer first, so a template
// error becomes a clean 500 instead of a half-written page.
func (s *Server) render(w http.ResponseWriter, status int, data any, names ...string) {
	var buf bytes.Buffer
	for _, name := range names {
		if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			s.log.Error("render", "template", name, "err", err)
			http.Error(w, "erro ao renderizar", http.StatusInternalServerError)
			return
		}
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Add("Vary", "HX-Request")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// toast asks the client (app.js) to show a notification via the HX-Trigger header.
func toast(w http.ResponseWriter, level, msg string) {
	b, _ := json.Marshal(map[string]any{"toast": map[string]string{"level": level, "message": msg}})
	w.Header().Set("HX-Trigger", string(b))
}

// fail maps store errors to HTTP statuses. htmx requests get no swap and a
// toast (HX-Trigger) with the message.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, msg := http.StatusInternalServerError, "Erro interno. Veja os logs."
	var verr store.ValidationError
	var maxErr *http.MaxBytesError
	switch {
	case errors.Is(err, store.ErrNotFound):
		status, msg = http.StatusNotFound, "Não encontrado."
	case errors.Is(err, store.ErrColumnNotEmpty):
		status, msg = http.StatusConflict, "A coluna tem cards. Escolha para onde movê-los."
	case errors.As(err, &verr):
		// 400, not 422: 422 is reserved for HTML form re-renders that htmx swaps.
		status, msg = http.StatusBadRequest, verr.Error()
	case errors.As(err, &maxErr):
		status, msg = http.StatusRequestEntityTooLarge, "Conteúdo grande demais."
	case errors.Is(err, errBadRequest):
		status, msg = http.StatusBadRequest, "Requisição inválida."
	default:
		s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	}
	if r.Header.Get("HX-Request") == "true" {
		// htmx 4 swaps error responses too; keep the page and show a toast.
		w.Header().Set("HX-Reswap", "none")
		toast(w, "error", msg)
	} else if status == http.StatusNotFound {
		s.notFound(w, r)
		return
	}
	http.Error(w, msg, status)
}

var errBadRequest = errors.New("bad request")

func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, errBadRequest
	}
	return id, nil
}

func formInt(r *http.Request, name string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(r.FormValue(name)))
	return n
}

// filterFrom reads the board filter from the query string of u.
func (s *Server) filterFrom(q url.Values) store.Filter {
	f := store.Filter{Query: strings.TrimSpace(q.Get("q")), Today: s.today()}
	f.LabelID, _ = strconv.ParseInt(q.Get("label"), 10, 64)
	if p, err := strconv.Atoi(q.Get("priority")); err == nil && p >= 0 && p < len(store.Priorities) {
		f.Priority, f.HasPriority = p, true
	}
	f.Overdue = q.Get("overdue") == "1"
	return f
}

// currentFilter recovers the filter the user is looking at from the
// HX-Current-URL header, so mutations re-render the board as it was shown.
func (s *Server) currentFilter(r *http.Request) store.Filter {
	u, err := url.Parse(r.Header.Get("HX-Current-URL"))
	if err != nil {
		return store.Filter{Today: s.today()}
	}
	return s.filterFrom(u.Query())
}

func (s *Server) today() string { return s.now().Format("2006-01-02") }

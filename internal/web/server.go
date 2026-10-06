// Package web serves the HTMX user interface over net/http.
package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/AlexGalhardo/trello-golang-htmx/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

const (
	maxFormBytes   = 1 << 20  // 1 MiB for regular forms
	maxImportBytes = 20 << 20 // 20 MiB for JSON imports
)

type Server struct {
	store   *store.Store
	log     *slog.Logger
	version string
	tmpl    *template.Template
	now     func() time.Time
	handler http.Handler
	// backupMu serializes backups so concurrent clicks cannot pick the same
	// file name between the existence check and VACUUM INTO.
	backupMu sync.Mutex
}

// New builds the HTTP handler. Templates are parsed once here so a broken
// template fails at startup, not on the first request.
func New(st *store.Store, log *slog.Logger, version string) (*Server, error) {
	assets, err := assetHash()
	if err != nil {
		return nil, err
	}
	fm := template.FuncMap{"assetVersion": func() string { return assets }}
	for k, v := range funcs {
		fm[k] = v
	}
	tmpl, err := template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	s := &Server{store: st, log: log, version: version, tmpl: tmpl, now: time.Now}
	s.handler = s.middleware(s.routes())
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	static, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", cacheStatic(http.FileServerFS(static))))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /{$}", s.dashboard)
	mux.HandleFunc("/", s.fallback)

	mux.HandleFunc("GET /boards/new", s.newBoardDialog)
	mux.HandleFunc("POST /boards", s.createBoard)
	mux.HandleFunc("GET /boards/{id}", s.showBoard)
	mux.HandleFunc("GET /boards/{id}/settings", s.boardSettings)
	mux.HandleFunc("PUT /boards/{id}", s.updateBoard)
	mux.HandleFunc("POST /boards/{id}/archive", s.archiveBoard(true))
	mux.HandleFunc("POST /boards/{id}/restore", s.archiveBoard(false))
	mux.HandleFunc("DELETE /boards/{id}", s.deleteBoard)
	mux.HandleFunc("POST /boards/{id}/columns", s.createColumn)
	mux.HandleFunc("GET /boards/{id}/labels", s.labelsDialog)
	mux.HandleFunc("POST /boards/{id}/labels", s.createLabel)

	mux.HandleFunc("PUT /columns/{id}", s.updateColumn)
	mux.HandleFunc("DELETE /columns/{id}", s.deleteColumn)
	mux.HandleFunc("POST /columns/{id}/move", s.moveColumn)
	mux.HandleFunc("POST /columns/{id}/cards", s.createCard)

	mux.HandleFunc("GET /cards/{id}", s.cardDetail)
	mux.HandleFunc("GET /cards/{id}/card", s.cardFragment)
	mux.HandleFunc("GET /cards/{id}/title", s.cardTitleForm)
	mux.HandleFunc("PUT /cards/{id}/title", s.renameCard)
	mux.HandleFunc("PUT /cards/{id}", s.updateCard)
	mux.HandleFunc("DELETE /cards/{id}", s.deleteCard)
	mux.HandleFunc("POST /cards/{id}/move", s.moveCard)
	mux.HandleFunc("POST /cards/{id}/labels/{label}", s.toggleCardLabel)

	mux.HandleFunc("PUT /labels/{id}", s.updateLabel)
	mux.HandleFunc("DELETE /labels/{id}", s.deleteLabel)

	mux.HandleFunc("POST /markdown/preview", s.markdownPreview)
	mux.HandleFunc("GET /export", s.export)
	mux.HandleFunc("GET /import", s.importDialog)
	mux.HandleFunc("POST /import", s.importData)
	mux.HandleFunc("POST /backup", s.backup)
	return mux
}

// assetHash fingerprints the embedded static files. Asset URLs carry it as
// ?v=, so any change to CSS/JS busts the immutable browser cache even when
// the app version string did not change (dev builds, rebuilt releases).
func assetHash() (string, error) {
	h := sha256.New()
	err := fs.WalkDir(staticFS, "static", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := staticFS.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = h.Write([]byte(path))
		_, _ = h.Write(b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:12], err
}

func cacheStatic(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Assets are embedded in the binary; URLs carry ?v=<version> so a new
		// release busts the cache while repeat loads stay instant.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		h.ServeHTTP(w, r)
	})
}

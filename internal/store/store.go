// Package store persists boards, columns, cards and labels in SQLite.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/schema"

	"github.com/AlexGalhardo/vukem-kanban/database/migrations"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrColumnNotEmpty = errors.New("column has cards")
	// ErrConflict means the card changed since the caller loaded it.
	ErrConflict = errors.New("card changed since it was loaded")
)

// ValidationError maps a form field to a human readable message.
type ValidationError map[string]string

// fieldNames are the pt-BR labels used in user-facing messages.
var fieldNames = map[string]string{
	"name": "Nome", "description": "Descrição", "color": "Cor", "title": "Título",
	"priority": "Prioridade", "due_date": "Entrega", "wip_limit": "Limite WIP",
	"column_id": "Coluna", "before_id": "Posição", "label_id": "Label", "file": "Arquivo",
}

// Message is the user-facing form of the error ("Título: obrigatório").
func (v ValidationError) Message() string {
	parts := make([]string, 0, len(v))
	for k, msg := range v {
		label := fieldNames[k]
		if label == "" {
			label = k
		}
		parts = append(parts, label+": "+msg)
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for k, msg := range v {
		parts = append(parts, k+": "+msg)
	}
	sort.Strings(parts)
	return "validation failed: " + strings.Join(parts, "; ")
}

// Colors is the fixed palette shared by boards and labels. Keeping it closed
// lets templates map each value to a literal Tailwind class (no inline styles,
// so the CSP can forbid them).
var Colors = []string{"slate", "red", "orange", "amber", "green", "teal", "sky", "indigo", "violet", "pink"}

func validColor(c string) bool {
	for _, v := range Colors {
		if v == c {
			return true
		}
	}
	return false
}

type Store struct {
	db   *sql.DB
	path string
	now  func() time.Time
	// closer releases a database this store opened itself (see Open).
	closer func() error
}

// New wraps an open database whose schema is already migrated. The caller
// (the vukem app) owns the connection and closes it on shutdown.
func New(ctx context.Context, db *sql.DB) (*Store, error) {
	var path string
	if err := db.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&path); err != nil {
		return nil, fmt.Errorf("locate database file: %w", err)
	}
	return &Store{db: db, path: path, now: time.Now}, nil
}

// Open opens a standalone SQLite database at path, migrates it and returns a
// store that closes it. The app itself gets its connection from vukem; this
// is for tests and tools.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := database.Open(ctx, "file:"+filepath.ToSlash(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	m, err := schema.NewMigrator(db, migrations.All())
	if err == nil {
		err = migrations.Apply(ctx, db, m)
	}
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	s, err := New(ctx, db.SQL())
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	s.closer = db.Close
	return s, nil
}

// Close releases the database when this store opened it; it is a no-op for
// a store built with New.
func (s *Store) Close() error {
	if s.closer == nil {
		return nil
	}
	return s.closer()
}

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// Backup writes a consistent copy of the database to dest (which must not exist).
func (s *Store) Backup(ctx context.Context, dest string) error {
	_, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, dest)
	return err
}

func (s *Store) tx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// clampText cleans multi-line user text: invalid UTF-8 and invisible
// characters (NUL, zero-width, bidi controls) are dropped, newlines and tabs
// are kept, and the result is trimmed.
func clampText(s string) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

// singleLine is clampText for names and titles: any whitespace run,
// including newlines, collapses to one space.
func singleLine(s string) string { return strings.Join(strings.Fields(clampText(s)), " ") }

func checkLen(v ValidationError, field, value string, minLen, maxLen int) {
	n := len([]rune(value))
	switch {
	case n < minLen && minLen == 1:
		v[field] = "obrigatório"
	case n < minLen:
		v[field] = fmt.Sprintf("mínimo de %d caracteres", minLen)
	case n > maxLen:
		v[field] = fmt.Sprintf("máximo de %d caracteres", maxLen)
	}
}

func orNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func affectedOrNotFound(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

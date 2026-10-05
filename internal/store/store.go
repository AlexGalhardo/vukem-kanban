// Package store persists boards, columns, cards and labels in SQLite.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var (
	ErrNotFound       = errors.New("not found")
	ErrColumnNotEmpty = errors.New("column has cards")
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
}

// Open opens (creating if needed) the database at path and applies pending
// migrations. When an existing database has pending migrations, a copy is
// written next to it first so a failed upgrade never loses data.
func Open(ctx context.Context, path string) (*Store, error) {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite allows a single writer; one connection serializes everything and
	// avoids "database is locked" under concurrent requests.
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db, path: path, now: time.Now}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	applied := map[string]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return err
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	names, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	var pending []string
	for _, n := range names {
		if !applied[filepath.Base(n)] {
			pending = append(pending, n)
		}
	}
	if len(pending) == 0 {
		return nil
	}
	if len(applied) > 0 && s.path != ":memory:" {
		dest := fmt.Sprintf("%s.pre-%s.bak", s.path, strings.TrimSuffix(filepath.Base(pending[0]), ".sql"))
		if err := s.Backup(ctx, dest); err != nil {
			return fmt.Errorf("backup before migration: %w", err)
		}
	}
	for _, n := range pending {
		body, err := migrationsFS.ReadFile(n)
		if err != nil {
			return err
		}
		if err := s.tx(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, string(body)); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
				filepath.Base(n), s.now().Unix())
			return err
		}); err != nil {
			return fmt.Errorf("apply migration %s: %w", n, err)
		}
	}
	return nil
}

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

func clampText(s string) string { return strings.TrimSpace(s) }

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

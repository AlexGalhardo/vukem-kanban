// Package migrations holds the schema migrations. The SQL lives in sql/ so
// the files stay readable and diffable; the vukem migrator records them.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/AlexGalhardo/vukem/database"
	"github.com/AlexGalhardo/vukem/schema"
)

//go:embed sql/*.sql
var sqlFS embed.FS

// All lists the migrations in order.
func All() []schema.Migration {
	return []schema.Migration{
		fromSQL("0001_init", func(s *schema.Schema) (bool, error) { return s.HasTable("boards") }),
		fromSQL("0002_card_revision", func(s *schema.Schema) (bool, error) { return s.HasColumn("cards", "revision") }),
	}
}

// Source returns the SQL of migration id, for tests that build databases as
// an older release left them.
func Source(id string) (string, error) {
	b, err := sqlFS.ReadFile("sql/" + id + ".sql")
	return string(b), err
}

// fromSQL runs sql/<id>.sql. applied reports that databases written before
// the move to the vukem migrator (they tracked versions in their own
// schema_migrations table) already contain the change, so it is skipped.
func fromSQL(id string, applied func(*schema.Schema) (bool, error)) schema.Migration {
	return schema.Migration{
		ID: id,
		Up: func(s *schema.Schema) error {
			if done, err := applied(s); err != nil || done {
				return err
			}
			body, err := sqlFS.ReadFile("sql/" + id + ".sql")
			if err != nil {
				return err
			}
			return s.Raw(string(body))
		},
		Down: func(*schema.Schema) error { return fmt.Errorf("migration %s is forward-only", id) },
	}
}

// Apply runs the pending migrations. When the database already holds data
// and something is pending, it first writes a copy next to the file
// (<db>.pre-<migration>.bak) so a failed upgrade never loses data.
func Apply(ctx context.Context, db *database.DB, m *schema.Migrator) error {
	// Read before Pending: it creates the migrations table on a new database.
	existing := hasData(ctx, db.SQL())
	pending, err := m.Pending(ctx)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}
	path, err := filePath(ctx, db.SQL())
	if err != nil {
		return err
	}
	if path != "" && existing {
		dest := fmt.Sprintf("%s.pre-%s.bak", path, pending[0].ID)
		if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, dest); err != nil {
			return fmt.Errorf("backup before migration: %w", err)
		}
	}
	if _, err := m.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// filePath is the main database file, empty for in-memory databases.
func filePath(ctx context.Context, db *sql.DB) (string, error) {
	var path string
	err := db.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&path)
	return strings.TrimSpace(path), err
}

// hasData reports whether the database is not brand new: either it holds
// the legacy tracking table or vukem already recorded a migration.
func hasData(ctx context.Context, db *sql.DB) bool {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name IN ('schema_migrations', 'migrations', 'boards')`).Scan(&n)
	return err == nil && n > 0
}

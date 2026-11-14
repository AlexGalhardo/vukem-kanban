package bootstrap

import (
	"context"

	vukem "github.com/AlexGalhardo/vukem"

	"github.com/AlexGalhardo/trello-golang-htmx/database/migrations"
)

// Providers lists the application providers in registration order.
func Providers() []vukem.Provider {
	return []vukem.Provider{autoMigrate{}}
}

// autoMigrate applies pending migrations when the app starts. The kanban
// ships as a desktop app: nobody runs `vukem migrate` before opening it, and
// a new release has to upgrade the user's database by itself.
type autoMigrate struct{}

func (autoMigrate) Register(*vukem.App) error { return nil }

func (autoMigrate) Kinds() []vukem.Kind { return []vukem.Kind{vukem.KindWeb, vukem.KindTest} }

func (autoMigrate) Boot(ctx context.Context, app *vukem.App) error {
	db, err := app.DB()
	if err != nil {
		return err
	}
	m, err := app.Migrator()
	if err != nil {
		return err
	}
	return migrations.Apply(ctx, db, m)
}

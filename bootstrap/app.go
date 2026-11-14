// Package bootstrap wires the application together. There is no hidden
// registration: everything the app uses is listed here.
package bootstrap

import (
	vukem "github.com/AlexGalhardo/vukem"

	"github.com/AlexGalhardo/vukem-kanban/app/controllers"
	"github.com/AlexGalhardo/vukem-kanban/database/migrations"
	"github.com/AlexGalhardo/vukem-kanban/resources"
	"github.com/AlexGalhardo/vukem-kanban/routes"
)

// New builds the application; cmd/kanban and the tests call it.
func New(opts ...vukem.Option) (*vukem.App, error) {
	assets, err := resources.AssetHash()
	if err != nil {
		return nil, err
	}
	base := []vukem.Option{
		vukem.WithName("Kanban"),
		vukem.WithProviders(Providers()...),
		vukem.WithMigrations(migrations.All()),
		vukem.WithViews(resources.Views()),
		vukem.WithViewFuncs(controllers.Funcs(assets)),
	}
	for _, fn := range routes.All() {
		base = append(base, vukem.WithRoutes(fn))
	}
	return vukem.New(append(base, opts...)...)
}

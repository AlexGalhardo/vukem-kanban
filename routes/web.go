package routes

import (
	"context"

	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/middleware"
	"github.com/AlexGalhardo/vukem/router"

	"github.com/AlexGalhardo/vukem-kanban/app/controllers"
	"github.com/AlexGalhardo/vukem-kanban/internal/store"
	"github.com/AlexGalhardo/vukem-kanban/resources"
)

const maxFormBytes = 1 << 20 // 1 MiB for regular forms

// Web registers the browser routes. vukem route:list shows them.
func Web(r *router.Router, app *vukem.App) error {
	// Before the default stack: its method-override middleware parses form
	// bodies, so the cap must already be in place. Routes that take larger
	// bodies (the import) raise it with their own BodyLimit.
	r.UseServer(middleware.BodyLimit(maxFormBytes))

	// No sessions: the kanban has no users, so CSRF rests on the
	// Origin/Sec-Fetch-Site check alone.
	err := vukem.DefaultMiddlewareWith(app, r, vukem.MiddlewareOptions{
		Stateless: true,
		SecureHeaders: func(cfg *middleware.SecureHeadersConfig) {
			cfg.ReferrerPolicy = "no-referrer"
		},
	})
	if err != nil {
		return err
	}
	// Asset URLs carry ?v=<hash of the files>, so a changed file is a new URL
	// and the old one can stay cached for a year.
	r.UseServer(middleware.StaticFS(resources.Public(), middleware.StaticConfig{
		Prefix:            "/",
		ImmutablePrefixes: []string{"/static/"},
	}))

	db, err := app.DB()
	if err != nil {
		return err
	}
	st, err := store.New(context.Background(), db.SQL())
	if err != nil {
		return err
	}
	k := controllers.NewKanban(st, app.Log, app.Version, app.Now)
	k.Routes(r)
	return nil
}

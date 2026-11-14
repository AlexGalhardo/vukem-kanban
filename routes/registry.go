// Package routes declares the HTTP routes.
package routes

import (
	vukem "github.com/AlexGalhardo/vukem"
	"github.com/AlexGalhardo/vukem/router"
)

// All lists the route files, in order.
func All() []func(r *router.Router, app *vukem.App) error {
	return []func(r *router.Router, app *vukem.App) error{Web}
}

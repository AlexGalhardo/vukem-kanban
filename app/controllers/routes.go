package controllers

import (
	"github.com/AlexGalhardo/vukem/middleware"
	"github.com/AlexGalhardo/vukem/router"
)

// Routes registers every kanban route. Names follow the resource they act on.
func (k *Kanban) Routes(r *router.Router) {
	r.Get("/healthz", k.Healthz).Name("healthz")

	r.Get("/{$}", k.dashboard).Name("dashboard")

	r.Get("/boards/new", k.newBoardDialog).Name("boards.new")
	r.Post("/boards", k.form(k.createBoard)).Name("boards.store")
	r.Get("/boards/{id}", k.showBoard).Name("boards.show")
	r.Get("/boards/{id}/settings", k.boardSettings).Name("boards.settings")
	r.Put("/boards/{id}", k.form(k.updateBoard)).Name("boards.update")
	r.Post("/boards/{id}/archive", k.form(k.archiveBoard(true))).Name("boards.archive")
	r.Post("/boards/{id}/restore", k.form(k.archiveBoard(false))).Name("boards.restore")
	r.Delete("/boards/{id}", k.form(k.deleteBoard)).Name("boards.destroy")
	r.Post("/boards/{id}/columns", k.form(k.createColumn)).Name("columns.store")
	r.Get("/boards/{id}/labels", k.labelsDialog).Name("labels.index")
	r.Post("/boards/{id}/labels", k.form(k.createLabel)).Name("labels.store")

	r.Put("/columns/{id}", k.form(k.updateColumn)).Name("columns.update")
	r.Delete("/columns/{id}", k.form(k.deleteColumn)).Name("columns.destroy")
	r.Post("/columns/{id}/move", k.form(k.moveColumn)).Name("columns.move")
	r.Post("/columns/{id}/cards", k.form(k.createCard)).Name("cards.store")

	r.Get("/cards/{id}", k.cardDetail).Name("cards.show")
	r.Get("/cards/{id}/card", k.cardFragment).Name("cards.fragment")
	r.Get("/cards/{id}/title", k.cardTitleForm).Name("cards.title.edit")
	r.Put("/cards/{id}/title", k.form(k.renameCard)).Name("cards.title.update")
	r.Put("/cards/{id}", k.form(k.updateCard)).Name("cards.update")
	r.Delete("/cards/{id}", k.form(k.deleteCard)).Name("cards.destroy")
	r.Post("/cards/{id}/move", k.form(k.moveCard)).Name("cards.move")
	r.Post("/cards/{id}/labels/{label}", k.form(k.toggleCardLabel)).Name("cards.labels.toggle")

	r.Put("/labels/{id}", k.form(k.updateLabel)).Name("labels.update")
	r.Delete("/labels/{id}", k.form(k.deleteLabel)).Name("labels.destroy")

	r.Post("/markdown/preview", k.form(k.markdownPreview)).Name("markdown.preview")
	r.Get("/export", k.export).Name("export")
	r.Get("/import", k.importDialog).Name("import.dialog")
	r.Post("/import", k.importData).Name("import.store").Use(middleware.BodyLimit(MaxImportBytes))
	r.Post("/backup", k.form(k.backup)).Name("backup")

	// Unknown paths: a full "not found" page for browsers, text otherwise.
	r.Any("/{path...}", k.fallback).Name("fallback")
}

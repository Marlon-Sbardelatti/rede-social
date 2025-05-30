package routes

import (
	"github.com/go-chi/chi/v5"
	"main.go/app"
	"main.go/handlers"
)

func RegisterRoutes(r chi.Router, app *app.App) {
	// User
	r.Route("/user", func(r chi.Router) {
		r.Post("/", handlers.CreateUserHandler(app))
		r.Get("/", handlers.GetAllUsersHandler(app))
		r.Put("/", handlers.UpdateUserHandler(app))
	})
}

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
		r.Post("/{id}/follow/{second-id}", handlers.FollowUserHandler(app))
		r.Post("/{id}/unfollow/{second-id}", handlers.UnfollowUserHandler(app))
		r.Get("/", handlers.GetAllUsersHandler(app))
		r.Get("/{id}", handlers.GetUserByIdHandler(app))
		r.Get("/email/{email}", handlers.GetUserByEmailHandler(app))
		r.Put("/", handlers.UpdateUserHandler(app))
		r.Delete("/{id}", handlers.DeleteUserHandler(app))
	})
}

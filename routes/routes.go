package routes

import (
	"github.com/go-chi/chi/v5"
	"main.go/app"
	"main.go/handlers"
)

func RegisterRoutes(r chi.Router, app *app.App) {

	r.Route("/user", func(r chi.Router) {
		r.Post("/", handlers.CreateUserHandler(app))
		r.Post("/{id}/follow/{second-id}", handlers.FollowUserHandler(app))
		r.Post("/{id}/unfollow/{second-id}", handlers.UnfollowUserHandler(app))
		r.Post("/{id}/like/{post-id}", handlers.LikePostHandler(app))
		r.Post("/{id}/dislike/{post-id}", handlers.DislikePostHandler(app))
		r.Post("/login", handlers.LoginHandler(app))
		r.Get("/", handlers.GetAllUsersHandler(app))
		r.Get("/{requesterId}/profile/{id}", handlers.GetProfileHandler(app))
		r.Get("/{id}", handlers.GetUserByIdHandler(app))
		r.Get("/{id}/followers", handlers.GetFollowersHandler(app))
		r.Get("/{id}/following", handlers.GetFollowingHandler(app))
		r.Get("/email/{email}", handlers.GetUserByEmailHandler(app))
		r.Put("/", handlers.UpdateUserHandler(app))
		r.Delete("/{id}", handlers.DeleteUserHandler(app))
	})

	r.Route("/posts", func(r chi.Router) {
		r.Post("/", handlers.CreatePostHandler(app))
		r.Get("/", handlers.GetAllPostsHandler(app))
		r.Get("/{id}", handlers.GetPostsFromUserHandler(app))
		r.Delete("/{post-id}/user/{id}", handlers.DeletePostHandler(app))
	})
}

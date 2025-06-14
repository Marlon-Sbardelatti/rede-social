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
		r.Post("/{id}/like/{post-id}", handlers.LikePostRequest(app))
		r.Post("/{id}/dislike/{post-id}", handlers.DislikePostRequest(app))
		r.Get("/", handlers.GetAllUsersHandler(app))
		r.Get("/{id}", handlers.GetUserByIdHandler(app))
		r.Get("/{id}/followers", handlers.GetFollowersRequest(app))
		r.Get("/{id}/following", handlers.GetFollowingRequest(app))
		r.Get("/email/{email}", handlers.GetUserByEmailHandler(app))
		r.Put("/", handlers.UpdateUserHandler(app))
		r.Delete("/{id}", handlers.DeleteUserHandler(app))
	})

	r.Route("/posts", func(r chi.Router) {
		r.Post("/", handlers.CreatePostRequest(app))
		r.Get("/", handlers.GetAllPostsHandler(app))
		r.Get("/{id}", handlers.GetPostsFromUserRequest(app))
		r.Delete("/{post-id}/user/{id}", handlers.DeletePostRequest(app))
	})
}

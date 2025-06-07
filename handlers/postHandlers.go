package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"main.go/app"
	"main.go/models"
)

func CreatePostRequest(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(0)
		if err != nil {
			http.Error(w, "Error parsing multipart form", http.StatusBadRequest)
			return
		}

		post := extractPostFromForm(w, r)
		if post == nil {
			return
		}

		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		_, err = session.Run(
			ctx, 
			`
				CREATE (p: Post)
			`
			)

	}
}

func extractPostFromForm(w http.ResponseWriter, r *http.Request) *models.Post{
	userIdStr := r.FormValue("user_id")
	userId, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user id", http.StatusBadRequest)
		return nil
	}

	files := r.MultipartForm.File["images"]
	if len(files) > 20 {
		http.Error(w, "Too many images (max 20 allowed)", http.StatusBadRequest)
		return nil
	}

	var images [][]byte
	for _, fileHeader := range files {
		if fileHeader.Size > (50 << 20) {
			http.Error(w, "File too large", http.StatusBadRequest)
			return nil
		}

		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Invalid image", http.StatusBadRequest)
			return nil
		}

		imgData, err := io.ReadAll(file)
		file.Close()

		if err != nil {
			http.Error(w, "Invalid image", http.StatusBadRequest)
			return nil
		}

		images = append(images, imgData)
	}

	post := models.NewPost(userId, r.FormValue("description"), images)

	return post

}

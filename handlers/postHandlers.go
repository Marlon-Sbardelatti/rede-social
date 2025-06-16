package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
	"github.com/go-chi/chi/v5"
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

		userIdStr := r.FormValue("user_id")
		userId, err := strconv.ParseInt(userIdStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user id", http.StatusBadRequest)
			return
		}

		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		res, err := session.Run(
			ctx,
			`MATCH (u:User) WHERE id(u) = $user_id
				CREATE (p:Post {
					description: $description,
					created_at: $created_at
				})
				CREATE (u)-[:POSTED]->(p)
				RETURN id(p) AS post_id`,
			map[string]any{
				"user_id":     userId,
				"description": r.FormValue("description"),
				"created_at":  time.Now().UTC().Format(time.RFC3339),
			},
		)

		if err != nil {
			fmt.Println(err)
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		record, err := res.Single(ctx)
		if err != nil {
			http.Error(w, "Failed to create post", http.StatusInternalServerError)
			return
		}

		postIdInt, ok := record.Get("post_id")
		if !ok {
			http.Error(w, "Failed to retrieve post id", http.StatusInternalServerError)
			return
		}

		postId, ok := postIdInt.(int64)
		if !ok {
			http.Error(w, "Invalid post id type", http.StatusInternalServerError)
			return
		}

		paths := addImages(w, r, userId, postId)

		_, err = session.Run(
			ctx,
			`MATCH (p:Post) WHERE id(p) = $post_id
			 SET p.images = $images`,
			map[string]any{
				"post_id": postId,
				"images":  paths,
			},
		)

		if err != nil {
			http.Error(w, "Failed to update post images", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Post created"))

	}
}

func DeletePostRequest(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}
		postId, err := strconv.ParseInt(chi.URLParam(r, "post-id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}

		res, err := session.Run(
			ctx,
			`MATCH (u:User)-[r:POSTED]->(p:Post)
			 WHERE id(u) = $id AND id(p) = $postId
			 DETACH DELETE p 
			 RETURN COUNT(r) as count`,
			map[string]any{"id": id, "postId": postId},
		)
		// usamos detach pois o post esta relacionado a LIKED e POSTED, apenas DELETE só funciona
		// para nós simples sem relações

		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		record, err := res.Single(ctx)
		if err != nil {
			http.Error(w, "Unexpected DB result", http.StatusNotFound)
			return
		}

		count, ok := record.Get("count")
		if !ok {
			http.Error(w, "Error getting existence result", http.StatusInternalServerError)
			return
		}

		if count.(int64) == 0 {
			http.Error(w, "User or Post not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Deleted"))
	}
}

func addImages(w http.ResponseWriter, r *http.Request, userId int64, postId int64) []string {
	files := r.MultipartForm.File["images"]
	if len(files) > 20 {
		http.Error(w, "Too many images (max 20 allowed)", http.StatusBadRequest)
		return nil
	}

	var imagePaths []string
	for idx, fileHeader := range files {
		if fileHeader.Size > (50 << 20) {
			http.Error(w, "File too large", http.StatusBadRequest)
			return nil
		}

		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, "Invalid image", http.StatusBadRequest)
			return nil
		}
		defer file.Close()

		if err := os.MkdirAll(fmt.Sprintf("imgs/user-%d/post%d/", userId, postId), os.ModePerm); err != nil {
			http.Error(w, "Failed to save image", http.StatusInternalServerError)
			return nil
		}
		filename := fmt.Sprintf("imgs/user-%d/post%d/%d.jpg", userId, postId, idx)

		outFile, err := os.Create(filename)
		if err != nil {
			http.Error(w, "Failed to save image", http.StatusInternalServerError)
			return nil
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, file)
		if err != nil {
			http.Error(w, "Failed to save image", http.StatusInternalServerError)
			return nil
		}

		imagePaths = append(imagePaths, filename)
	}

	return imagePaths
}

func GetAllPostsHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		res, err := session.Run(ctx, `
			MATCH (u:User)-[:POSTED]->(p:Post)
			RETURN p, id(u) AS userId, u.name AS userName
		`, nil)
		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		posts := postRecordsToJSON(ctx, res)

		if err = res.Err(); err != nil {
			http.Error(w, "Result iteration error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(posts)
	}
}

func GetPostsFromUserRequest(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}

		res, err := session.Run(ctx, `
			MATCH (u:User)-[:POSTED]->(p:Post)
			WHERE id(u) = $id
			RETURN p, id(u) AS userId, u.name AS userName
		`, map[string]any{"id": id})
		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		posts := postRecordsToJSON(ctx, res)

		if err = res.Err(); err != nil {
			http.Error(w, "Result iteration error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(posts)
	}
}

func postRecordsToJSON(ctx context.Context, res neo4j.ResultWithContext) []models.Post {
	var posts []models.Post

	for res.Next(ctx) {
		record := res.Record()

		node, ok := record.Get("p")
		if !ok {
			continue
		}

		postNode := node.(neo4j.Node)
		props := postNode.Props

		userIdRaw, ok := record.Get("userId")
		if !ok {
			continue
		}
		userId := int64(userIdRaw.(int64))

		userNameRaw, ok := record.Get("userName")
		if !ok {
			continue
		}
		userName := userNameRaw.(string)

		var base64Images []string
		if imagesRaw, ok := props["images"].([]any); ok {
			for _, img := range imagesRaw {
				if pathStr, ok := img.(string); ok {
					imageBytes, err := os.ReadFile(pathStr)
					if err != nil {
						log.Printf("Erro ao ler imagem %s: %v", pathStr, err)
						continue
					}
					base64Images = append(base64Images, base64.StdEncoding.EncodeToString(imageBytes))
				}
			}
		}

		var createdAt time.Time
		if createdAtStr, ok := props["created_at"].(string); ok {
			newCreatedAt, err := time.Parse(time.RFC3339, createdAtStr)
			if err != nil {
				log.Printf("Erro ao converter created_at: %v", err)
				createdAt = time.Time{}
			} else {
				createdAt = newCreatedAt
			}
		}

		post := models.Post{
			Id:          postNode.GetId(),
			UserID:      userId,
			UserName:    userName,
			Description: props["description"].(string),
			CreatedAt:   createdAt,
			Images:      base64Images,
		}

		posts = append(posts, post)
	}

	return posts
}

func getImagesRecord(record *neo4j.Record, prop string) []string {
	imagesAny, ok := record.Get(prop)
	var imagesList []string
	if ok {
		follows, ok := imagesAny.([]any)
		if ok {
			for _, f := range follows {
				if imageStr, ok := f.(string); ok {
					imagesList = append(imagesList, imageStr)
				}
			}
		}
	}

	return imagesList
}

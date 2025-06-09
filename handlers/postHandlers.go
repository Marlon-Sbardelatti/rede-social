package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

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
					likes: $likes,
					created_at: $created_at
				})
				CREATE (u)-[:POSTED]->(p)
				RETURN id(p) AS post_id`,
			map[string]any{
				"user_id":     userId,
				"description": r.FormValue("description"),
				"likes":       []int64{},
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

		res, err := session.Run(
			ctx,
			`MATCH (p:Post)<-[r:POSTED]-(u:User)
			OPTIONAL MATCH (l:User)-[r:LIKED]->(p) 
			RETURN 
				p AS post, 
				id(u) AS authorId,
				collect(DISTINCT id(l)) AS likes
			ORDER BY p.createdAt DESC`,
			nil,
		)

		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		var posts []models.Post
		for res.Next(ctx) {
			record := res.Record()

			likes := getIDsRecord(record, "likes")
			images := getImagesRecord(record, "images")
			//createdAt := parseTimeFieldRecord(record, "createdAt")

			node, ok := record.Get("post")
			if ok {
				post_attr := node.(neo4j.Node).Props

				post := models.Post{
					Id:        		node.(neo4j.Node).GetId(),
					Description: 	post_attr["description"].(string),
					Likes:     		likes,
					Images:  		images,
					CreatedAt:		time.Time{},
				}

					posts = append(posts, post)
			}
		}

		postsJson, err := json.Marshal((posts))
		if err != nil {
			http.Error(w, "Error encoding posts to JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Conten-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(postsJson)
	}
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
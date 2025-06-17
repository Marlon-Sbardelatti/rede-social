package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"golang.org/x/crypto/bcrypt"
	"main.go/app"
	"main.go/models"
)

func CreateUserHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		name := r.FormValue("name")
		email := r.FormValue("email")
		password := r.FormValue("password")

		if name == "" || email == "" || password == "" {
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}

		// Verifica se o email ja existe
		exists, err := emailExists(ctx, session, email)
		if err != nil {
			http.Error(w, "Failed to check email", http.StatusInternalServerError)
			return
		}
		if exists {
			http.Error(w, "Email already in use", http.StatusConflict)
			return
		}

		hashedPassword, err := hashPassword(password)
		if err != nil {
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}

		res, err := session.Run(
			ctx,
			`CREATE (u:User {name: $name, email: $email, password: $password})
			 RETURN id(u) AS id`,
			map[string]any{"name": name, "email": email, "password": hashedPassword},
		)
		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		record, err := res.Single(ctx)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		resId, ok := record.Get("id")
		if !ok {
			http.Error(w, "Error getting user ID", http.StatusInternalServerError)
			return
		}

		userId, ok := resId.(int64)
		if !ok {
			http.Error(w, "Invalid user ID type", http.StatusInternalServerError)
			return
		}

		createUserImgsDir(userId)

		_, fileHeader, err := r.FormFile("image")
		// user tem img
		if err == nil {

			filename, err, code := createProfilePicture(userId, fileHeader)
			if err != nil {
				http.Error(w, err.Error(), code)
			}

			_, err = session.Run(
				ctx,
				`MATCH (u:User) WHERE id(u) = $id SET u.image = $imagePath`,
				map[string]any{"id": userId, "imagePath": filename},
			)
			if err != nil {
				http.Error(w, "Failed to update image path", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("User created"))
	}
}

func LoginHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		type LoginRequest struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		res, err := session.Run(ctx,
			`MATCH (u:User)
			 WHERE u.email = $email
			 RETURN u.password AS hash, id(u) as id, properties(u) as props`,
			map[string]any{"email": req.Email})

		if err != nil || !res.Next(ctx) {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		record := res.Record()
		storedHash := record.Values[0].(string)

		if !checkPasswordHash(req.Password, storedHash) {
			http.Error(w, "Invalid password", http.StatusUnauthorized)
			return
		}

		userId := record.Values[1].(int64)
		props := record.Values[2].(map[string]any)

		user := map[string]any{
			"id":    userId,
			"name":  props["name"],
			"email": props["email"],
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}
}

func createUserImgsDir(id int64) {
	if err := os.MkdirAll(fmt.Sprintf("imgs/user-%d", id), os.ModePerm); err != nil {
		log.Fatal(err)
	}
}

func GetAllUsersHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		res, err := session.Run(
			ctx,
			`MATCH (u:User)
			RETURN u`,
			nil,
		)

		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		usersJson, err, code := usersToJson(ctx, res, "u")
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}

		w.Header().Set("Conten-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(usersJson)
	}

}

func GetUserByIdHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}

		res, err := session.Run(
			ctx,
			`MATCH (u:User) WHERE id(u) = $id
			 RETURN 
				id(u) AS id, 
				properties(u) AS props`,
			map[string]any{"id": id},
		)

		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		user := recordToJSON(ctx, w, res)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(user)
	}
}

func GetProfileHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}

		requesterId, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}

		res, err := session.Run(
			ctx,
			`MATCH (u:User) WHERE id(u) = $profileId
			 OPTIONAL MATCH (requester:User)-[:FOLLOWS]->(u)
			 WHERE id(requester) = $requesterId
			 OPTIONAL MATCH (u)-[:POSTED]->(p:Post)
			 OPTIONAL MATCH (follower:User)-[:FOLLOWS]->(u)
			 OPTIONAL MATCH (u)-[:FOLLOWS]->(followed:User)
			 RETURN 
				id(u) AS id, 
				properties(u) AS props,
				CASE WHEN requester IS NULL THEN false ELSE true END AS isFollower, 
			    COUNT(DISTINCT p) as postCount, 
			    COUNT(DISTINCT follower) as totalFollowers,
			    COUNT(DISTINCT followed) as totalFollowed`,
			map[string]any{"profileId": id, "requesterId": requesterId},
		)

		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		user := recordToJSON(ctx, w, res)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(user)
	}
}

func GetUserByEmailHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		email := chi.URLParam(r, "email")
		res, err := session.Run(
			ctx,
			`MATCH (u:User) WHERE u.email = $email 
			 RETURN 
				id(u) AS id, 
				properties(u) AS props`,
			map[string]any{"email": email},
		)
		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		user := recordToJSON(ctx, w, res)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(user)
	}
}

func UpdateUserHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		var user struct {
			Id    int64  `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		err := decoder.Decode(&user)
		if err != nil || user.Name == "" || user.Email == "" {
			http.Error(w, "Invalid JSON or missing required fields", http.StatusBadRequest)
			return
		}

		res, err := session.Run(
			ctx,
			`MATCH (u:User) 
			 WHERE id(u) = $id 
			 SET u.name = $name, u.email = $email
			 RETURN 
				id(u) AS id, 
				properties(u) AS props`,
			map[string]any{
				"id":    user.Id,
				"name":  user.Name,
				"email": user.Email,
			},
		)
		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		newUser := recordToJSON(ctx, w, res)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(newUser)
	}
}

func DeleteUserHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}
		_, err = session.Run(
			ctx,
			"MATCH (u: User) WHERE id(u) = $id DELETE u",
			map[string]any{"id": id},
		)

		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func FollowUserHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		userId, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}

		otherId, err := strconv.ParseInt(chi.URLParam(r, "second-id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}

		res, err := session.Run(
			ctx,
			`MATCH (a:User), (b:User) 
			 WHERE id(a) = $userId AND id(b) = $otherId 
			 MERGE (a)-[r:FOLLOWS]->(b)
			 RETURN COUNT(r) as count`,
			map[string]any{"userId": userId, "otherId": otherId},
		)

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
			http.Error(w, "Users not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Followed"))
	}
}

func UnfollowUserHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		userId, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}

		otherId, err := strconv.ParseInt(chi.URLParam(r, "second-id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}

		res, err := session.Run(
			ctx,
			`MATCH (a:User)-[r:FOLLOWS]->(b:User) 
			 WHERE id(a) = $userId AND id(b) = $otherId 
			 DELETE r
			 RETURN COUNT(r) as count`,
			map[string]any{"userId": userId, "otherId": otherId},
		)

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
			http.Error(w, "Error getting deletion result", http.StatusInternalServerError)
			return
		}

		if count.(int64) == 0 {
			http.Error(w, "No following relantionship", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("Unfollowed"))

	}
}

func GetFollowersHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}

		res, err := session.Run(
			ctx,
			`MATCH (target: User) 
			 WHERE id(target) = $id
			 MATCH (follower:User)-[:FOLLOWS]->(target)
			 RETURN follower`,
			map[string]any{"id": id},
		)

		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		usersJson, err, code := usersToJson(ctx, res, "follower")
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}

		w.Header().Set("Conten-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(usersJson)

	}
}

func GetFollowingHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "Couldn't parse the url param", http.StatusInternalServerError)
			return
		}

		res, err := session.Run(
			ctx,
			`MATCH (u: User) 
			 WHERE id(u) = $id
			 MATCH (u)-[:FOLLOWS]->(followed:User)
			 RETURN followed`,
			map[string]any{"id": id},
		)

		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		usersJson, err, code := usersToJson(ctx, res, "followed")
		if err != nil {
			http.Error(w, err.Error(), code)
			return
		}

		w.Header().Set("Conten-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(usersJson)

	}
}

func LikePostHandler(app *app.App) http.HandlerFunc {
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
			`MATCH (u:User), (p:Post)
			 WHERE id(u) = $id AND id(p) = $postId
			 MERGE (u)-[r:LIKED]->(p)
			 RETURN COUNT(r) as count`,
			map[string]any{"id": id, "postId": postId},
		)

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
		w.Write([]byte("Liked"))
	}
}

func DislikePostHandler(app *app.App) http.HandlerFunc {
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
			`MATCH (u:User)-[r:LIKED]->(p:Post)
			 WHERE id(u) = $id AND id(p) = $postId
			 DELETE r
			 RETURN COUNT(r) as count`,
			map[string]any{"id": id, "postId": postId},
		)

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
		w.Write([]byte("Liked"))

	}
}

func recordToJSON(ctx context.Context, w http.ResponseWriter, res neo4j.ResultWithContext) []byte {
	record, err := res.Single(ctx)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return nil
	}

	resId, ok := record.Get("id")
	if !ok {
		http.Error(w, "Error getting user ID", http.StatusInternalServerError)
		return nil
	}

	props, ok := record.Get("props")
	if !ok {
		http.Error(w, "Error getting user properties", http.StatusInternalServerError)
		return nil
	}

	propsMap, ok := props.(map[string]any)
	if !ok {
		http.Error(w, "Error converting properties", http.StatusInternalServerError)
		return nil
	}
	propsMap["id"] = resId

	follows, ok := record.Get("isFollower")
	if ok {
		propsMap["follows"] = follows
	}

	postCount, ok := record.Get("postCount")
	if ok {
		propsMap["postCount"] = postCount
	}

	totalFollowers, ok := record.Get("totalFollowers")
	if ok {
		propsMap["followers"] = totalFollowers
	}

	totalFollowed, ok := record.Get("totalFollowed")
	if ok {
		propsMap["following"] = totalFollowed
	}

	if propsMap["image"] != nil {
		img, err := ImageToBase64(propsMap["image"].(string))

		if err != nil {
			http.Error(w, "Error encoding user to JSON", http.StatusInternalServerError)
			return nil
		}

		propsMap["image"] = img
	}

	user, err := json.Marshal(propsMap)
	if err != nil {
		http.Error(w, "Error encoding user to JSON", http.StatusInternalServerError)
		return nil
	}

	return user
}

func getIDsRecord(record *neo4j.Record, prop string) []int64 {
	followsAny, ok := record.Get(prop)
	var idList []int64
	if ok {
		// transforma de any para []any
		follows, ok := followsAny.([]any)
		if ok {
			for _, f := range follows {
				if id, ok := f.(int64); ok {
					idList = append(idList, id)
				}
			}
		}
	}

	return idList
}

func usersToJson(ctx context.Context, res neo4j.ResultWithContext, prop string) ([]byte, error, int) {
	var users []models.User
	for res.Next(ctx) {
		record := res.Record()

		node, ok := record.Get(prop)
		if ok {
			var user models.User
			user_attr := node.(neo4j.Node).Props

			imgPath := user_attr["image"]
			if imgPath != nil {
				// user com imagem
				img, err := ImageToBase64(user_attr["image"].(string))

				if err != nil {
					return nil, errors.New("Error encoding user to JSON"), 500
				}
				user = models.User{
					Id:       node.(neo4j.Node).GetId(),
					Name:     user_attr["name"].(string),
					Email:    user_attr["email"].(string),
					Password: user_attr["password"].(string),
					Image:    img,
				}
			} else {
				// user sem imagem
				user = models.User{
					Id:       node.(neo4j.Node).GetId(),
					Name:     user_attr["name"].(string),
					Email:    user_attr["email"].(string),
					Password: user_attr["password"].(string),
				}
			}

			users = append(users, user)
		}
	}

	if len(users) == 0 {
		return nil, errors.New("Not Found"), 404
	}

	usersJson, err := json.Marshal(users)
	if err != nil {
		return nil, errors.New("Error encoding users to JSON"), 500
	}

	return usersJson, nil, 200
}

func ImageToBase64(imagePath string) (string, error) {
	imageBytes, err := os.ReadFile(imagePath)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Erro ao ler imagem %s: %v", imagePath, err))
	}

	imageEncoded := base64.StdEncoding.EncodeToString(imageBytes)
	return imageEncoded, nil
}

func createProfilePicture(userId int64, fileHeader *multipart.FileHeader) (string, error, int) {
	if fileHeader.Size > (50 << 20) {
		return "", errors.New("File too large"), 400
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", errors.New("Invalid image"), 400
	}
	defer file.Close()

	dirPath := fmt.Sprintf("imgs/user-%d/profile-picture/", userId)
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return "", errors.New("Failed to save profile picture"), 500
	}

	filename := dirPath + "profile-picture.png"

	outFile, err := os.Create(filename)
	if err != nil {
		return "", errors.New("Failed to save image"), 500
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		return "", errors.New("Failed to save image"), 500
	}

	return filename, nil, 200
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(bytes), err
}

func checkPasswordHash(password string, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func emailExists(ctx context.Context, session neo4j.SessionWithContext, email string) (bool, error) {
	res, err := session.Run(ctx, `
		MATCH (u:User {email: $email})
		RETURN COUNT(u) > 0 AS exists
	`, map[string]any{"email": email})
	if err != nil {
		return false, err
	}

	record, err := res.Single(ctx)
	if err != nil {
		return false, err
	}

	exists, ok := record.Get("exists")
	if !ok {
		return false, nil
	}

	return exists.(bool), nil
}

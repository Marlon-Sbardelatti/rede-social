package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

		var user models.User
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		err := decoder.Decode(&user)
		if err != nil || user.Name == "" || user.Email == "" || user.Password == "" {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		psw, err := hashPassword(user.Password)
		if err != nil {
			http.Error(w, "Failed to hash user password", http.StatusInternalServerError)
			return
		}

		res, err := session.Run(
			ctx,
			`CREATE (u: User{name: $name, email: $email, password: $password})
			 RETURN 
				id(u) AS id`,
			map[string]any{"name": user.Name, "email": user.Email, "password": psw},
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

		userIdInt, ok := resId.(int64)
		if !ok {
			http.Error(w, "Invalid user ID type", http.StatusInternalServerError)
			return
		}

		createUserImgsDir(userIdInt)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("User created"))
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

		usersJson, err := usersToJson(ctx, res, "u")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

		var user models.User
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		err := decoder.Decode(&user)
		if err != nil || user.Name == "" || user.Email == "" || user.Password == "" {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		res, err := session.Run(
			ctx,
			`MATCH (u:User) 
			 WHERE id(u) = $id 
			 SET u.name = $name, u.email = $email, u.password = $password 
			 RETURN 
				id(u) AS id, 
				properties(u) AS props`,
			map[string]any{
				"id":       user.Id,
				"name":     user.Name,
				"email":    user.Email,
				"password": user.Password,
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

func GetFollowersRequest(app *app.App) http.HandlerFunc {
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

		usersJson, err := usersToJson(ctx, res, "follower")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Conten-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(usersJson)

	}
}

func GetFollowingRequest(app *app.App) http.HandlerFunc {
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

		usersJson, err := usersToJson(ctx, res, "followed")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Conten-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(usersJson)

	}
}

func LikePostRequest(app *app.App) http.HandlerFunc {
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

func DislikePostRequest(app *app.App) http.HandlerFunc {
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

func usersToJson(ctx context.Context, res neo4j.ResultWithContext, prop string) ([]byte, error) {
	var users []models.User
	for res.Next(ctx) {
		record := res.Record()

		node, ok := record.Get(prop)
		if ok {
			user_attr := node.(neo4j.Node).Props

			user := models.User{
				Id:       node.(neo4j.Node).GetId(),
				Name:     user_attr["name"].(string),
				Email:    user_attr["email"].(string),
				Password: user_attr["password"].(string),
			}

			users = append(users, user)
		}
	}

	usersJson, err := json.Marshal(users)
	if err != nil {
		return nil, errors.New("Error encoding users to JSON")
	}

	return usersJson, nil
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(bytes), err
}

func checkPasswordHash(password string, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

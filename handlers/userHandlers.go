package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
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

		res, err := session.Run(
			ctx,
			`CREATE (u: User{name: $name, email: $email, password: $password})
			 RETURN 
				id(u) AS id`,
			map[string]any{"name": user.Name, "email": user.Email, "password": user.Password},
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
			 OPTIONAL MATCH (u)-[:FOLLOWS]->(f:User)
			 OPTIONAL MATCH (follower:User)-[:FOLLOWS]->(u)
			RETURN 
				u, 
				collect(DISTINCT id(f)) AS follows,
				collect(DISTINCT id(follower)) AS followers`,
			nil,
		)

		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		var users []models.User
		for res.Next(ctx) {
			record := res.Record()

			follows := getIDsRecord(record, "follows")
			followers := getIDsRecord(record, "followers")

			node, ok := record.Get("u")
			if ok {
				user_attr := node.(neo4j.Node).Props

				user := models.User{
					Id:        node.(neo4j.Node).GetId(),
					Name:      user_attr["name"].(string),
					Email:     user_attr["email"].(string),
					Password:  user_attr["password"].(string),
					Follows:   follows,
					Followers: followers,
				}

				users = append(users, user)
			}
		}

		usersJson, err := json.Marshal(users)
		if err != nil {
			http.Error(w, "Error encoding users to JSON", http.StatusInternalServerError)
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
			 OPTIONAL MATCH (u)-[:FOLLOWS]->(f:User)
			 OPTIONAL MATCH (follower:User)-[:FOLLOWS]->(u)
			 RETURN 
				id(u) AS id, 
				properties(u) AS props, 
				collect(DISTINCT id(f)) AS follows,
				collect(DISTINCT id(follower)) AS followers`,
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
			 OPTIONAL MATCH (u)-[:FOLLOWS]->(f:User)
			 OPTIONAL MATCH (follower:User)-[:FOLLOWS]->(u)
			 RETURN 
				id(u) AS id, 
				properties(u) AS props, 
				collect(DISTINCT id(f)) AS follows,
				collect(DISTINCT id(follower)) AS followers`,
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
			 OPTIONAL MATCH (u)-[:FOLLOWS]->(f:User)
			 OPTIONAL MATCH (follower:User)-[:FOLLOWS]->(u)
			 SET u.name = $name, u.email = $email, u.password = $password 
			 RETURN 
				id(u) AS id, 
				properties(u) AS props, 
				collect(DISTINCT id(f)) AS follows,
				collect(DISTINCT id(follower)) AS followers`,
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

	follows := getIDsRecord(record, "follows")

	followers := getIDsRecord(record, "followers")

	propsMap, ok := props.(map[string]any)
	if !ok {
		http.Error(w, "Error converting properties", http.StatusInternalServerError)
		return nil
	}

	propsMap["id"] = resId
	if follows != nil {
		propsMap["follows"] = follows
		propsMap["followers"] = followers
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

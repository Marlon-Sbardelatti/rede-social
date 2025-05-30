package handlers

import (
	"context"
	"encoding/json"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"main.go/app"
	"main.go/models"
	"net/http"
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

		_, err = session.Run(
			ctx,
			"CREATE (u: User{name: $name, email: $email, password: $password})",
			map[string]any{"name": user.Name, "email": user.Email, "password": user.Password},
		)

		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("User created"))
	}
}

func GetAllUsersHandler(app *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		session := app.DB.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "neo4j"})
		defer session.Close(ctx)

		res, err := session.Run(
			ctx,
			"MATCH (u: User) RETURN u",
			nil,
		)

		if err != nil {
			http.Error(w, "DB operation failed", http.StatusInternalServerError)
			return
		}

		var users []models.User
		for res.Next(ctx) {
			record := res.Record()

			node, ok := record.Get("u")
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
			http.Error(w, "Error encoding users to JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Conten-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(usersJson)
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
			 RETURN id(u) AS id, properties(u) AS props`,
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

		record, err := res.Single(ctx)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		id, ok := record.Get("id")
		if !ok {
			http.Error(w, "Error getting user ID", http.StatusInternalServerError)
			return
		}

		props, ok := record.Get("props")
		if !ok {
			http.Error(w, "Error getting user properties", http.StatusInternalServerError)
			return
		}

		propsMap, ok := props.(map[string]any)
		if !ok {
			http.Error(w, "Error converting properties", http.StatusInternalServerError)
			return
		}

		propsMap["id"] = id

		newUserJson, err := json.Marshal(propsMap)
		if err != nil {
			http.Error(w, "Error encoding user to JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(newUserJson)
	}
}

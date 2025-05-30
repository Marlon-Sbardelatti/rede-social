package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"main.go/app"
	"main.go/db"
	"main.go/routes"
)

func main() {
	driver, err := db.InitDB()
	if err != nil {
		panic(err)
	}

	defer driver.Close(context.Background())

	app := &app.App{DB: driver}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	routes.RegisterRoutes(r, app)

	log.Println("Servidor rodando na porta 3000")
	err = http.ListenAndServe(":3000", r)
	if err != nil {
		panic(fmt.Errorf("Não foi possível inicializar o servidor, erro: %v",err))
	}
}

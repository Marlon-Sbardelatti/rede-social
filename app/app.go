package app

import "github.com/neo4j/neo4j-go-driver/v5/neo4j"

type App struct {
	DB neo4j.DriverWithContext
}

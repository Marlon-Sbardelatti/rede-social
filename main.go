package main

import (
	"context"
	"fmt"
	"log"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func main() {
	ctx := context.Background()
	driver, err := neo4j.NewDriverWithContext("bolt://localhost:7687", neo4j.BasicAuth("neo4j", "ihatewin", ""))
	if err != nil {
		log.Fatalf("Não foi possível se conectar ao drive do neo4j, erro: %v", err)
	}
	defer driver.Close(ctx)

	err = driver.VerifyConnectivity(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Println("Conexão estabelecida!")
}

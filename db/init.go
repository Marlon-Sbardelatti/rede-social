package db

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func InitDB() (neo4j.DriverWithContext, error) {
	ctx := context.Background()

	err := godotenv.Load(".env")
	if err != nil {
		return nil, fmt.Errorf("Arquivo de configuração .env não encontrado, erro: %v", err)
	}

	user := os.Getenv("USR")
	psw := os.Getenv("PSW")

	driver, err := neo4j.NewDriverWithContext("bolt://localhost:7687", neo4j.BasicAuth(user, psw, ""))
	if err != nil {
		return nil, fmt.Errorf("Não foi possível se conectar ao drive do neo4j, erro: %v", err)
	}

	err = driver.VerifyConnectivity(ctx)
	if err != nil {
		return nil, fmt.Errorf("Não foi possível estabelecer uma conexão com o neo4j, erro: %v", err)
	}
	fmt.Println("Conexão estabelecida!")
	return driver, nil
}

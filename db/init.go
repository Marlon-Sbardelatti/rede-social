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
		return nil, fmt.Errorf("arquivo de configuração .env não encontrado, erro: %v", err)
	}

	user := os.Getenv("USR")
	psw := os.Getenv("PSW")
	uri := os.Getenv("URI")

	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, psw, ""))
	if err != nil {
		return nil, fmt.Errorf("não foi possível se conectar ao driver do neo4j, erro: %v", err)
	}

	err = driver.VerifyConnectivity(ctx)
	if err != nil {
		return nil, fmt.Errorf("não foi possível estabelecer uma conexão com o neo4j, erro: %v", err)
	}
	fmt.Println("Conexão estabelecida!")
	return driver, nil
}

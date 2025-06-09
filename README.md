# Como rodar o projeto

1. Instale o Go (ao menos 1.24.3)

2. Crie uma conta neo4j ou acesse a sua conta.

3. Crie uma instância na nuvem ou baixe neo4j local e copie seu usuário e senha.

4. Insira essas informações num arquivo .env na raiz do projeto. Exemplo:

```.env
USR = neo4j
PSW = senhaGerada
URI = enderecoConexao (localhost ou neo4j+s://...)
```
5. Rode o projeto com o comando ```go run main.go```.
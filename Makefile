.PHONY: run build exe desktop test migrate-up migrate-down lint docker-up docker-down

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

# Ejecutable de Windows con interfaz y SQLite embebidos.
exe:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/parqueo.exe ./cmd/parqueo

desktop:
	go run ./cmd/parqueo

test:
	go test ./...

migrate-up:
	go run cmd/migrate/main.go up

migrate-down:
	go run cmd/migrate/main.go down

lint:
	golangci-lint run ./... 2>/dev/null || echo "golangci-lint no instalado"

docker-up:
	docker compose up -d

docker-down:
	docker compose down
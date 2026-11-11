.PHONY: run build test migrate-up migrate-down dbcheck lint

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./...

migrate-up:
	go run cmd/migrate/main.go up

migrate-down:
	go run cmd/migrate/main.go down

dbcheck:
	go run ./cmd/dbcheck

lint:
	golangci-lint run ./... 2>/dev/null || echo "golangci-lint no instalado"

docker-up:
	docker compose up -d

docker-down:
	docker compose down
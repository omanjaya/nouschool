# NouSchool — perintah pengembangan.
# Windows: jalankan via Git Bash / make dari MSYS, atau baca perintahnya manual.

DB_URL ?= $(DATABASE_URL)

.PHONY: dev build test migrate-up migrate-down sqlc web-dev web-build

dev: ## jalankan server backend (baca .env manual / set env dulu)
	go run ./cmd/server

build:
	go build -o bin/server.exe ./cmd/server

test:
	go test ./...

migrate-up:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$(DB_URL)" up

migrate-down:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$(DB_URL)" down

sqlc: ## generate kode type-safe dari queries.sql
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

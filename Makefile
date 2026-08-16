# NouSchool — perintah pengembangan.
# Windows: jalankan via Git Bash / make dari MSYS, atau baca perintahnya manual.

DB_URL ?= $(DATABASE_URL)
# DB_URL dari host ke container db (docker-compose.yml publish db ke localhost:5434).
DOCKER_DB_URL ?= postgres://nouschool:nouschool@localhost:5434/nouschool?sslmode=disable

.PHONY: dev build test migrate-up migrate-down sqlc web-dev web-build docker-up docker-down docker-logs docker-migrate

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

# --- Docker dev environment (lihat docker-compose.yml) ---

docker-up: ## nyalakan SEMUA: db + api (Air) + web (Vite) — semua log via docker
	docker compose up -d

docker-down: ## matikan & lepas container (data db tetap di volume nouschool_pgdata)
	docker compose down

docker-logs: ## ikuti log semua service (db + api + web)
	docker compose logs -f

docker-migrate: ## jalankan migrasi goose dari HOST ke db yang dipublish container (localhost:5434)
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$(DOCKER_DB_URL)" up

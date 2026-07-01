.PHONY: run build test migrate-up migrate-down migrate-create docker-up docker-down \
	prod-up prod-down prod-build prod-migrate-up prod-migrate-down

include .env
export

DEV=docker compose --profile tools run --rm files-dev
GOOSE=$(DEV) go run github.com/pressly/goose/v3/cmd/goose@latest
DB_URL_DOCKER=postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@files-db:5432/$(POSTGRES_DB)?sslmode=disable

run:
	docker compose up -d --build --force-recreate

build:
	docker compose build

test:
	$(DEV) go test ./... -v -count=1

migrate-up:
	$(GOOSE) -dir migrations postgres "$(DB_URL_DOCKER)" up

migrate-down:
	$(GOOSE) -dir migrations postgres "$(DB_URL_DOCKER)" down

migrate-create:
	$(GOOSE) -dir migrations create $(name) sql

docker-up:
	docker compose up -d

docker-down:
	docker compose down

PROD=docker compose -f docker-compose.prod.yml

prod-up:
	$(PROD) up -d --build

prod-down:
	$(PROD) down

prod-build:
	$(PROD) build

prod-migrate-up:
	$(PROD) run --rm --entrypoint goose files-app -dir /migrations postgres "$(DB_URL_DOCKER)" up

prod-migrate-down:
	$(PROD) run --rm --entrypoint goose files-app -dir /migrations postgres "$(DB_URL_DOCKER)" down

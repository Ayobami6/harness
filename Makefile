GOOSE ?= goose
GOOSE_VERSION ?= v3.29.0
DB_PATH ?= ./harness.db
MIGRATIONS_DIR ?= db/migrations

GOOSE_ARGS = -dir "$(MIGRATIONS_DIR)" sqlite3 "$(DB_PATH)"

DOCKER_COMPOSE ?= docker compose
DOCKER_DB_PATH ?= /data/harness.db
DOCKER_MIGRATIONS_DIR ?= /app/db/migrations
DOCKER_GOOSE_ARGS = -dir "$(DOCKER_MIGRATIONS_DIR)" sqlite3 "$(DOCKER_DB_PATH)"

.PHONY: goose-install migrate-create migrate-status migrate-up migrate-down test test-cov test-cov-html lint run up docker-up docker-down docker-migrate-status docker-migrate-up docker-migrate-down compose-up compose-down compose-migrate-status compose-migrate-up compose-migrate-down

COVERAGE_FILE ?= coverage.out

test:
	go test -race -v ./...

test-cov:
	go test -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	go tool cover -func=$(COVERAGE_FILE)

test-cov-html: test-cov
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html

lint:
	golangci-lint run --timeout 5m

goose-install:
	go install -tags='no_postgres no_mysql no_ydb no_mssql no_clickhouse no_azuresql no_vertica no_libsql' github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)

migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "Usage: make migrate-create name=add_example_table" >&2; \
		exit 2; \
	fi
	$(GOOSE) $(GOOSE_ARGS) create "$(name)" sql

migrate-status:
	$(GOOSE) $(GOOSE_ARGS) status

migrate-up:
	$(GOOSE) $(GOOSE_ARGS) up

migrate-down:
	$(GOOSE) $(GOOSE_ARGS) down

run:
	go run cmd/main.go

up:
	$(MAKE) migrate-up
	$(MAKE) run

docker-migrate-status:
	$(DOCKER_COMPOSE) run --rm harness goose $(DOCKER_GOOSE_ARGS) status

docker-migrate-up:
	$(DOCKER_COMPOSE) run --rm harness goose $(DOCKER_GOOSE_ARGS) up

docker-migrate-down:
	$(DOCKER_COMPOSE) run --rm harness goose $(DOCKER_GOOSE_ARGS) down

docker-up:
# 	$(MAKE) docker-migrate-up
	$(DOCKER_COMPOSE) up --build

docker-down:
	$(DOCKER_COMPOSE) down

compose-up: docker-up
compose-down: docker-down
compose-migrate-status: docker-migrate-status
compose-migrate-up: docker-migrate-up
compose-migrate-down: docker-migrate-down

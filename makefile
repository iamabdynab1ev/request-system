# Makefile for Goose migrations

# Variables
DB_DSN=postgres://postgres:postgres@localhost:5432/request-system?sslmode=disable
GOOSE_DIR=./database/migrations
GOOSE_DRIVER=postgres

.PHONY: migrate-create migrate-up migrate-up-by-one migrate-down migrate-down-by-one migrate-status migrate-reset

# Create a new migration file
migrate-create:
	@read -p "Enter migration name: " name; \
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" create "$$name" sql

# Apply all pending migrations
migrate-up:
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" up

# Apply next pending migration
migrate-up-by-one:
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" up-by-one

# Rollback all migrations
migrate-down:
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" down

# Rollback last applied migration
migrate-down-by-one:
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" down-by-one

# Show migration status
migrate-status:
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" status

# Reset database (down then up)
migrate-reset:
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" reset

# Create versioned migration files (recommended structure)
migrations-init:
	@mkdir -p ${GOOSE_DIR}
	@echo "Created migrations directory at ${GOOSE_DIR}"
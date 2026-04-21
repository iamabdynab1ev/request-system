# Makefile for Goose migrations

# Variables
DB_DSN ?= $(DATABASE_URL)
GOOSE_DIR=./database/migrations
GOOSE_DRIVER=postgres
CERT_PRIMARY_IP ?= 127.0.0.1

.PHONY: ensure-dsn migrate-create migrate-up migrate-up-by-one migrate-down migrate-down-by-one migrate-status migrate-reset

ensure-dsn:
ifndef DB_DSN
	$(error DB_DSN or DATABASE_URL must be set)
endif
ifeq ($(strip $(DB_DSN)),)
	$(error DB_DSN or DATABASE_URL must be set)
endif

# Create a new migration file
migrate-create: ensure-dsn
	@read -p "Enter migration name: " name; \
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" create "$$name" sql

# Apply all pending migrations
migrate-up: ensure-dsn
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" up

# Apply next pending migration
migrate-up-by-one: ensure-dsn
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" up-by-one

# Rollback all migrations
migrate-down: ensure-dsn
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" down

# Rollback last applied migration
migrate-down-by-one: ensure-dsn
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" down-by-one

# Show migration status
migrate-status: ensure-dsn
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" status

# Reset database (down then up)
migrate-reset: ensure-dsn
	goose -dir ${GOOSE_DIR} ${GOOSE_DRIVER} "${DB_DSN}" reset

# Create versioned migration files (recommended structure)
migrations-init:
	@mkdir -p ${GOOSE_DIR}
	@echo "Created migrations directory at ${GOOSE_DIR}"


# ==========================================
# SSL Certificate Generation (OpenSSL, development only)
# ==========================================
cert-gen:
	@echo "Создаем SSL сертификат для $(CERT_PRIMARY_IP)..."
	openssl req -x509 -nodes -newkey ec:<(openssl ecparam -name prime256v1) \
		-keyout server.key \
		-out server.crt \
		-days 3650 \
		-subj "/C=TJ/O=Bank HelpDesk SSL/CN=$(CERT_PRIMARY_IP)" \
		-addext "subjectAltName=IP:$(CERT_PRIMARY_IP),IP:127.0.0.1"
	@echo "✅ Сертификат создан (валиден 10 лет)"

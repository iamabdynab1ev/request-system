# Request System Backend

Backend service for HelpDesk / Request System.

## Requirements

- Go 1.24+
- PostgreSQL
- Redis
- `.env` file with required settings

## Run

Start the application:

```powershell
go run ./app
```

## Seeders

Core dictionaries:

```powershell
go run ./seeders/cmd/seed/main.go -core
```

Roles and admin:

```powershell
go run ./seeders/cmd/seed/main.go -roles
```

All seeders:

```powershell
go run ./seeders/cmd/seed/main.go -all
```

## Tests

```powershell
go test ./...
```

## Main Environment Variables

- `DATABASE_URL`
- `REDIS_ADDRESS`
- `JWT_SECRET_KEY`
- `SERVER_PORT`
- `SERVER_BASE_URL`
- `FRONTEND_BASE_URL`
- `ALLOWED_ORIGINS`
- `APP_TIMEZONE`
- `ONE_C_API_KEY`
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_BOT_USERNAME`
- `TELEGRAM_WEBHOOK_SECRET_TOKEN`
- `TELEGRAM_ADVANCED_MODE_ENABLED`
- `SSL_CERT_PATH`
- `SSL_KEY_PATH`

## Runtime Notes

- Goose migrations run on startup. If migrations fail, the server does not start.
- `GET /ping` is available as a simple health endpoint.
- Dashboard access requires `dashboard:view`.
- `/api/sync/1c` is disabled when `ONE_C_API_KEY` is empty.
- WebSocket authentication uses `Authorization: Bearer <token>` or `Sec-WebSocket-Protocol: bearer, <token>`.
- Telegram deep link can be built from `TELEGRAM_BOT_USERNAME` and is returned by `POST /api/profile/telegram/generate-token` as `bot_link`.
- Telegram webhook registration requires `SERVER_BASE_URL` with `https://...`; optional request validation uses `TELEGRAM_WEBHOOK_SECRET_TOKEN`.
- For domain rollout over HTTPS by IP, use the AD CS flow in `docs/ad-ip-certificate-rollout.md` instead of a plain self-signed server certificate.

## Project Structure

- `app/` - application entrypoint
- `internal/controllers/` - HTTP controllers
- `internal/services/` - business logic
- `internal/repositories/` - database access
- `internal/routes/` - route wiring
- `database/migrations/` - Goose migrations
- `seeders/` - seed scripts
- `pkg/` - shared infrastructure and utilities

## API Documentation

- `docs/API.md` - practical map of current HTTP endpoints and integration entrypoints.
- `docs/loadtest-checklist.md` - commands for dashboard, order list, and history load checks.
- `docs/deployment-checklist.md` - safe backend rollout, smoke checks, and rollback steps.

## Repository Rules

- Do not commit `.env`, certificates, keys, archives, or uploaded files.
- Keep migrations in `database/migrations/` under version control.
- Do not store generated binaries in the repository.

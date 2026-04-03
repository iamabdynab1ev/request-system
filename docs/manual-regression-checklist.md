# Manual Regression Checklist

## Before Start

1. Start backend:
   ```powershell
   go run ./app
   ```
2. Verify health check:
   `GET /ping -> 200 OK`
3. Verify migrations are applied on startup.
4. Verify seeders were applied for permissions and roles if dashboard permissions changed:
   ```powershell
   go run ./seeders/cmd/seed/main.go -all
   ```

## Telegram Prerequisites

Check `.env`:

- `SERVER_BASE_URL=https://<public-url>`
- `TELEGRAM_BOT_TOKEN=...`
- `TELEGRAM_BOT_USERNAME=...`
- `TELEGRAM_WEBHOOK_SECRET_TOKEN=...`
- `TELEGRAM_ADVANCED_MODE_ENABLED=true`

Expected:

- backend log contains `Telegram webhook registered`
- Telegram bot responds to commands

## Telegram Linking

1. Call:
   `POST /api/profile/telegram/generate-token`
2. Verify response contains:
   - `token`
   - `bot_link`
3. Open `bot_link` or send `/start <token>` manually in Telegram.

Expected:

- account is linked successfully
- `/status` shows current linked user
- `/unlink` asks for confirmation before unlink

## Telegram Main Flow

1. Run `/start`
2. Open main menu
3. Open `Мои заявки`
4. Open one order card
5. Return back
6. Open `Статистика`
7. Open `Справка`
8. Run `/status`

Expected:

- bot keeps one live screen instead of spamming chat with old screens
- inline buttons do not freeze
- main menu remains available
- `/status` shows linked account and chat id

## Telegram Search

1. Open `Поиск`
2. Enter exact order number that returns 1 result
3. Return back
4. Enter query that returns several results
5. Open one result
6. Return back
7. Enter query with no results

Expected:

- search respects project access rules
- inaccessible order is not opened
- back from order card returns to search results, not to unrelated list
- no-result search keeps user in search flow

## Telegram Order Actions

1. Open active order
2. Change status
3. Change deadline
4. Add comment
5. Delegate to another user
6. Save changes

Expected:

- callbacks confirm quickly
- no endless loading spinner
- validation errors stay in the same screen
- after save, screen stays consistent

## Telegram Closed Orders

1. Open closed order

Expected:

- order card opens in read-only mode
- order data is visible
- edit controls are hidden

## Telegram Rebinding

1. Link bot to user A
2. Generate new token for user B
3. Send `/start <token-for-B>` from the same Telegram account

Expected:

- Telegram account is reassigned to user B
- `/status` shows user B

## Dashboard Access

1. Open `/api/dashboard` with user without `dashboard:view`
2. Open `/api/dashboard` with user with `dashboard:view`

Expected:

- first user gets `403`
- second user gets `200`

## Dashboard Periods

Check:

- `period=today`
- `period=7d`
- `period=30d`
- `period=month`
- `period=custom&date_from=YYYY-MM-DD&date_to=YYYY-MM-DD`

Expected:

- `meta.period`, `meta.date_from`, `meta.date_to` match requested period
- period-based widgets change when period changes

## Dashboard Scope

Check with users having:

- `scope:own`
- `scope:office`
- `scope:branch`
- `scope:department`
- `scope:all`

Expected:

- `meta.effective_scope` matches applied access
- numbers and lists are limited by user scope

## Dashboard Metrics

Create and update orders through this sequence:

1. `OPEN -> IN_PROGRESS -> COMPLETED`
2. `COMPLETED -> REFINEMENT`
3. `REFINEMENT -> COMPLETED`
4. `COMPLETED -> CLOSED`

Expected:

- `COMPLETED` writes resolution metrics
- `REFINEMENT` clears resolution metrics
- repeated `COMPLETED` recalculates metrics
- `CLOSED` does not overwrite completion metrics

## Dashboard Structure Rules

Check orders with these combinations:

1. `department_id` only
2. `branch_id` only
3. `department_id + branch_id + office_id`

Expected:

- if `department_id` exists, order goes to `departments`
- if `department_id` is empty and `branch_id` exists, order goes to `branches`
- branch order must not be double-counted in both blocks

## Dashboard Data Shape

Expected:

- `meta.timezone` uses `APP_TIMEZONE`
- `last_activity` is `[]`, not `null`
- empty period widgets return empty arrays consistently

## Post-Release Smoke Check

1. `go test ./...`
2. `GET /ping`
3. Telegram `/start`
4. Telegram `Мои заявки`
5. Telegram `Поиск`
6. `GET /api/dashboard?period=30d`
7. `GET /api/dashboard?period=custom&date_from=<from>&date_to=<to>`

Expected:

- application starts cleanly
- bot responds
- dashboard responds
- no obvious regressions in logs

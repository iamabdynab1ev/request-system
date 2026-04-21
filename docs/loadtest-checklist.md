# Backend Load Test Checklist

Use this checklist before production rollout or after heavy dashboard/order
changes. Replace placeholders before running:

- `BASE_URL`: backend URL, for example `https://192.168.10.79:8091`.
- `TOKEN`: valid access token.
- `ORDER_ID`: existing order ID visible to the token user.

## Smoke Check

```powershell
curl.exe -k "$env:BASE_URL/ping"
```

Expected result:

- HTTP `200`.
- No TLS/runtime startup errors in backend logs.

## Dashboard

```powershell
go run ./tools/backend_loadtest `
  -url "$env:BASE_URL/api/dashboard?period=month&scope=all" `
  -token "$env:TOKEN" `
  -requests 200 `
  -concurrency 20 `
  -timeout 20s `
  -insecure
```

Watch:

- `p95` latency.
- non-`2xx` responses.
- PostgreSQL CPU and active connections.
- Redis availability.

## Orders List

```powershell
go run ./tools/backend_loadtest `
  -url "$env:BASE_URL/api/order?withPagination=true&page=1&limit=50" `
  -token "$env:TOKEN" `
  -requests 300 `
  -concurrency 30 `
  -timeout 20s `
  -insecure
```

Watch:

- pagination response time.
- slow SQL caused by text search or broad filters.
- DB pool saturation.

## Order History

```powershell
go run ./tools/backend_loadtest `
  -url "$env:BASE_URL/api/order/$env:ORDER_ID/history" `
  -token "$env:TOKEN" `
  -requests 200 `
  -concurrency 20 `
  -timeout 20s `
  -insecure
```

Watch:

- history response time.
- repeated dictionary/user lookups.
- `STRUCTURE_CHANGE` text readability.

## Telegram Hot Paths

```powershell
go run ./tools/telegram_loadtest -h
```

Use the Telegram-specific tool only with a safe test bot/token and non-production
chat IDs unless the operation is read-only.

## Basic Acceptance Targets

These are starting targets, not final SLA:

- `p95` dashboard: under `2s` for warm cache.
- `p95` order list: under `1s` for common filters.
- `p95` order history: under `1s` for normal history size.
- error rate: `0%` for valid authenticated requests.

If the first dashboard request after cache invalidation is slower, repeat the
same test and compare cold-cache vs warm-cache numbers.

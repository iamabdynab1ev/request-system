# API Overview

This document is a practical map of the current backend API. It is not a full
OpenAPI contract yet, but it gives one place to check the main endpoints,
authentication model, and integration entrypoints.

## Base

- Health check: `GET /ping`
- Main API prefix: `/api`
- Authenticated endpoints require `Authorization: Bearer <access_token>`.
- Most business endpoints also require RBAC permissions through `AuthorizeAny`.

## Auth

- `POST /api/auth/login` - login and receive tokens.
- `POST /api/auth/refresh_token` - refresh access token.
- `GET /api/auth/me` - current user profile.
- `POST /api/auth/logout` - logout.
- `PUT /api/auth/me` - update current user profile.
- `POST /api/auth/password/request` - request password reset.
- `POST /api/auth/password/verify_phone` - verify reset code.
- `POST /api/auth/password/reset` - reset password.

## Users And AD

- `GET /api/users` - list users.
- `POST /api/users` - create user.
- `GET /api/users/:id` - get user by ID.
- `PUT /api/users/:id` - update user.
- `DELETE /api/users/:id` - delete user.
- `GET /api/users/permission/:id` - get user permissions.
- `PUT /api/users/permission/:id` - update user permissions.
- `GET /api/ad-users` - search users in AD.
- `POST /api/users/bind-ad-usernames` - mass-bind local users to AD usernames.

## Orders

- `POST /api/order` - create order.
- `GET /api/order` - list orders with filters and pagination.
- `GET /api/order/:id` - get order by ID.
- `PUT /api/order/:id` - update order.
- `DELETE /api/order/:id` - delete order.
- `GET /api/order/:orderID/history` - get order timeline/history.
- `GET /api/order-comments` - list order comments.
- `GET /api/order-comment/:id` - get order comment.
- `POST /api/order-comment` - create order comment.
- `PUT /api/order-comment/:id` - update order comment.
- `DELETE /api/order-comment/:id` - delete order comment.
- `GET /api/attachment` - list order attachments.
- `DELETE /api/attachment/:id` - delete attachment.

## Dashboard And Reports

- `GET /api/dashboard` - dashboard statistics by period and scope.
- `GET /api/report` - report data.

Dashboard business rules:

- `total_orders` counts orders created in the selected period.
- `open_orders` counts all current orders except `CLOSED`.
- `resolved_orders` counts orders that reached final `CLOSED` in the period.
- `sla_compliance` uses only closed orders with `duration`.
- `avg_resolve_time` uses orders closed in the period and measures full time up to final `CLOSED`.
- `active_agents` counts real executor activity in the selected period.
- Reports use final `CLOSED` transition for completion date, SLA status, and resolution time.

## Telegram

- `GET /api/profile/telegram` - get Telegram link status for current user.
- `DELETE /api/profile/telegram` - unlink Telegram from current user.
- `POST /api/profile/telegram/generate-token` - generate Telegram deep-link token.
- `POST /api/webhooks/telegram` - Telegram webhook endpoint.

Notes:

- Webhook registration requires `SERVER_BASE_URL` with `https://`.
- Optional webhook request validation uses `TELEGRAM_WEBHOOK_SECRET_TOKEN`.
- Bot menus and actions are handled inside `internal/controllers/telegram`.

## 1C Sync

- `POST /api/sync/1c` - receive 1C sync payload.

Notes:

- This endpoint is enabled only when `ONE_C_API_KEY` is set.
- It is protected by API key middleware.
- The sync service uses a single-run guard so concurrent sync runs do not overlap.

## Equipment

- `GET /api/equipment` - list equipment.
- `POST /api/equipment` - create equipment.
- `GET /api/equipment/:id` - get equipment by ID.
- `PUT /api/equipment/:id` - update equipment.
- `DELETE /api/equipment/:id` - delete equipment.
- `POST /api/equipment-import` - import equipment from Excel files.
- `GET /api/equipment-types` - list equipment types.
- `POST /api/equipment-types` - create equipment type.
- `GET /api/equipment-types/:id` - get equipment type by ID.
- `PUT /api/equipment-types/:id` - update equipment type.
- `DELETE /api/equipment-types/:id` - delete equipment type.

## Dictionaries And Structure

The following dictionaries follow the standard CRUD shape:

- Branches: `/api/branches`
- Departments: `/api/departments`
- Otdels: `/api/otdels`
- Offices: `/api/offices`
- Positions: `/api/positions`
- Priorities: `/api/priorities`
- Statuses: `/api/statuses`
- Roles: `/api/roles`
- Permissions: `/api/permissions`
- Role permissions: `/api/role-permissions`
- Order types: `/api/order-types`
- Order routing rules: `/api/order-routing-rules`

Standard CRUD shape:

- `GET <base>` - list.
- `POST <base>` - create.
- `GET <base>/:id` - get by ID.
- `PUT <base>/:id` - update.
- `DELETE <base>/:id` - delete.

## WebSocket

- `GET /api/ws` - websocket connection.

Authentication:

- `Authorization: Bearer <token>`, or
- `Sec-WebSocket-Protocol: bearer, <token>`.

## Operational Notes

- Goose migrations run during application startup.
- Redis is used for cache and Telegram link/state flows.
- PostgreSQL pool is configured through `DB_POOL_*` environment variables.
- HTTPS is configured through `SSL_CERT_PATH` and `SSL_KEY_PATH`.

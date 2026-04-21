# Deployment Checklist

Use this checklist for backend rollout to a Windows server.

## 1. Before Build

- Check current branch and commit:

```powershell
git status --branch --short
git log -1 --oneline
```

- Run tests:

```powershell
go test ./...
```

- Confirm required environment variables exist on the server:

```text
DATABASE_URL
REDIS_ADDRESS
JWT_SECRET_KEY
SERVER_PORT
SERVER_BASE_URL
FRONTEND_BASE_URL
ALLOWED_ORIGINS
SSL_CERT_PATH
SSL_KEY_PATH
```

Optional integration variables:

```text
ONE_C_API_KEY
TELEGRAM_BOT_TOKEN
TELEGRAM_BOT_USERNAME
TELEGRAM_WEBHOOK_SECRET_TOKEN
LDAP_SEARCH_ENABLED
LDAP_BIND_DN
LDAP_BIND_PASSWORD
```

## 2. Build

```powershell
go build -o server_new.exe ./app
```

Check the file:

```powershell
Get-Item .\server_new.exe | Select-Object FullName,Length,LastWriteTime
```

## 3. Backup On Server

On the backend server:

```powershell
cd C:\apps\request-system
Copy-Item .\server.exe .\server_backup_YYYYMMDD_HHMM.exe
```

If the server uses local config/certs, verify they exist:

```powershell
Test-Path .\.env
Test-Path C:\apps\request-system\certs\ad\server.crt
Test-Path C:\apps\request-system\certs\ad\server.key
```

## 4. Stop Service

If backend runs as Windows service:

```powershell
Stop-Service request-system
```

If backend runs as a process:

```powershell
Stop-Process -Name server -Force
```

## 5. Replace Binary

```powershell
Copy-Item C:\path\to\server_new.exe C:\apps\request-system\server.exe -Force
```

## 6. Start Service

If backend runs as Windows service:

```powershell
Start-Service request-system
```

If backend runs as a process:

```powershell
cd C:\apps\request-system
.\server.exe
```

## 7. Smoke Checks

Health check:

```powershell
curl.exe -k https://192.168.10.79:8091/ping
```

Dashboard:

```powershell
curl.exe -k -H "Authorization: Bearer <TOKEN>" "https://192.168.10.79:8091/api/dashboard?period=month&scope=all"
```

Orders list:

```powershell
curl.exe -k -H "Authorization: Bearer <TOKEN>" "https://192.168.10.79:8091/api/order?withPagination=true&page=1&limit=20"
```

Order history:

```powershell
curl.exe -k -H "Authorization: Bearer <TOKEN>" "https://192.168.10.79:8091/api/order/<ORDER_ID>/history"
```

Telegram:

- Open bot.
- Press `Все заявки`.
- Open an order card.
- Check pagination buttons.
- Check that text is readable.

## 8. Rollback

If startup or smoke checks fail:

```powershell
Stop-Service request-system
Copy-Item .\server_backup_YYYYMMDD_HHMM.exe .\server.exe -Force
Start-Service request-system
```

For process-based startup:

```powershell
Stop-Process -Name server -Force
Copy-Item .\server_backup_YYYYMMDD_HHMM.exe .\server.exe -Force
.\server.exe
```

## 9. After Rollout

- Check backend logs for startup and migration errors.
- Check PostgreSQL active connections.
- Check Redis availability.
- Run `docs/loadtest-checklist.md` commands for dashboard, orders, and history.

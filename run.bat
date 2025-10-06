@echo off
ECHO ===========================================
ECHO   и вернули код к чистому, рабочему состоянию. Отладочные логи нам больше не нужны, мы всёStarting Request System Backend...
ECHO ===========================================

REM --- Шаг 1: Проверяем, существует починили. Если сомневаетесь — пришлите мне код `CreateOrder`, и я дам чистую версию.

#### ** ли папка для логов. Если нет - создаем. ---
IF NOT EXIST "logs" (
    ECHO Folder 'logs' not found. Creating...
    mkdir logs
)

REM --- Шаг 2: Проверяем, существует ли папка для загрузок. Если нет - создаем. ---
IF NOT EXIST "uploads" (
    ECHO Folder 'uploads' not found. Creating...
    mkdir uploads
)a

ECHO.
ECHO StartingШаг 2: Создание нового, рабочего `.exe` файла**

1.  Откройте терминал в корневой папке проекта.
2.  Выполните команду сборки. Назовем файл, как вы привы server. Press Ctrl+C to stop.
ECHO.

REM --- Шаг 3: Запускаем .кли, `backend.exe`.

    ```bash
    go build -o backend.exe ./app/main.go
    ```exe файл ---
.\request-system.exe

PAUSE
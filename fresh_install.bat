@echo off
ECHO ==========================================================
ECHO ==           СЦЕНАРИЙ "ЧИСТОЙ" УСТАНОВКИ              ==
ECHO ==========================================================
ECHO.

ECHO [1/6] Установка временной переменной для чистой БД...
set FRESH_DB_NAME=request_system_test
ECHO      > Готово. Будет использоваться временная база: %FRESH_DB_NAME%
ECHO.

ECHO [2/6] Проверка и создание рабочих папок...
IF NOT EXIST "logs" (mkdir logs)
IF NOT EXIST "uploads" (mkdir uploads)
ECHO      > Готово. Папки 'logs' и 'uploads' проверены.
ECHO.

ECHO [3/6] Сборка исполняемых файлов...
go build -o seeder.exe ./cmd/seeder/main.go
go build -o request-system.exe ./app/main.go
ECHO      > Готово. Файлы seeder.exe и request-system.exe созданы.
ECHO.

ECHO [4/6] Применение миграций к ЧИСТОЙ БД...
goose -dir ./database/migrations up
ECHO      > Готово. Структура чистой базы данных создана.
ECHO.

ECHO [5/6] Наполнение ЧИСТОЙ базы начальными данными...
seeder.exe
ECHO      > Готово. Чистая база данных наполнена.
ECHO.

ECHO [6/6] Запуск основного приложения с ЧИСТОЙ БД...
ECHO.
request-system.exe

pause
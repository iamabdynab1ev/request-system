// Файл: pkg/database/postgresql/postgresql.go (или connection.go)
package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"

	// <<<--- ПРАВИЛЬНЫЕ ИМПОРТЫ ДЛЯ GOOSE ---
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // Драйвер pgx для database/sql
	"github.com/pressly/goose/v3"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// <<<--- НОВАЯ, ПРАВИЛЬНАЯ ФУНКЦИЯ ЗАПУСКА МИГРАЦИЙ ДЛЯ GOOSE ---
func runGooseMigrations(dsn string) {
	log.Println("ℹ️ Запуск миграций с помощью библиотеки Goose...")
	// Goose требует стандартный *sql.DB для работы
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Ошибка открытия соединения для миграций: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Ошибка пинга БД для миграций: %v", err)
	}

	// Указываем папку, где лежат наши миграции
	migrationsDir := "database/migrations"
	log.Printf("...поиск миграций в папке: %s", migrationsDir)

	// Применяем все .up.sql миграции
	if err := goose.Up(db, migrationsDir); err != nil {
		log.Fatalf("Ошибка применения Goose миграций: %v", err)
	}

	log.Println("✅ Миграции Goose успешно применены.")
}

func ConnectDB() *pgxpool.Pool {
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbHost := getEnv("DB_HOST", "localhost")
	dbName := getEnv("DB_NAME", "request-system")
	dbPortStr := getEnv("DB_PORT", "5432")

	dbPort, err := strconv.Atoi(dbPortStr)
	if err != nil {
		log.Fatalf("Неверное значение порта в DB_PORT: %v", err)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)


	//runGooseMigrations(dsn)


	log.Printf("ℹ️ Попытка подключения к БД: %s", fmt.Sprintf("postgresql://%s:***@%s:%d/%s", dbUser, dbHost, dbPort, dbName))

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("Ошибка парсинга DSN: %v", err)
	}

	dbpool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatalf("Ошибка создания пула соединений к БД: %v", err)
	}

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatalf("Не удалось пинговать БД: %v", err)
	}

	log.Println("✅ Успешное подключение к PostgreSQL")
	return dbpool
}

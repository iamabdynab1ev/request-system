package postgresql

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// getEnv считывает переменную окружения или возвращает значение по умолчанию
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func ConnectDB() *pgxpool.Pool {
	// Читаем переменные окружения, которые были загружены в main.go из .env
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

	log.Printf("ℹ️ Попытка подключения к БД: postgresql://%s:***@%s:%d/%s", dbUser, dbHost, dbPort, dbName)

	dbpool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Ошибка создания пула соединений к БД: %v", err)
	}
	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatalf("Не удалось пинговать БД: %v", err)
	}

	log.Println("✅ Успешное подключение к PostgreSQL")
	return dbpool
}

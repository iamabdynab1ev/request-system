// Файл: pkg/database/postgresql/postgresql.go (или connection.go)
package postgresql

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func ConnectDB(dsn string) *pgxpool.Pool {
	log.Printf("ℹ️ Попытка подключения к БД для приложения: %s", dsn) // Логируем реальный DSN

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

	log.Println("✅ Успешное подключение к PostgreSQL для приложения")
	return dbpool
}

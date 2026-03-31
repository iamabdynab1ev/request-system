package postgresql

import (
	"context"
	"log"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func ConnectDB(dsn string) *pgxpool.Pool {
	log.Printf("ℹ️ Попытка подключения к БД для приложения: %s", sanitizeDSNForLog(dsn))

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

func sanitizeDSNForLog(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "[dsn masked]"
	}

	if parsed.User != nil {
		username := parsed.User.Username()
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(username, "***")
		}
	}

	return parsed.String()
}

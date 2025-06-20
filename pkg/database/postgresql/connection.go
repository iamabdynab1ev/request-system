package postgresql

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectDB() *pgxpool.Pool {
	var path = fmt.Sprintf("postgres://%s:%s@%s:%d/%s", "postgres", "postgres", "localhost", 5432, "request-system") //   192.168.56.226

	dbpool, err := pgxpool.New(context.Background(), path)
	if err != nil {
		log.Fatalf("Ошибка создания пула соединений к БД: %v", err)
	}

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatalf("Не удалось пинговать БД: %v", err)
	}

	log.Println("✅ Подключено к PostgreSQL")
	return dbpool
}

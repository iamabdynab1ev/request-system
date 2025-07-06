// Файл: pkg/database/postgresql/postgresql.go
// КОНЕЧНАЯ РАБОЧАЯ ВЕРСИЯ

package postgresql

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ConnectDB() *pgxpool.Pool {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		"postgres",       // Имя пользователя
		"postgres",       // Пароль
		"localhost",      // Хост
		5432,             // Порт
		"request-system", // Имя базы данных
	)

	dbpool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Ошибка создания пула соединений к БД: %v", err)
	}

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatalf("Не удалось пинговать БД: %v", err)
	}

	log.Println("✅ Подключено к PostgreSQL")
	return dbpool
}

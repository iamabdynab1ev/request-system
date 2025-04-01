package postgresql

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DbConn *pgxpool.Pool

func GetDb() *pgxpool.Pool {
	return DbConn
}

func ConnectDB() *pgxpool.Pool {
	// postgres://user:password@localhost:5432/dbname
	var path = fmt.Sprintf("postgres://%s:%s@%s:%d/%s", "postgres", "postgres", "localhost", 5432, "request-system")

	DbConn, err := pgxpool.New(context.Background(), path)
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}

	log.Println("✅ Подключено к PostgreSQL")
	return DbConn
}

package main

import (
	"log"

	"request-system/pkg/config"
	"request-system/pkg/database/postgresql"
	"request-system/seeders"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg := config.New()

	log.Println("Используется DSN для сидера:", cfg.Postgres.DSN)

	dbPool := postgresql.ConnectDB(cfg.Postgres.DSN)
	defer dbPool.Close()

	seeders.SeedAll(dbPool)
}

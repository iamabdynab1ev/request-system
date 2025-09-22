package main

import (
	"log"

	"request-system/pkg/database/postgresql"
	"request-system/seeders"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../../../.env"); err != nil {
		log.Println("Предупреждение: .env файл не найден.")
	}
	dbPool := postgresql.ConnectDB()
	defer dbPool.Close()

	seeders.SeedAll(dbPool)
}

package main

import (
	"flag"
	"log"

	"request-system/pkg/config"
	"request-system/pkg/database/postgresql"
	"request-system/seeders"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	log.Println("======================================================")
	log.Println("       🌱 СИСТЕМА СИДЕРОВ (Наполнение БД)           ")
	log.Println("======================================================")

	// --- Определяем флаги ---
	runCore := flag.Bool("core", false, "Запустить наполнение базовых справочников (статусы, права и т.д.)")
	runRoles := flag.Bool("roles", false, "Запустить создание ролей и Супер-Администратора")
	runAll := flag.Bool("all", false, "Запустить все базовые сидеры (core + roles)")

	flag.Parse()

	// ИСПРАВЛЕНИЕ: Убрали !*runEquipment из проверки ниже
	if !*runCore && !*runRoles && !*runAll {
		log.Println("❌ Не выбран ни один сидер для запуска.")
		log.Println("")
		log.Println("Доступные флаги:")
		flag.PrintDefaults()
		log.Println("")
		log.Println("Примеры использования:")
		log.Println("  go run ./seeders/cmd/seed/main.go -core")
		log.Println("  go run ./seeders/cmd/seed/main.go -roles")
		log.Println("  go run ./seeders/cmd/seed/main.go -all")
		log.Println("======================================================")
		return
	}

	// Подключаемся к БД
	cfg := config.New()
	log.Println("📦 Используется DSN:", cfg.Postgres.DSN)
	dbPool := postgresql.ConnectDB(cfg.Postgres.DSN)
	defer dbPool.Close()

	log.Println("======================================================")

	// Запуск сидеров
	if *runAll || *runCore {
		seeders.SeedCoreDictionaries(dbPool)
		log.Println("======================================================")
	}

	if *runAll || *runRoles {
		seeders.SeedRolesAndAdmin(dbPool, cfg)
		log.Println("======================================================")
	}

	log.Println("✅ Все операции сидирования успешно завершены.")
	log.Println("======================================================")
}

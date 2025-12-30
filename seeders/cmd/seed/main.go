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
	runEquipment := flag.Bool("equipment", false, "Запустить наполнение справочников оборудования")
	runAll := flag.Bool("all", false, "Запустить все сидеры (эквивалентно -core -roles -equipment)")

	flag.Parse()

	// Если ни один флаг не указан - показываем справку
	if !*runCore && !*runRoles && !*runEquipment && !*runAll {
		log.Println("❌ Не выбран ни один сидер для запуска.")
		log.Println("")
		log.Println("Доступные флаги:")
		flag.PrintDefaults()
		log.Println("")
		log.Println("Примеры использования:")
		log.Println("  go run ./seeders/cmd/seed/main.go -core")
		log.Println("  go run ./seeders/cmd/seed/main.go -core -roles")
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

	// Запуск сидеров в правильном порядке
	if *runAll || *runCore {
		seeders.SeedCoreDictionaries(dbPool)
		log.Println("======================================================")
	}


	if *runAll || *runRoles {
		// Роли и админ зависят от базовых справочников
		seeders.SeedRolesAndAdmin(dbPool, cfg)
		log.Println("======================================================")
	}

	log.Println("✅ Все указанные операции сидирования успешно завершены.")
	log.Println("======================================================")
}

// Файл: main.go
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
	// --- Определяем флаги ---
	runCore := flag.Bool("core", false, "Запустить наполнение базовых справочников (статусы, права и т.д.)")
	runRoles := flag.Bool("roles", false, "Запустить создание ролей и Супер-Администратора")
	runEquipment := flag.Bool("equipment", false, "Запустить наполнение справочников оборудования")
	runAll := flag.Bool("all", false, "Запустить все сидеры (эквивалентно -core -roles -equipment)")

	flag.Parse() // <-- Парсим аргументы командной строки

	// --- Проверяем, был ли задан хоть какой-то флаг ---
	if !*runCore && !*runRoles && !*runEquipment && !*runAll {
		log.Println("Не выбран ни один сидер для запуска. Используйте флаги:")
		flag.PrintDefaults() // Печатаем справку
		return
	}

	cfg := config.New()
	log.Println("Используется DSN для сидера:", cfg.Postgres.DSN)
	dbPool := postgresql.ConnectDB(cfg.Postgres.DSN)
	defer dbPool.Close()

	log.Println("======================================================")

	// --- Выполняем сидеры в зависимости от флагов ---
	if *runAll || *runCore {
		seeders.SeedCoreDictionaries(dbPool)
		log.Println("======================================================")
	}

	if *runAll || *runEquipment {
		seeders.SeedEquipmentData(dbPool)
		log.Println("======================================================")
	}

	if *runAll || *runRoles {
		// Роли и админ зависят от базовых справочников, поэтому их лучше запускать после -core
		seeders.SeedRolesAndAdmin(dbPool)
		log.Println("======================================================")
	}

	log.Println("Все указанные операции сидирования успешно завершены.")
}

// Файл: seeders/seeder.go
package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	
	// !!! ДОБАВЬТЕ ЭТУ СТРОКУ !!!
	"request-system/pkg/config" 
)

// SeedCoreDictionaries наполняет самые базовые справочники, не имеющие зависимостей.
func SeedCoreDictionaries(db *pgxpool.Pool) {
	ctx := context.Background()
	log.Println("▶️ Запуск наполнения базовых справочников...")

	if err := seedPermissions(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Прав (Permissions): %v", err)
	}
	if err := seedStatuses(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Статусов (Statuses): %v", err)
	}
	if err := seedPriorities(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Приоритетов (Priorities): %v", err)
	}
	if err := seedOrderTypes(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Типов Заявок (OrderTypes): %v", err)
	}
	log.Println("✅ Наполнение базовых справочников завершено!")
}

// SeedEquipmentData наполняет справочники, связанные с оборудованием.
func SeedEquipmentData(db *pgxpool.Pool) {
	ctx := context.Background()
	log.Println("▶️ Запуск наполнения справочников оборудования...")

	if err := seedEquipmentTypes(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Типов оборудования: %v", err)
	}
	if err := seedEquipments(ctx, db); err != nil {
		log.Printf("ПРЕДУПРЕЖДЕНИЕ: Ошибка наполнения Оборудования: %v", err)
		log.Println("ℹ️ Это может быть нормально, если оргструктура (офисы, филиалы) еще не загружена.")
	}
	log.Println("✅ Наполнение справочников оборудования завершено!")
}

// SeedRolesAndAdmin настраивает роли, их связи и создает суперпользователя.
func SeedRolesAndAdmin(db *pgxpool.Pool, cfg *config.Config) {
	ctx := context.Background()
	log.Println("▶️ Запуск настройки ролей и администратора...")
	
	if err := seedRoles(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Ролей (Roles): %v", err)
	}
	if err := seedRolePermissions(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Связей Ролей и Прав: %v", err)
	}
	
	// Теперь cfg доступен, так как мы импортировали пакет config
	if err := SeedSuperAdmin(db, cfg); err != nil { 
		log.Fatalf("Ошибка создания SuperAdmin: %v", err)
	}

	log.Println("✅ Настройка ролей и администратора завершена!")
}
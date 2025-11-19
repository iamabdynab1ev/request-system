package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func SeedAll(db *pgxpool.Pool) {
	ctx := context.Background()
	log.Println("▶️ Запуск наполнения базы начальными данными...")

	// --- 1. Базовая конфигурация системы (справочники) ---
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

	// --- 2. Справочники по оборудованию (зависят от уже созданных офисов/филиалов) ---
	// ПРИМЕЧАНИЕ: Эти сидеры покажут ПРЕДУПРЕЖДЕНИЯ, если филиалы и офисы еще не были созданы через интеграцию.
	// Это нормально.
	if err := seedEquipmentTypes(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Типов оборудования: %v", err)
	}
	if err := seedEquipments(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Оборудования: %v", err)
	}

	// --- 3. Настройка ролей и их связей ---
	if err := seedRoles(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Ролей (Roles): %v", err)
	}
	if err := seedRolePermissions(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Связей Ролей и Прав: %v", err)
	}

	// --- 4. Создание единственного Супер-Администратора в самом конце ---
	// Он зависит от созданных ролей и статусов.
	if err := seedSuperAdmin(ctx, db); err != nil {
		log.Fatalf("Ошибка создания Супер-Администратора: %v", err)
	}

	// Мы УБРАЛИ вызовы seedBaseDictionaries и seedAdminIB, так как они больше не нужны.

	log.Println("✅ Наполнение базы начальными данными успешно завершено!")
}

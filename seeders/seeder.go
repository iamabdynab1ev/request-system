// Файл: seeders/seeder.go

package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func SeedAll(db *pgxpool.Pool) {
	ctx := context.Background()
	log.Println("▶️ Запуск наполнения базы начальными данными...")

	// ПОРЯДОК ОЧЕНЬ ВАЖЕН!

	if err := seedPermissions(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Прав (Permissions): %v", err)
	}
	if err := seedStatuses(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Статусов (Statuses): %v", err)
	}
	if err := seedPriorities(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Приоритетов (Priorities): %v", err)
	}

	if err := seedBaseDictionaries(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Базовых Справочников: %v", err)
	}

	if err := seedRoles(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Ролей (Roles): %v", err)
	}

	if err := seedAdminIB(ctx, db); err != nil {
		log.Fatalf("Ошибка создания Супер-Администратора: %v", err)
	}
	if err := seedRolePermissions(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Связей Ролей и Прав: %v", err)
	}
	if err := seedPositions(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Должностей (Positions): %v", err)
	}
	if err := seedUsers(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Пользователей (Users): %v", err)
	}
	if err := seedOperationalUsers(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Пользователей Операционного Департамента: %v", err)
	}
	if err := seedITUsers(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Пользователей IT-Департамента: %v", err)
	}
	if err := seedOrderTypes(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Типов Заказов: %v", err)
	}
	log.Println("✅ Наполнение базы начальными данными успешно завершено!")
}

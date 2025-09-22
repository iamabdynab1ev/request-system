package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func SeedAll(db *pgxpool.Pool) {
	ctx := context.Background()
	log.Println("▶️ Запуск наполнения базы начальными данными...")

	if err := seedPermissions(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Прав (Permissions): %v", err)
	}
	if err := seedStatuses(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Статусов (Statuses): %v", err)
	}
	if err := seedPriorities(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Приоритетов (Priorities): %v", err)
	}
	if err := seedRoles(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Ролей (Roles): %v", err)
	}
	if err := seedRolePermissions(ctx, db); err != nil {
		log.Fatalf("Ошибка наполнения Связей Ролей и Прав: %v", err)
	}
	if err := seedSuperAdmin(ctx, db); err != nil {
		log.Fatalf("Ошибка создания Супер-Администратора: %v", err)
	}

	log.Println("✅ Наполнение базы начальными данными успешно завершено!")
}

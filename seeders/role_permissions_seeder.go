package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedRolePermissions(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'role_permissions'...")

	rolePermissions := map[string][]string{
		"Admin":         {"scope:all", "user:create", "user:view", "user:update", "user:delete", "user:password:reset", "role:create", "role:view", "role:update", "role:delete", "permission:view", "position:create", "position:view", "position:update", "position:delete"},
		"Developer":     {"scope:all", "report:view"},
		"User":          {"scope:own", "order:create", "order:view", "order:update", "profile:update", "password:update"},
		"Executor":      {"scope:own", "order:view", "order:update"},
		"Viewing audit": {"scope:all_view"},
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "TRUNCATE TABLE role_permissions RESTART IDENTITY"); err != nil {
		return err
	}

	for roleName, permissions := range rolePermissions {
		var roleID uint64
		err := tx.QueryRow(ctx, "SELECT id FROM roles WHERE name = $1", roleName).Scan(&roleID)
		if err != nil {
			log.Printf("ПРЕДУПРЕЖДЕНИЕ: Роль '%s' не найдена, пропускаем.", roleName)
			continue
		}

		for _, permName := range permissions {
			var permID uint64
			err := tx.QueryRow(ctx, "SELECT id FROM permissions WHERE name = $1", permName).Scan(&permID)
			if err != nil {
				log.Printf("ПРЕДУПРЕЖДЕНИЕ: Привилегия '%s' не найдена, пропускаем.", permName)
				continue
			}

			if _, err := tx.Exec(ctx, "INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)", roleID, permID); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

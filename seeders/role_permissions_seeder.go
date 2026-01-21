package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ▼▼▼ ВАШ ВЫБОР: РЕЖИМ "ТОЛЬКО ДОБАВЛЕНИЕ" ▼▼▼
// false - Ничего НЕ УДАЛЯТЬ, только ДОБАВИТЬ недостающие связи.
// Это безопасно для работающей системы, где права могут меняться вручную.
const fullSync_RolePermissions = false

// getRolePermissionsMap - здесь определяются базовые связи ролей и прав.
// Сидер будет пытаться добавить их, если их еще нет в базе.
func getRolePermissionsMap() map[string][]string {
	return map[string][]string{
		"Офис | Контроль":            {"scope:office", "order:update_in_office_scope", "order:update:executor_id", "order:update:duration"},
		"Филиал | Контроль":          {"scope:branch", "order:update_in_branch_scope", "order:update:executor_id", "order:update:duration"},
		"Создатель":                  {"order:create", "order:create:name", "order:create:address", "order:create:department_id", "order:create:otdel_id", "order:create:branch_id", "order:create:office_id", "order:create:equipment_id", "order:create:equipment_type_id", "order:create:priority_id", "order:create:file", "order:create:comment", "order:create:order_type_id"},
		"Отдел | Контроль":           {"scope:otdel", "order:update_in_otdel_scope", "order:update:executor_id", "order:update:duration"},
		"Базовые привилегии":         {"scope:own", "order:view", "order:update", "order:update:status_id", "order:update:comment", "order:update:file", "user:view", "profile:update", "password:update", "role:view", "permission:view", "status:view", "priority:view", "department:view", "otdel:view", "branch:view", "office:view", "equipment:view", "equipment_type:view", "order_type:view", "position:view", "order_rule:view"},
		"Аудитор":                    {"scope:all_view"},
		"Департамент | Контроль":     {"scope:department", "order:update_in_department_scope", "order:update:executor_id", "order:update:duration"},
		"Администратор справочников": {"status:create", "status:update", "status:delete", "priority:create", "priority:update", "priority:delete", "department:create", "department:update", "department:delete", "otdel:create", "otdel:update", "otdel:delete", "branch:create", "branch:update", "branch:delete", "office:create", "office:update", "office:delete", "equipment:create", "equipment:update", "equipment:delete", "equipment_type:create", "equipment_type:update", "equipment_type:delete", "order_type:create", "order_type:update", "order_type:delete", "position:create", "position:update", "position:delete"},
		"Администратор Системы":      {"scope:all", "role:create", "role:update", "role:delete", "permission:create", "permission:update", "permission:delete", "order_rule:create", "order_rule:update", "order_rule:delete", "integration:view", "integration:sync:run", "integration:update"},
		"Управление доступом":        {"scope:all_view", "user:create", "user:update", "user:delete", "user:password:reset"},
	}
}

func seedRolePermissions(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'role_permissions'...")

	rolePermissions := getRolePermissionsMap()

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// В зависимости от "ключика", либо очищаем таблицу, либо нет.
	if fullSync_RolePermissions {
		log.Println("    - Стратегия: Полная перезапись (TRUNCATE)")
		if _, err := tx.Exec(ctx, "TRUNCATE TABLE role_permissions RESTART IDENTITY"); err != nil {
			return err
		}
	} else {
		log.Println("    - Стратегия: Только добавление новых связей (ADDITIVE)")
	}

	// Этот запрос всегда будет пытаться добавить связь.
	// ON CONFLICT DO NOTHING не даст ему создать дубликат и не вызовет ошибку.
	query := `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`

	for roleName, permissionNames := range rolePermissions {
		var roleID uint64
		err := tx.QueryRow(ctx, "SELECT id FROM roles WHERE name = $1", roleName).Scan(&roleID)
		if err != nil {
			log.Printf("ПРЕДУПРЕЖДЕНИЕ: Роль '%s' не найдена, пропускаем.", roleName)
			continue
		}

		for _, permName := range permissionNames {
			var permID uint64
			err := tx.QueryRow(ctx, "SELECT id FROM permissions WHERE name = $1", permName).Scan(&permID)
			if err != nil {
				log.Printf("ПРЕДУПРЕЖДЕНИЕ: Привилегия '%s' не найдена, пропускаем.", permName)
				continue
			}

			if _, err := tx.Exec(ctx, query, roleID, permID); err != nil {
				// Эта ошибка в теории не должна произойти с "ON CONFLICT", но лучше ее оставить.
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

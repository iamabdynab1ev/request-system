// Файл: seeders/role_permissions.go (ФИНАЛЬНАЯ ВЕРСИЯ)

package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// allViewPermissions - выносим общие права на просмотр в отдельный список, чтобы не дублировать код.
var allViewPermissions = []string{
	"user:view", "role:view", "permission:view", "status:view", "priority:view",
	"department:view", "otdel:view", "branch:view", "office:view",
	"equipment:view", "equipment_type:view", "order_type:view", "position:view",
	"order_rule:view",
}

// rolePermissions - главная конфигурация прав для всех ролей в системе.
var rolePermissions = map[string][]string{
	// --- 1. Admin (ИБ - Управление доступом) ---
	"Admin": append([]string{
		"scope:all", // Может обновлять ЛЮБОГО пользователя и ЛЮБУЮ роль
		"user:create", "user:update", "user:delete", "user:password:reset",
		"role:create", "role:update", "role:delete",
	}, allViewPermissions...),

	// --- 2. Developer (Разработчик системы - Полный доступ к данным) ---
	"Developer": append([]string{
		"order_rule:create", "order_rule:update", "order_rule:delete",
		"scope:all", // Может управлять ВСЕМИ данными
		// Управление должностями
		"position:create", "position:update", "position:delete",

		// Управление ВСЕМИ справочниками
		"status:create", "status:update", "status:delete",
		"priority:create", "priority:update", "priority:delete",
		"department:create", "department:update", "department:delete",
		"otdel:create", "otdel:update", "otdel:delete",
		"branch:create", "branch:update", "branch:delete",
		"office:create", "office:update", "office:delete",
		"equipment:create", "equipment:update", "equipment:delete",
		"equipment_type:create", "equipment_type:update", "equipment_type:delete",
		// Управление ТИПАМИ ЗАЯВОК
		"order_type:create", "order_type:update", "order_type:delete", "rule:view",
		// Управление маршрутами заявок

		// Полное управление Заявками
		"order:create", "order:view", "order:update", "order:delete",
		"order:create:name", "order:create:address", "order:create:department_id", "order:create:otdel_id",
		"order:create:branch_id", "order:create:office_id", "order:create:equipment_id",
		"order:create:equipment_type_id", "order:create:executor_id", "order:create:priority_id",
		"order:create:duration", "order:create:comment",
		"order:update:name", "order:update:address", "order:update:department_id", "order:update:otdel_id",
		"order:update:branch_id", "order:update:office_id", "order:update:equipment_id",
		"order:update:equipment_type_id", "order:update:executor_id", "order:update:status_id",
		"order:update:priority_id", "order:update:duration", "order:update:comment", "order:update:reopen", "order:update:file",
		// Полное управление Ролями
		"role:create", "role:update", "role:delete",
		// Полное управление Пользователями
		"user:create", "user:update", "user:delete", "user:password:reset",

		// Личные права
		"profile:update", "password:update", "report:view",
	}, allViewPermissions...),

	// --- 3. User (Заявитель) ---
	"User": append([]string{
		"scope:own", // Видит только свои заявки
		"order:create", "order:view", "order:update",
		"order:create:name", "order:create:address", "order:create:department_id", "order:create:branch_id",
		"order:create:office_id", "order:create:equipment_id", "order:create:equipment_type_id",
		"order:create:priority_id", "order:create:comment",
		"order:update:comment", "order:update:file", "order:update:status_id",
		"profile:update", "password:update",
		"branch:create", "office:create", "equipment:create", "equipment_type:create",
	}, allViewPermissions...),

	// --- 4. Head of department (Руководитель департамента) ---
	"Head of department": append([]string{
		"scope:own",                        // Видит свои личные заявки
		"scope:department",                 // Видит ВСЕ заявки в своем департаменте
		"order:update_in_department_scope", // Может редактировать ЛЮБУЮ заявку в своем департаменте
		"order:create", "order:view", "order:update",
		"order:create:name", "order:create:address", "order:create:department_id", "order:create:otdel_id",
		"order:create:branch_id", "order:create:office_id", "order:create:equipment_id",
		"order:create:equipment_type_id", "order:create:executor_id", "order:create:priority_id",
		"order:create:duration", "order:create:comment",
		"order:update:name", "order:update:address", "order:update:department_id", "order:update:otdel_id",
		"order:update:branch_id", "order:update:office_id", "order:update:equipment_id",
		"order:update:equipment_type_id", "order:update:executor_id", "order:update:status_id",
		"order:update:priority_id", "order:update:duration", "order:update:comment", "order:update:reopen", "order:update:file",
		"profile:update", "password:update",
	}, allViewPermissions...),

	// --- 5. Executor (Исполнитель) ---
	"Executor": append([]string{
		"scope:own", // Видит заявки, где он создатель или исполнитель
		"order:view", "order:update",
		"order:update:status_id",
		"order:update:comment",
		"order:update:file",
		"profile:update", "password:update",
	}, allViewPermissions...),

	// --- 6. Viewing audit (Ревизор просмотра) ---
	"Viewing audit": append([]string{
		"scope:all_view",
		"order:view",
	}, allViewPermissions...),
}

// seedRolePermissions - функция для наполнения связей между ролями и привилегиями.
func seedRolePermissions(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'role_permissions'...")
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
			_, err = tx.Exec(ctx, "INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)", roleID, permID)
			if err != nil {
				if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
					log.Printf("ПРЕДУПРЕЖДЕНИЕ: Дубликат права '%s' для роли '%s'. Проверьте сидер.", permName, roleName)
					continue
				}
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

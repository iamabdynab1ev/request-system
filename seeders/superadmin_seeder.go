// Файл: seeders/superadmin_seeder.go
package seeders

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"request-system/pkg/utils"
)

func seedSuperAdmin(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Создание пользователя 'SuperAdmin' с назначением ВСЕХ ролей...")

	email := "superadmin@example.com"
	password := "password"

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID uint64
	err = tx.QueryRow(ctx, "SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL", email).Scan(&userID)
	if err == nil {
		log.Println("    - Пользователь 'SuperAdmin' уже существует. Обновляем его роли...")
	} else {
		var statusID uint64
		if err := tx.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE'").Scan(&statusID); err != nil {
			return fmt.Errorf("не удалось найти статус 'ACTIVE'")
		}

		hashedPassword, err := utils.HashPassword(password)
		if err != nil {
			return fmt.Errorf("ошибка хеширования пароля: %w", err)
		}

		query := `INSERT INTO users (fio, email, phone_number, password, status_id, must_change_password)
				  VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
		err = tx.QueryRow(ctx, query,
			"Super Admin", email, "992000000000", hashedPassword, statusID, false,
		).Scan(&userID)
		if err != nil {
			return fmt.Errorf("ошибка при создании пользователя 'SuperAdmin': %w", err)
		}
		log.Printf("    - Пользователь 'SuperAdmin' успешно создан с email: %s и паролем: %s", email, password)
	}

	rows, err := tx.Query(ctx, "SELECT id FROM roles")
	if err != nil {
		return fmt.Errorf("ошибка получения списка ролей: %w", err)
	}

	var allRoleIDs []uint64
	for rows.Next() {
		var roleID uint64
		if err := rows.Scan(&roleID); err != nil {
			rows.Close()
			return err
		}
		allRoleIDs = append(allRoleIDs, roleID)
	}
	rows.Close()

	var assignedRolesCount int
	for _, roleID := range allRoleIDs {
		queryUserRole := `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
		tag, err := tx.Exec(ctx, queryUserRole, userID, roleID)
		if err != nil {
			return fmt.Errorf("ошибка при привязке роли (ID=%d) к 'SuperAdmin': %w", roleID, err)
		}
		if tag.RowsAffected() > 0 {
			assignedRolesCount++
		}
	}

	log.Printf("    - Пользователю 'SuperAdmin' назначено/подтверждено %d новых ролей.", assignedRolesCount)
	return tx.Commit(ctx)
}

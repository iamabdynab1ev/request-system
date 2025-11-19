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

	// --- ШАГ 1: Создаем (или находим) самого пользователя ---
	var userID uint64
	err = tx.QueryRow(ctx, "SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL", email).Scan(&userID)
	if err == nil {
		log.Println("    - Пользователь 'SuperAdmin' уже существует. Обновляем его роли...")
	} else {
		// Пользователя нет, создаем его
		var statusID uint64
		if err := tx.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE'").Scan(&statusID); err != nil {
			return fmt.Errorf("не удалось найти статус 'ACTIVE'")
		}

		hashedPassword, err := utils.HashPassword(password)
		if err != nil {
			return fmt.Errorf("ошибка хеширования пароля: %w", err)
		}

		// Создаем пользователя с минимальным набором полей
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

	// --- ШАГ 2: Находим ID ВСЕХ существующих ролей ---
	rows, err := tx.Query(ctx, "SELECT id FROM roles")
	if err != nil {
		return fmt.Errorf("ошибка получения списка ролей: %w", err)
	}
	// Убираем defer отсюда!
	// defer rows.Close()

	// --- ШАГ 3: Привязываем каждую роль к пользователю ---
	var assignedRolesCount int
	for rows.Next() {
		var roleID uint64
		if err := rows.Scan(&roleID); err != nil {
			rows.Close() // <-- Закрываем rows в случае ошибки!
			return err
		}

		queryUserRole := `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
		tag, err := tx.Exec(ctx, queryUserRole, userID, roleID)
		if err != nil {
			rows.Close() // <-- И здесь тоже
			return fmt.Errorf("ошибка при привязке роли (ID=%d) к 'SuperAdmin': %w", roleID, err)
		}
		if tag.RowsAffected() > 0 {
			assignedRolesCount++
		}
	}

	rows.Close()

	log.Printf("    - Пользователю 'SuperAdmin' назначено/подтверждено %d ролей.", assignedRolesCount)
	return tx.Commit(ctx)
}

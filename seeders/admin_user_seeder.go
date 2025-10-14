// Файл: seeders/admin_user_seeder.go

package seeders

import (
	"context"
	"errors"
	"fmt"
	"log"

	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func seedAdminIB(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Создание пользователя 'Admin (ИБ)'...")

	email := "admin-ib@arvand.tj"

	// Шаг 1: Проверяем, существует ли пользователь
	var existingUserID uint64
	err := db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL", email).Scan(&existingUserID)
	if err == nil {
		log.Println("    - Пользователь Admin (ИБ) уже существует. Пропускаем.")
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("ошибка при проверке существования пользователя: %w", err)
	}

	// Шаг 2: Получаем все необходимые ID из справочников

	var statusID uint64
	err = db.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE'").Scan(&statusID)
	if err != nil {
		return fmt.Errorf("не найден статус 'ACTIVE' (code='ACTIVE'): %w", err)
	}

	var roleID uint64
	err = db.QueryRow(ctx, "SELECT id FROM roles WHERE name = 'Admin'").Scan(&roleID)
	if err != nil {
		return fmt.Errorf("не найдена роль 'Admin': %w", err)
	}

	// ---> НАШЕ ГЛАВНОЕ ИЗМЕНЕНИЕ <---
	// Находим ID должности "Администратор ИБ" по ее системному коду
	var positionID uint64
	err = db.QueryRow(ctx, "SELECT id FROM positions WHERE code = 'SECURITY_ADMIN'").Scan(&positionID)
	if err != nil {
		return fmt.Errorf("не найдена должность с кодом 'SECURITY_ADMIN'. Запустите сначала сидер для должностей (positions_seeder). %w", err)
	}
	// ---> КОНЕЦ ИЗМЕНЕНИЯ <---

	// Используем те же branch_id и department_id, что были у вас
	branchID := uint64(1)     // Предполагаем, что базовый филиал имеет ID=1
	departmentID := uint64(1) // Предполагаем, что базовый департамент имеет ID=1

	// Шаг 3: Готовим данные для создания пользователя

	phoneNumber := "992999999999"
	hashedPassword, err := utils.HashPassword(phoneNumber)
	if err != nil {
		return fmt.Errorf("ошибка хеширования пароля: %w", err)
	}

	// Шаг 4: Создаем пользователя с position_id

	var createdUserID uint64

	// ---> ИЗМЕНЕННЫЙ SQL ЗАПРОС <---
	query := `
		INSERT INTO users (
			fio, email, phone_number, password, status_id, branch_id, 
			department_id, is_head, must_change_password, position_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) 
		RETURNING id
	`
	err = db.QueryRow(ctx, query,
		"Администратор ИБ", // fio
		email,              // email
		phoneNumber,        // phone_number
		hashedPassword,     // password
		statusID,           // status_id
		branchID,           // branch_id
		departmentID,       // department_id
		false,              // is_head
		true,               // must_change_password
		positionID,         // position_id <--- НАШЕ ГЛАВНОЕ ИЗМЕНЕНИЕ
	).Scan(&createdUserID)
	if err != nil {
		return fmt.Errorf("ошибка при создании пользователя 'Admin (ИБ)': %w", err)
	}

	// Шаг 5: Привязываем роль к созданному пользователю
	queryUserRole := `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	if _, err := db.Exec(ctx, queryUserRole, createdUserID, roleID); err != nil {
		return fmt.Errorf("ошибка при привязке роли 'Admin' к пользователю: %w", err)
	}

	log.Println("    - Пользователь Admin (ИБ) успешно создан и ему назначена роль.")
	return nil
}

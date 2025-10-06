package seeders

import (
	"context"
	"fmt"
	"log"

	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Вынесем эту функцию в отдельный файл, например, seeders/main.go
// func SeedAll(db *pgxpool.Pool) { ... }

// seedUsers создает двух пользователей: Admin (ИБ) и Тестовый Сотрудник
func seedUsers(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Создание пользователей...")

	var statusID, branchID, departmentID uint64

	// Получаем необходимые ID из справочников
	err := db.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE' AND type = 2 LIMIT 1").Scan(&statusID)
	if err != nil {
		return fmt.Errorf("не найден статус 'ACTIVE' для пользователя: %w", err)
	}
	err = db.QueryRow(ctx, "SELECT id FROM branches LIMIT 1").Scan(&branchID)
	if err != nil {
		return fmt.Errorf("не найден ни один филиал в справочнике: %w", err)
	}
	err = db.QueryRow(ctx, "SELECT id FROM departments LIMIT 1").Scan(&departmentID)
	if err != nil {
		return fmt.Errorf("не найден ни один департамент в справочнике: %w", err)
	}

	// --- 1. Создание "Admin (ИБ)" ---
	emailAdmin := "admin-ib@arvand.tj"
	var adminID uint64
	var adminRoleID uint64 = 1 // Предполагаем, что у роли "Admin" ID=1

	err = db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", emailAdmin).Scan(&adminID)
	if err == nil {
		log.Println("    - Пользователь Admin (ИБ) уже существует.")
	} else {
		hashedPassword, _ := utils.HashPassword("992999999999")
		query := `INSERT INTO users (fio, "position", email, phone_number, password, status_id, branch_id, department_id, is_head, must_change_password)
				  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`

		err = db.QueryRow(ctx, query,
			"Администратор ИБ", "Администратор Информационной Безопасности",
			emailAdmin, "992999999999", hashedPassword,
			statusID, branchID, departmentID, false, false,
		).Scan(&adminID)
		if err != nil {
			return fmt.Errorf("ошибка создания пользователя Admin: %w", err)
		}
		log.Println("    - Пользователь Admin (ИБ) успешно создан.")
	}
	// Привязываем роль к Admin (ИБ)
	_, err = db.Exec(ctx, "INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", adminID, adminRoleID)
	if err != nil {
		return fmt.Errorf("ошибка привязки роли к Admin: %w", err)
	}
	log.Println("    - Роль 'Admin' назначена пользователю Admin (ИБ).")

	// --- 2. Создание "Тестовый Сотрудник" ---
	emailEmployee := "test.employee@example.com"
	var employeeID uint64
	employeeRoleIDs := []uint64{2, 3} // Роли "Сотрудник" и "Наблюдатель"

	err = db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", emailEmployee).Scan(&employeeID)
	if err == nil {
		log.Println("    - Пользователь Тестовый Сотрудник уже существует.")
	} else {
		hashedPassword, _ := utils.HashPassword("111222333")
		query := `INSERT INTO users (fio, email, phone_number, password, position, status_id, branch_id, department_id, must_change_password) 
				  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, true) RETURNING id`
		err = db.QueryRow(ctx, query,
			"Тестовый Сотрудник", emailEmployee, "111222333",
			hashedPassword, "Тестировщик", statusID, branchID, departmentID,
		).Scan(&employeeID)
		if err != nil {
			return fmt.Errorf("ошибка создания тестового сотрудника: %w", err)
		}
		log.Println("    - Пользователь Тестовый Сотрудник успешно создан.")
	}
	// Привязываем роли к Тестовому Сотруднику
	for _, roleID := range employeeRoleIDs {
		_, err = db.Exec(ctx, "INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", employeeID, roleID)
		if err != nil {
			return fmt.Errorf("ошибка привязки роли %d к тестовому сотруднику: %w", roleID, err)
		}
	}
	log.Printf("    - Тестовому сотруднику назначены роли: %v\n", employeeRoleIDs)

	return nil
}

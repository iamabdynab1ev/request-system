package seeders

import (
	"context"
	"fmt"
	"log"

	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedSuperAdmin(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Создание пользователя 'Super Admin'...")
	email := "super.admin.test@gmail.com"
	var exists bool
	err := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		log.Println("    - Пользователь Super Admin уже существует. Пропускаем.")
		return nil
	}

	var statusID, roleID, branchID, departmentID uint64
	err = db.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE' and type = 2 LIMIT 1").Scan(&statusID)
	if err != nil {
		return fmt.Errorf("не найден статус 'ACTIVE' для пользователя: %w", err)
	}
	err = db.QueryRow(ctx, "SELECT id FROM roles WHERE name = 'Super Admin' LIMIT 1").Scan(&roleID)
	if err != nil {
		return fmt.Errorf("не найдена роль 'Super Admin': %w", err)
	}
	err = db.QueryRow(ctx, "SELECT id FROM branches LIMIT 1").Scan(&branchID)
	if err != nil {
		if err := db.QueryRow(ctx, "INSERT INTO branches (name, short_name, address, phone_number, email, open_date, status_id) VALUES ('Головной офис', 'ГО', 'г.Душанбе', '992000000000', 'ho@bank.tj', NOW(), $1) RETURNING id", statusID).Scan(&branchID); err != nil {
			return err
		}
	}
	err = db.QueryRow(ctx, "SELECT id FROM departments LIMIT 1").Scan(&departmentID)
	if err != nil {
		if err := db.QueryRow(ctx, "INSERT INTO departments (name, status_id) VALUES ('IT-департамент', $1) RETURNING id", statusID).Scan(&departmentID); err != nil {
			return err
		}
	}

	hashedPassword, err := utils.HashPassword("Password123!")
	if err != nil {
		return err
	}

	query := `INSERT INTO users (fio, "position", email, phone_number, password, status_id, role_id, branch_id, department_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	if _, err := db.Exec(ctx, query, "Супер Админ Тестовый", "Главный администратор", email, "921111111", hashedPassword, statusID, roleID, branchID, departmentID); err != nil {
		return err
	}
	return nil
}

package seeders

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"request-system/pkg/utils"
)

func seedAdminIB(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Создание пользователя 'Admin (ИБ)'...")

	email := "admin-ib@arvand.tj"
	var existingUserID uint64
	err := db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL", email).Scan(&existingUserID)
	if err == nil {
		log.Println("    - Пользователь Admin (ИБ) уже существует. Пропускаем.")
		return nil
	}

	var statusID, roleID, positionID, branchID, departmentID uint64

	db.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE'").Scan(&statusID)
	db.QueryRow(ctx, "SELECT id FROM roles WHERE name = 'Admin'").Scan(&roleID)
	db.QueryRow(ctx, "SELECT id FROM positions WHERE name = 'Администратор ИБ'").Scan(&positionID)
	db.QueryRow(ctx, "SELECT id FROM branches WHERE name = 'Головной офис'").Scan(&branchID)
	db.QueryRow(ctx, "SELECT id FROM departments WHERE name = 'Департамент Информационной Безопасности'").Scan(&departmentID)

	if statusID == 0 || roleID == 0 || positionID == 0 || branchID == 0 || departmentID == 0 {
		return fmt.Errorf("не удалось найти все необходимые ID из справочников для создания Admin")
	}

	phoneNumber := "992999999999"
	hashedPassword, err := utils.HashPassword(phoneNumber)
	if err != nil {
		return fmt.Errorf("ошибка хеширования пароля: %w", err)
	}

	var createdUserID uint64
	query := `INSERT INTO users (fio, email, phone_number, password, status_id, branch_id, department_id, is_head, must_change_password, position_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`
	err = db.QueryRow(ctx, query, "Администратор ИБ", email, phoneNumber, hashedPassword, statusID, branchID, departmentID, false, true, positionID).Scan(&createdUserID)
	if err != nil {
		return fmt.Errorf("ошибка при создании пользователя 'Admin (ИБ)': %w", err)
	}

	queryUserRole := `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	if _, err := db.Exec(ctx, queryUserRole, createdUserID, roleID); err != nil {
		return fmt.Errorf("ошибка при привязке роли 'Admin' к пользователю: %w", err)
	}

	log.Println("    - Пользователь Admin (ИБ) успешно создан и ему назначена роль.")
	return nil
}

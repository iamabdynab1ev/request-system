// Файл: seeders/admin_user.go
package seeders

import (
	"context"
	"fmt"
	"log"

	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedAdminIB(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Создание пользователя 'Admin (ИБ)'...")

	email := "admin-ib@arvand.tj" // Уникальный email для администратора ИБ
	var userID uint64             // Переменная для хранения ID созданного пользователя
	err := db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err == nil { // Пользователь уже существует
		log.Println("    - Пользователь Admin (ИБ) уже существует. Пропускаем.")
		return nil
	}
	// Если err != nil и это не ErrNoRows, то это настоящая ошибка
	if err != nil && err.Error() != "no rows in result set" {
		return fmt.Errorf("ошибка при проверке существования пользователя: %w", err)
	}

	var statusID, roleID, branchID, departmentID uint64

	// Находим статус "Активный" для пользователя
	err = db.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE' AND type = 2 LIMIT 1").Scan(&statusID)
	if err != nil {
		return fmt.Errorf("не найден статус 'ACTIVE' для пользователя: %w", err)
	}

	// Находим роль "Admin" (это наша роль ИБ)
	err = db.QueryRow(ctx, "SELECT id FROM roles WHERE name = 'Admin' LIMIT 1").Scan(&roleID)
	if err != nil {
		return fmt.Errorf("не найдена роль 'Admin': %w", err)
	}

	// Пытаемся найти или создать базовые сущности
	err = db.QueryRow(ctx, "SELECT id FROM branches LIMIT 1").Scan(&branchID)
	if err != nil {
		_, err = db.Exec(ctx, "INSERT INTO branches (name, short_name, status_id) VALUES ('Головной офис', 'ГО', $1) ON CONFLICT (name) DO NOTHING", statusID)
		if err != nil {
			return fmt.Errorf("не удалось вставить базовый филиал: %w", err)
		}
		err = db.QueryRow(ctx, "SELECT id FROM branches WHERE name = 'Головной офис' LIMIT 1").Scan(&branchID)
		if err != nil {
			return fmt.Errorf("не удалось найти базовый филиал после вставки: %w", err)
		}
	}

	err = db.QueryRow(ctx, "SELECT id FROM departments LIMIT 1").Scan(&departmentID)
	if err != nil {
		_, err = db.Exec(ctx, "INSERT INTO departments (name, status_id) VALUES ('Департамент Информационной Безопасности', $1) ON CONFLICT (name) DO NOTHING", statusID)
		if err != nil {
			return fmt.Errorf("не удалось вставить базовый департамент: %w", err)
		}
		err = db.QueryRow(ctx, "SELECT id FROM departments WHERE name = 'Департамент Информационной Безопасности' LIMIT 1").Scan(&departmentID)
		if err != nil {
			return fmt.Errorf("не удалось найти базовый департамент после вставки: %w", err)
		}
	}

	phoneNumber := "992999999999"

	hashedPassword, err := utils.HashPassword(phoneNumber)
	if err != nil {
		return err
	}

	// --- ИЗМЕНЕНО: Удален 'role_id' из INSERT запроса в users ---
	query := `INSERT INTO users (fio, "position", email, phone_number, password, status_id, branch_id, department_id, is_head, must_change_password)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id` // Добавлено RETURNING id

	// Теперь мы получаем ID созданного пользователя
	err = db.QueryRow(ctx, query,
		"Администратор ИБ",
		"Администратор Информационной Безопасности",
		email,
		phoneNumber,
		hashedPassword,
		statusID,
		branchID,
		departmentID,
		false,
		true,
	).Scan(&userID) // СКАНИРУЕМ ID СОЗДАННОГО ПОЛЬЗОВАТЕЛЯ
	if err != nil {
		return fmt.Errorf("ошибка при создании пользователя: %w", err)
	}

	// --- НОВОЕ: Вставляем запись в таблицу user_roles ---
	queryUserRole := `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	if _, err := db.Exec(ctx, queryUserRole, userID, roleID); err != nil {
		return fmt.Errorf("ошибка при привязке роли к пользователю: %w", err)
	}

	log.Println("    - Пользователь Admin (ИБ) успешно создан и ему назначена роль.")
	return nil
}

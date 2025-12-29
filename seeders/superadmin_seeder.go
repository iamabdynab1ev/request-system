package seeders

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"request-system/pkg/config"
	"request-system/pkg/utils"
)

func SeedSuperAdmin(db *pgxpool.Pool, cfg *config.Config) error {
	ctx := context.Background()
	log.Println("  - Запуск сидера ТЕСТОВОГО АДМИНИСТРАТОРА (Bypass LDAP)...")

	// 1. Берем данные из конфига (.env)
	email := cfg.Seeder.AdminEmail       // admin_test@helpdesk.tj
	password := cfg.Seeder.AdminPassword // TestPass12345!

	if email == "" || password == "" {
		log.Println("    [SKIP] Данные SEED_ADMIN не заданы в .env. Пропускаем.")
		return nil
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 2. Проверяем, существует ли он уже по email
	var userID uint64
	err = tx.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", email).Scan(&userID)

	if err == nil {
		log.Println("    - Пользователь уже существует. Обновление не требуется.")
		return tx.Commit(ctx)
	}

	// 3. Получаем ID статуса 'ACTIVE'
	var statusID uint64
	err = tx.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE'").Scan(&statusID)
	if err != nil {
		return fmt.Errorf("сначала запустите наполнение статусов (-core)")
	}

	// 4. Хешируем пароль
	hashedPassword, _ := utils.HashPassword(password)

	// 5. Вставляем запись.
	// ВАЖНО: username ставим 'admin_test' (можно взять часть email)
	query := `
		INSERT INTO users (
			fio, email, phone_number, password,
			status_id, must_change_password, source_system, username
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	err = tx.QueryRow(ctx, query,
		"Test Administrator",
		email,
		"992-000-TEST", // Заглушка номера
		hashedPassword,
		statusID,
		true,      // Требуем сменить пароль при входе
		"LOCAL",   // Пометка для БД, что он локальный
		"admin_test", // Тот самый логин для входа
	).Scan(&userID)

	if err != nil {
		return fmt.Errorf("ошибка SQL при создании admin_test: %w", err)
	}

	// 6. Даем права (роли)
	roles := []string{"Базовые привилегии", "Управление доступом", "Администратор Системы"}
	for _, rName := range roles {
		_, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id FROM roles WHERE name = $2
			ON CONFLICT DO NOTHING
		`, userID, rName)
		if err != nil {
			log.Printf("      [!] Не удалось выдать роль %s: %v", rName, err)
		}
	}

	log.Printf("    ✅ УСПЕХ: Пользователь %s создан. Логин для входа: admin_test", email)
	return tx.Commit(ctx)
}

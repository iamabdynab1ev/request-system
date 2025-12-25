package seeders

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"request-system/pkg/config" // <--- Важно
	"request-system/pkg/utils"
)

// Теперь принимаем еще и конфиг
func SeedSuperAdmin(db *pgxpool.Pool, cfg *config.Config) error {
	ctx := context.Background()
	log.Println("  - Запуск сидера SuperAdmin...")

	email := cfg.Seeder.AdminEmail
	password := cfg.Seeder.AdminPassword

	if email == "" || password == "" {
		log.Println("    [INFO] SEED_ADMIN_EMAIL не задан. Пропускаем создание.")
		return nil
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID uint64
	// Используем email для поиска существующего админа
	err = tx.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", email).Scan(&userID)

	if err == nil {
		log.Println("    - Root пользователь уже существует. Не трогаем.")
		return tx.Commit(ctx) 
	} 

	log.Println("    - Создаем нового Root пользователя...")

	var statusID uint64
	if err := tx.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE'").Scan(&statusID); err != nil {
		return fmt.Errorf("статус ACTIVE не найден. Запустите сначала Core seeders")
	}

	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return err
	}

	// ИСПРАВЛЕННЫЙ ЗАПРОС: заменяем 'login' на 'username'
	query := `
		INSERT INTO users (
			fio, email, phone_number, password, 
			status_id, must_change_password, source_system, username
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) 
		RETURNING id
	`
	// Записываем "root" в колонку username
	err = tx.QueryRow(ctx, query,
		"System Administrator", email, "LOCAL-ROOT", hashedPassword, 
		statusID, true, "LOCAL", "root", 
	).Scan(&userID)

	if err != nil {
		return fmt.Errorf("ошибка SQL при создании Root: %w", err)
	}

	// Выдача ролей (оставляем как было)
	roleNames := []string{"Базовые привилегии", "Управление доступом"}
	for _, rName := range roleNames {
		_, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id) 
			SELECT $1, id FROM roles WHERE name = $2
			ON CONFLICT DO NOTHING
		`, userID, rName)
		if err != nil {
			log.Printf("Не удалось выдать роль %s: %v", rName, err)
		}
	}

	log.Printf("    - Пользователь %s успешно создан (Username: root, Source: LOCAL)", email)
	return tx.Commit(ctx)
}
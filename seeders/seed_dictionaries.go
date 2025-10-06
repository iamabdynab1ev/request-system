// Файл: seeders/seed_dictionaries.go

package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedBaseDictionaries наполняет базовые справочники, от которых зависят другие сидеры.
func seedBaseDictionaries(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение базовых справочников (филиалы, департаменты)...")

	// Сначала получаем ID статуса "Активный", он нужен для обоих справочников
	var activeStatusID uint64
	err := db.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE' and type = 2 LIMIT 1").Scan(&activeStatusID)
	if err != nil {
		// Если не можем найти статус, значит, сидер статусов еще не отработал, это критическая ошибка
		return err
	}

	// 1. Создаем Филиал по умолчанию
	branchQuery := `
    INSERT INTO branches (name, short_name, status_id) 
    VALUES ('Головной офис', 'ГО', $1) 
    ON CONFLICT (name) DO NOTHING;`

	if _, err := db.Exec(ctx, branchQuery, activeStatusID); err != nil {
		return err
	}

	// 2. Создаем Департамент по умолчанию
	departmentQuery := `
		INSERT INTO departments (name, status_id) 
		VALUES ('Департамент Информационной Безопасности', $1) 
		ON CONFLICT (name) DO NOTHING;`

	if _, err := db.Exec(ctx, departmentQuery, activeStatusID); err != nil {
		return err
	}

	log.Println("    - Базовые справочники успешно проверены/созданы.")
	return nil
}

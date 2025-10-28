package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedBaseDictionaries(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение базовых справочников (филиалы, департаменты)...")
	var activeStatusID uint64
	err := db.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE' and type = 2 LIMIT 1").Scan(&activeStatusID)
	if err != nil {
		return err
	}

	branchQuery := `INSERT INTO branches (name, short_name, status_id) VALUES ('Головной офис', 'ГО', $1) ON CONFLICT (name) DO NOTHING;`
	if _, err := db.Exec(ctx, branchQuery, activeStatusID); err != nil {
		return err
	}

	departmentQuery := `INSERT INTO departments (name, status_id) VALUES ('Департамент Информационной Безопасности', $1) ON CONFLICT (name) DO NOTHING;`
	if _, err := db.Exec(ctx, departmentQuery, activeStatusID); err != nil {
		return err
	}

	log.Println("    - Базовые справочники успешно проверены/созданы.")
	return nil
}

package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// КЛЮЧИК: true - полностью очистить таблицу и записать типы с нуля.
// false - только добавить новые типы, не трогая существующие.
const fullSync_OrderTypes = false

func seedOrderTypes(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'order_types'...")

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if fullSync_OrderTypes {
		log.Println("    - Стратегия: Полная перезапись (TRUNCATE)")
		if _, err := tx.Exec(ctx, "TRUNCATE TABLE order_types RESTART IDENTITY CASCADE"); err != nil {
			return err
		}
	} else {
		log.Println("    - Стратегия: Только добавление новых типов (ADDITIVE)")
	}

	var activeStatusID uint64
	err = tx.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE'").Scan(&activeStatusID)
	if err != nil {
		return err
	}

	query := `INSERT INTO order_types (name, code, status_id) VALUES ($1, $2, $3) 
			  ON CONFLICT (code) DO NOTHING`

	for _, ot := range orderTypesData {
		if _, err := tx.Exec(ctx, query, ot.Name, ot.Code, activeStatusID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

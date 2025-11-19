package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// КЛЮЧИК: true - полностью очистить таблицу и записать типы с нуля.
// false - только добавить новые типы, не трогая существующие.
const fullSync_EquipmentTypes = false

func seedEquipmentTypes(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'equipment_types'...")

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if fullSync_EquipmentTypes {
		log.Println("    - Стратегия: Полная перезапись (TRUNCATE)")
		if _, err := tx.Exec(ctx, "TRUNCATE TABLE equipment_types RESTART IDENTITY CASCADE"); err != nil {
			return err
		}
	} else {
		log.Println("    - Стратегия: Только добавление новых типов (ADDITIVE)")
	}

	query := `INSERT INTO equipment_types (name) VALUES ($1) 
			  ON CONFLICT (name) DO NOTHING`

	for _, name := range equipmentTypesData {
		if _, err := tx.Exec(ctx, query, name); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

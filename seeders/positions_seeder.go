package seeders

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedPositions(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'positions'...")
	query := `INSERT INTO positions (name, "type", status_id) VALUES ($1, $2, $3) ON CONFLICT (name) DO UPDATE SET "type" = EXCLUDED.type, status_id = EXCLUDED.status_id;`
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, pos := range positionsData {
		if _, err := tx.Exec(ctx, query, pos.Name, pos.Type, pos.StatusID); err != nil {
			return fmt.Errorf("ошибка при вставке должности '%s': %w", pos.Name, err)
		}
	}
	return tx.Commit(ctx)
}

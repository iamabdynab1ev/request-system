package seeders

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedPositions(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'positions'...")

	// Используем `ON CONFLICT` по полю `code`, чтобы можно было безопасно перезапускать сидер
	// Он обновит `name` и `level`, если должность с таким кодом уже существует
	query := `
		INSERT INTO positions (name, code, level, status_id) 
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (code) DO UPDATE 
		SET name = EXCLUDED.name, 
		    level = EXCLUDED.level,
		    status_id = EXCLUDED.status_id,
		    updated_at = NOW();
	`
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, pos := range positionsData {
		if _, err := tx.Exec(ctx, query, pos.Name, pos.Code, pos.Level, pos.StatusID); err != nil {
			return fmt.Errorf("ошибка при вставке должности '%s': %w", pos.Name, err)
		}
	}

	return tx.Commit(ctx)
}

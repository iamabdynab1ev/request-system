package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedOrderTypes(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'order_types'...")
	query := `INSERT INTO order_types (name, code, status_id) VALUES ($1, $2, $3) ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, status_id = EXCLUDED.status_id;`
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, ot := range ordertypesData {
		if _, err := tx.Exec(ctx, query, ot.Name, ot.Code, ot.StatusID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

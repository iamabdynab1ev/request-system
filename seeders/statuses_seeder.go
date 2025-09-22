package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedStatuses(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'statuses'...")
	query := `INSERT INTO statuses (name, type, code) VALUES ($1, $2, $3) ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, type = EXCLUDED.type;`
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, s := range statusesData {
		if _, err := tx.Exec(ctx, query, s.Name, s.Type, s.Code); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedRoles(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'roles'...")
	var activeStatusID uint64
	err := db.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE' and type = 2 LIMIT 1").Scan(&activeStatusID)
	if err != nil {
		return err
	}

	query := `INSERT INTO roles (name, description, status_id) VALUES ($1, $2, $3) ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description;`
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, r := range rolesData {
		if _, err := tx.Exec(ctx, query, r.Name, r.Description, activeStatusID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

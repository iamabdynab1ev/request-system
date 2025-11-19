package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// КЛЮЧИК: true - обновить описание, если привилегия с таким NAME уже существует.
// false - пропустить, если привилегия уже существует.
const updateIfExists_Permissions = false

func seedPermissions(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'permissions'...")

	var query string
	if updateIfExists_Permissions {
		query = `INSERT INTO permissions (name, description) VALUES ($1, $2) 
				 ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description;`
		log.Println("    - Стратегия: Обновление существующих прав (UPSERT)")
	} else {
		query = `INSERT INTO permissions (name, description) VALUES ($1, $2) 
				 ON CONFLICT (name) DO NOTHING;`
		log.Println("    - Стратегия: Пропуск существующих прав (IGNORE)")
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, p := range permissionsData {
		if _, err := tx.Exec(ctx, query, p.Name, p.Description); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

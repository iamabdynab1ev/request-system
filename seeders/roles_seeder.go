package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// КЛЮЧИК: true - обновить описание и статус, если роль с таким NAME уже существует.
// false - пропустить, если роль уже существует.
const updateIfExists_Roles = false

func seedRoles(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'roles'...")

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var activeStatusID uint64
	err = tx.QueryRow(ctx, "SELECT id FROM statuses WHERE code = 'ACTIVE'").Scan(&activeStatusID)
	if err != nil {
		return err // Не можем продолжать без статуса 'ACTIVE'
	}

	var query string
	if updateIfExists_Roles {
		query = `INSERT INTO roles (name, description, status_id) VALUES ($1, $2, $3) 
				 ON CONFLICT (name) DO UPDATE SET description = EXCLUDED.description, status_id = EXCLUDED.status_id;`
		log.Println("    - Стратегия: Обновление существующих ролей (UPSERT)")
	} else {
		query = `INSERT INTO roles (name, description, status_id) VALUES ($1, $2, $3) 
				 ON CONFLICT (name) DO NOTHING;`
		log.Println("    - Стратегия: Пропуск существующих ролей (IGNORE)")
	}

	for _, r := range rolesData {
		if _, err := tx.Exec(ctx, query, r.Name, r.Description, activeStatusID); err != nil {
			log.Printf("Ошибка при вставке/обновлении роли '%s': %v", r.Name, err)
			return err
		}
	}

	return tx.Commit(ctx)
}

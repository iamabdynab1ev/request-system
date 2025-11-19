package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// КЛЮЧИК: true - обновить имя/тип, если статус с таким CODE уже существует.
// false - пропустить (ничего не делать), если статус с таким CODE уже существует.
const updateIfExists_Statuses = false

func seedStatuses(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'statuses'...")

	var query string
	if updateIfExists_Statuses {
		query = `INSERT INTO statuses (name, type, code) VALUES ($1, $2, $3) 
				 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, type = EXCLUDED.type;`
		log.Println("    - Стратегия: Обновление существующих статусов (UPSERT)")
	} else {
		query = `INSERT INTO statuses (name, type, code) VALUES ($1, $2, $3) 
				 ON CONFLICT (code) DO NOTHING;`
		log.Println("    - Стратегия: Пропуск существующих статусов (IGNORE)")
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, s := range statusesData {
		if _, err := tx.Exec(ctx, query, s.Name, s.Type, s.Code); err != nil {
			log.Printf("Ошибка при вставке/обновлении статуса '%s': %v", s.Name, err)
			return err
		}
	}

	return tx.Commit(ctx)
}

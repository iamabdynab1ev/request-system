package seeders

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// КЛЮЧИК: true - обновить имя/rate, если приоритет с таким CODE уже существует.
// false - пропустить, если приоритет с таким CODE уже существует.
const updateIfExists_Priorities = false

func seedPriorities(ctx context.Context, db *pgxpool.Pool) error {
	log.Println("  - Наполнение таблицы 'priorities'...")

	var query string
	if updateIfExists_Priorities {
		query = `INSERT INTO priorities (name, rate, code) VALUES ($1, $2, $3) 
				 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, rate = EXCLUDED.rate;`
		log.Println("    - Стратегия: Обновление существующих приоритетов (UPSERT)")
	} else {
		query = `INSERT INTO priorities (name, rate, code) VALUES ($1, $2, $3) 
				 ON CONFLICT (code) DO NOTHING;`
		log.Println("    - Стратегия: Пропуск существующих приоритетов (IGNORE)")
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, s := range prioritiesData {
		if _, err := tx.Exec(ctx, query, s.Name, s.Rate, s.Code); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

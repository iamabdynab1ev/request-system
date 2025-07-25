package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) (err error) {
	var tx pgx.Tx
	tx, err = pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				err = fmt.Errorf("ошибка при откате транзакции: %v (изначальная ошибка: %w)", rbErr, err)
			}
		} else {
			err = tx.Commit(ctx)
			if err != nil {
				err = fmt.Errorf("ошибка при коммите транзакции: %w", err)
			}
		}
	}()

	err = fn(tx)
	return err
}

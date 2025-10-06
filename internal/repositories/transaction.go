package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxManagerInterface interface {
	RunInTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) TxManagerInterface {
	return &TxManager{pool: pool}
}

func (m *TxManager) RunInTransaction(ctx context.Context, fn func(tx pgx.Tx) error) (err error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			_ = tx.Rollback(ctx)
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

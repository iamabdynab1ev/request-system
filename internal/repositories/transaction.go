package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type TxManagerInterface interface {
	RunInTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type TxManager struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// --- ИЗМЕНЕНИЕ 1: ДОБАВЛЯЕМ ЛОГГЕР В КОНСТРУКТОР ---
func NewTxManager(pool *pgxpool.Pool, logger *zap.Logger) TxManagerInterface {
	return &TxManager{pool: pool, logger: logger}
}

func (m *TxManager) RunInTransaction(ctx context.Context, fn func(tx pgx.Tx) error) (err error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {

			m.logger.Error("Паника в транзакции, откат", zap.Any("panic", p))
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {

			m.logger.Warn("Транзакция отменена из-за ошибки", zap.Error(err))
			_ = tx.Rollback(ctx)
		} else {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				m.logger.Error("Ошибка при коммите транзакции", zap.Error(commitErr))
				err = fmt.Errorf("ошибка при коммите транзакции: %w", commitErr)
			} else {
				m.logger.Debug("Транзакция успешно закоммичена")
			}
		}
	}()

	err = fn(tx)
	return err
}

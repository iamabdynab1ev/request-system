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
	logger *zap.Logger // Add this
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
			// Log panic details for debugging
			// Assuming logger injected; if not, add to NewTxManager and struct
			// m.logger.Error("Panic in transaction, rolling back", zap.Any("panic", p))
			_ = tx.Rollback(ctx)
			panic(p) // Re-panic to propagate
		} else if err != nil {
			// fn returned err: rollback (already doomed if DB error)
			_ = tx.Rollback(ctx)
			// m.logger.Warn("Transaction rolled back due to fn error", zap.Error(err))
		} else {
			// Attempt commit
			if commitErr := tx.Commit(ctx); commitErr != nil {
				// This is the "unexpected rollback" case—log specifically
				// m.logger.Error("Transaction commit failed (likely doomed tx)", zap.Error(commitErr))
				err = fmt.Errorf("ошибка при коммите транзакции: %w", commitErr)
				// Note: Original cause should be in err from fn; if not, it's ignored upstream
			} else {
				// Success
				// m.logger.Debug("Transaction committed successfully")
			}
		}
	}()

	err = fn(tx)
	return err
}

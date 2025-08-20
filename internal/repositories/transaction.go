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

// RunInTransaction выполняет функцию `fn` в рамках одной транзакции.
// Этот подход является идиоматичным и безопасным в Go.
func (m *TxManager) RunInTransaction(ctx context.Context, fn func(tx pgx.Tx) error) (err error) {
	// ИЗМЕНЕНИЕ 1: Мы используем "именованный результат" `(err error)`.
	// Это создает переменную `err`, которая доступна во всей функции, включая `defer`.
	// Теперь `defer` будет видеть ошибку, которая произошла внутри `fn`.

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}

	// ИЗМЕНЕНИЕ 2: Вся логика коммита и отката теперь находится в одном `defer`.
	// Этот блок выполнится в самом конце, перед тем как функция вернет управление.
	defer func() {
		// Шаг 1: Проверяем, была ли паника. Если да, откатываем транзакцию.
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			// Важно! "Перебрасываем" панику дальше, чтобы не скрывать серьезную ошибку.
			panic(p)
		} else if err != nil {
			// Шаг 2: Если паники не было, проверяем нашу переменную `err`.
			// Если она НЕ пустая (т.е. в `fn` произошла ошибка), откатываем транзакцию.
			// Мы не возвращаем ошибку отката, потому что исходная ошибка (`err`) важнее.
			_ = tx.Rollback(ctx)
		} else {
			// Шаг 3: Если паники не было и `err` пустой, значит все прошло успешно.
			// Коммитим транзакцию.
			// Если коммит вернет ошибку, она будет присвоена нашей переменной `err` и вернется пользователю.
			err = tx.Commit(ctx)
			if err != nil {
				err = fmt.Errorf("ошибка при коммите транзакции: %w", err)
			}
		}
	}()

	// ИЗМЕНЕНИЕ 3: Выполняем основную бизнес-логику.
	// Результат (ошибку или `nil`) мы присваиваем НАШЕЙ ОБЩЕЙ переменной `err`.
	err = fn(tx)

	// В конце функция неявно вернет то, что лежит в переменной `err`
	// после того, как `defer` выполнит свою работу.
	return err
}

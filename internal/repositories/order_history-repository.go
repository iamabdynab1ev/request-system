package repositories

import (
	"context"
	"database/sql"
	"request-system/internal/entities"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrderHistoryItem - DTO для передачи обогащенных данных из репозитория в сервис.
type OrderHistoryItem struct {
	entities.OrderHistory
	ActorFio      sql.NullString `db:"actor_fio"`
	NewStatusName sql.NullString `db:"new_status_name"`
}

type OrderHistoryRepositoryInterface interface {
	CreateInTx(ctx context.Context, tx pgx.Tx, history *entities.OrderHistory, attachmentID *uint64) error
	FindByOrderID(ctx context.Context, orderID uint64) ([]OrderHistoryItem, error)
}

type OrderHistoryRepository struct {
	storage *pgxpool.Pool
}

func NewOrderHistoryRepository(storage *pgxpool.Pool) OrderHistoryRepositoryInterface {
	return &OrderHistoryRepository{storage: storage}
}

func (r *OrderHistoryRepository) CreateInTx(ctx context.Context, tx pgx.Tx, history *entities.OrderHistory, attachmentID *uint64) error {
	query := `
		INSERT INTO order_history (order_id, user_id, event_type, old_value, new_value, comment, attachment_id) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := tx.Exec(ctx, query,
		history.OrderID, history.UserID, history.EventType,
		history.OldValue, history.NewValue, history.Comment, attachmentID)
	return err
}

// ФИНАЛЬНЫЙ ИСПРАВЛЕННЫЙ МЕТОД
func (r *OrderHistoryRepository) FindByOrderID(ctx context.Context, orderID uint64) ([]OrderHistoryItem, error) {
	// ИСПРАВЛЕННЫЙ SQL-ЗАПРОС
	query := `
		SELECT 
			h.id, h.order_id, h.user_id, h.event_type, h.old_value, h.new_value, h.comment, h.created_at,
			u.fio AS actor_fio,
			s.name AS new_status_name
		FROM 
			order_history h
		LEFT JOIN 
			users u ON h.user_id = u.id
		LEFT JOIN 
			statuses s ON s.id = (
				CASE 
					WHEN h.event_type = 'STATUS_CHANGE' AND h.new_value ~ '^[0-9]+$' 
					THEN CAST(h.new_value AS INTEGER)
					ELSE NULL
				END
			)
		WHERE h.order_id = $1
		ORDER BY h.created_at ASC, h.id ASC
	`
	// КОНЕЦ ИСПРАВЛЕННОГО ЗАПРОСА

	rows, err := r.storage.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []OrderHistoryItem
	for rows.Next() {
		var h OrderHistoryItem
		if err := rows.Scan(
			&h.ID, &h.OrderID, &h.UserID, &h.EventType, &h.OldValue, &h.NewValue, &h.Comment, &h.CreatedAt,
			&h.ActorFio, &h.NewStatusName,
		); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

package repositories

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/entities"
	"request-system/pkg/types"
)

type OrderHistoryItem struct {
	ID            uint64               `json:"id"`
	OrderID       uint64               `json:"order_id"`
	UserID        uint64               `json:"user_id"`
	EventType     string               `json:"event_type"`
	OldValue      sql.NullString       `json:"old_value"`
	NewValue      sql.NullString       `json:"new_value"`
	Comment       sql.NullString       `json:"comment"`
	AttachmentID  sql.NullInt64        `json:"attachment_id"`
	Attachment    *entities.Attachment `json:"attachment"`
	NewStatusName sql.NullString       `json:"new_status_name"`
	CreatedAt     time.Time            `json:"created_at"`
	TxID          *uuid.UUID           `json:"tx_id"`
	CreatorFio    sql.NullString       `json:"creator_fio"`
	DelegatorFio  sql.NullString       `json:"delegator_fio"`
	ExecutorFio   sql.NullString       `json:"executor_fio"`
}

// OrderHistoryRepositoryInterface определяет методы для работы с историей заявок
type OrderHistoryRepositoryInterface interface {
	FindByOrderID(ctx context.Context, orderID uint64, limit, offset uint64) ([]OrderHistoryItem, error)
	CreateInTx(ctx context.Context, tx pgx.Tx, item *OrderHistoryItem) error
	IsUserParticipant(ctx context.Context, orderID, userID uint64) (bool, error)
	GetOrderHistory(ctx context.Context, orderID uint64, filter types.Filter) ([]OrderHistoryItem, error)
}

// OrderHistoryRepository реализует доступ к таблице order_history
type OrderHistoryRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

// NewOrderHistoryRepository создает новый экземпляр OrderHistoryRepository
func NewOrderHistoryRepository(storage *pgxpool.Pool, logger *zap.Logger) OrderHistoryRepositoryInterface {
	return &OrderHistoryRepository{storage: storage, logger: logger}
}

func (r *OrderHistoryRepository) GetOrderHistory(ctx context.Context, orderID uint64, filter types.Filter) ([]OrderHistoryItem, error) {
	return r.FindByOrderID(ctx, orderID, uint64(filter.Limit), uint64(filter.Offset))
}

// CreateInTx создает запись в истории в рамках транзакции
func (r *OrderHistoryRepository) CreateInTx(ctx context.Context, tx pgx.Tx, item *OrderHistoryItem) error {
	query := `
		INSERT INTO order_history (
			order_id, user_id, event_type, old_value, new_value, comment, attachment_id,
			created_at, tx_id, creator_fio, delegator_fio, executor_fio
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := tx.Exec(ctx, query,
		item.OrderID,
		item.UserID,
		item.EventType,
		item.OldValue,
		item.NewValue,
		item.Comment,
		item.AttachmentID,
		item.CreatedAt,
		item.TxID,
		item.CreatorFio,
		item.DelegatorFio,
		item.ExecutorFio,
	)
	if err != nil {
		r.logger.Error("Ошибка при создании записи в истории",
			zap.Uint64("orderID", item.OrderID),
			zap.String("eventType", item.EventType),
			zap.Error(err))
		return err
	}
	r.logger.Debug("Запись в истории создана",
		zap.Uint64("orderID", item.OrderID),
		zap.String("eventType", item.EventType),
		zap.Uint64("userID", item.UserID))
	return nil
}

// FindByOrderID получает историю заявки с пагинацией
func (r *OrderHistoryRepository) FindByOrderID(ctx context.Context, orderID uint64, limit, offset uint64) ([]OrderHistoryItem, error) {
	query := `
		SELECT 
			h.id, h.order_id, h.user_id, h.event_type, h.old_value, h.new_value, h.comment, h.created_at, h.attachment_id,
			s.name AS new_status_name,
			h.creator_fio, h.delegator_fio, h.executor_fio,
			a.file_name, a.file_path, a.file_type, a.file_size,
			h.tx_id
		FROM order_history h
		LEFT JOIN statuses s ON h.new_value = s.id::text AND h.event_type = 'STATUS_CHANGE'
		LEFT JOIN attachments a ON h.attachment_id = a.id
		WHERE h.order_id = $1
		ORDER BY h.created_at ASC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.storage.Query(ctx, query, orderID, limit, offset)
	if err != nil {
		r.logger.Error("Ошибка при получении истории заявки",
			zap.Uint64("orderID", orderID),
			zap.Uint64("limit", limit),
			zap.Uint64("offset", offset),
			zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	history := make([]OrderHistoryItem, 0, limit)
	for rows.Next() {
		var item OrderHistoryItem
		var fileName, filePath, fileType sql.NullString
		var fileSize sql.NullInt64

		err := rows.Scan(
			&item.ID,
			&item.OrderID,
			&item.UserID,
			&item.EventType,
			&item.OldValue,
			&item.NewValue,
			&item.Comment,
			&item.CreatedAt,
			&item.AttachmentID, // Сканируем напрямую в поле структуры
			&item.NewStatusName,
			&item.CreatorFio,
			&item.DelegatorFio,
			&item.ExecutorFio,
			&fileName,
			&filePath,
			&fileType,
			&fileSize,
			&item.TxID,
		)
		if err != nil {
			r.logger.Error("Ошибка при сканировании строки истории",
				zap.Uint64("orderID", orderID),
				zap.Error(err))
			return nil, err
		}

		if item.AttachmentID.Valid {
			item.Attachment = &entities.Attachment{
				ID:       uint64(item.AttachmentID.Int64),
				FileName: fileName.String,
				FilePath: filePath.String,
				FileType: fileType.String,
				FileSize: fileSize.Int64,
			}
		} else {
			item.Attachment = nil
		}

		history = append(history, item)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Ошибка при итерации строк истории",
			zap.Uint64("orderID", orderID),
			zap.Error(err))
		return nil, err
	}

	r.logger.Info("История заявки получена",
		zap.Uint64("orderID", orderID),
		zap.Int("count", len(history)))
	return history, nil
}

// IsUserParticipant проверяет, участвовал ли пользователь в истории заявки
func (r *OrderHistoryRepository) IsUserParticipant(ctx context.Context, orderID, userID uint64) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM order_history WHERE order_id = $1 AND user_id = $2)`
	var exists bool
	err := r.storage.QueryRow(ctx, query, orderID, userID).Scan(&exists)
	if err != nil {
		r.logger.Error("Ошибка при проверке участия пользователя",
			zap.Uint64("orderID", orderID),
			zap.Uint64("userID", userID),
			zap.Error(err))
		return false, err
	}
	r.logger.Debug("Проверка участия пользователя",
		zap.Uint64("orderID", orderID),
		zap.Uint64("userID", userID),
		zap.Bool("exists", exists))
	return exists, nil
}

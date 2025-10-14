package repositories

import (
	"context"
	"database/sql"
	"fmt"

	// Убедись, что все импорты на месте
	"request-system/internal/entities"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderHistoryItem struct {
	entities.OrderHistory
	ActorFio      sql.NullString
	NewStatusName sql.NullString
	Attachment    *entities.Attachment
}

type OrderHistoryRepositoryInterface interface {
	CreateInTx(ctx context.Context, tx pgx.Tx, history *entities.OrderHistory, attachmentID *uint64) error
	FindByOrderID(ctx context.Context, orderID uint64) ([]OrderHistoryItem, error)
	IsUserParticipant(ctx context.Context, orderID, userID uint64) (bool, error) // Метод здесь, в интерфейсе
}

type OrderHistoryRepository struct { // А здесь структура
	storage *pgxpool.Pool
}

// Конструктор
func NewOrderHistoryRepository(storage *pgxpool.Pool) OrderHistoryRepositoryInterface { // Возвращаем интерфейс
	return &OrderHistoryRepository{storage: storage} // Создаем экземпляр структуры
}

// CreateInTx ...
func (r *OrderHistoryRepository) CreateInTx(ctx context.Context, tx pgx.Tx, history *entities.OrderHistory, attachmentID *uint64) error {
	query := `
        INSERT INTO order_history (
            order_id, user_id, event_type, old_value, new_value, 
            comment, attachment_id, tx_id, metadata
        ) 
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `

	_, err := tx.Exec(ctx, query,
		history.OrderID,
		history.UserID,
		history.EventType,
		history.OldValue,
		history.NewValue,
		history.Comment,
		attachmentID,
		history.TxID,
		history.Metadata,
	)
	// Добавим логгирование, чтобы точно видеть ошибку, если она будет
	if err != nil {
		return fmt.Errorf("ошибка при записи в order_history: %w", err)
	}

	return nil
}

func (r *OrderHistoryRepository) FindByOrderID(ctx context.Context, orderID uint64) ([]OrderHistoryItem, error) {
	query := `
		SELECT 
			h.id, h.order_id, h.user_id, h.event_type, h.old_value, h.new_value, h.comment, h.created_at, h.attachment_id,
			u.fio AS actor_fio,
			s.name AS new_status_name,
			a.file_name, a.file_path, a.file_type, a.file_size
		FROM 
			order_history h
		LEFT JOIN users u ON h.user_id = u.id
		LEFT JOIN statuses s ON s.id = (
			CASE 
				WHEN h.event_type = 'STATUS_CHANGE' AND h.new_value ~ '^[0-9]+$' 
				THEN CAST(h.new_value AS INTEGER)
				ELSE NULL
			END
		)
		LEFT JOIN attachments a ON h.attachment_id = a.id
		WHERE h.order_id = $1
		ORDER BY h.created_at ASC, h.id ASC
	`

	rows, err := r.storage.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []OrderHistoryItem
	for rows.Next() {
		var h OrderHistoryItem
		var fileName, filePath, fileType sql.NullString
		var fileSize sql.NullInt64

		// Сканируем attachment_id напрямую в поле структуры h
		if err := rows.Scan(
			&h.ID, &h.OrderID, &h.UserID, &h.EventType, &h.OldValue, &h.NewValue, &h.Comment, &h.CreatedAt, &h.AttachmentID,
			&h.ActorFio, &h.NewStatusName,
			&fileName, &filePath, &fileType, &fileSize,
		); err != nil {
			return nil, err
		}

		// --- ИСПРАВЛЕНО ЗДЕСЬ ---
		// Проверяем поле Valid у структуры sql.NullInt64
		if h.AttachmentID.Valid {
			// Если attachment_id не NULL, создаем вложенную структуру Attachment
			h.Attachment = &entities.Attachment{
				ID:       uint64(h.AttachmentID.Int64), // Используем значение из структуры
				FileName: fileName.String,
				FilePath: filePath.String,
				FileType: fileType.String,
				FileSize: fileSize.Int64,
			}
		}
		// --- КОНЕЦ ИСПРАВЛЕНИЙ ---

		history = append(history, h)
	}
	return history, rows.Err()
}

func (r *OrderHistoryRepository) IsUserParticipant(ctx context.Context, orderID, userID uint64) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM order_history WHERE order_id = $1 AND user_id = $2)`
	var exists bool
	err := r.storage.QueryRow(ctx, query, orderID, userID).Scan(&exists)
	return exists, err
}

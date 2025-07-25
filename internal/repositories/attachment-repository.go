package repositories

import (
	"context"
	"request-system/internal/entities"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AttachmentRepositoryInterface interface {
	Create(ctx context.Context, tx pgx.Tx, attachment *entities.Attachment) (uint64, error)
	FindAllByOrderID(ctx context.Context, orderID uint64, limit, offset int) ([]entities.Attachment, error)
}

type AttachmentRepository struct {
	storage *pgxpool.Pool
}

func NewAttachmentRepository(storage *pgxpool.Pool) AttachmentRepositoryInterface {
	return &AttachmentRepository{
		storage: storage,
	}
}

func (r *AttachmentRepository) Create(ctx context.Context, tx pgx.Tx, attachment *entities.Attachment) (uint64, error) {
	query := `
		INSERT INTO attachments 
		(order_id, user_id, file_name, file_path, file_type, file_size)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`
	var attachmentID uint64
	err := tx.QueryRow(ctx, query,
		attachment.OrderID, attachment.UserID, attachment.FileName,
		attachment.FilePath, attachment.FileType, attachment.FileSize,
	).Scan(&attachmentID)
	return attachmentID, err
}

func (r *AttachmentRepository) FindAllByOrderID(ctx context.Context, orderID uint64, limit, offset int) ([]entities.Attachment, error) {
	query := `
		SELECT id, order_id, user_id, file_name, file_path, file_type, file_size, created_at 
		FROM attachments 
		WHERE order_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`
	rows, err := r.storage.Query(ctx, query, orderID, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var attachments []entities.Attachment
	for rows.Next() {
		var a entities.Attachment
		if err := rows.Scan(&a.ID, &a.OrderID, &a.UserID, &a.FileName, &a.FilePath, &a.FileType, &a.FileSize, &a.CreatedAt); err != nil {
			return nil, err
		}
		attachments = append(attachments, a)
	}
	return attachments, rows.Err()
}

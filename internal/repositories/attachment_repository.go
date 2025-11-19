// Файл: internal/repositories/attachment_repository.go
package repositories

import (
	"context"
	"errors"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AttachmentRepositoryInterface interface {
	CreateInTx(ctx context.Context, tx pgx.Tx, attachment *entities.Attachment) (uint64, error)
	FindAllByOrderID(ctx context.Context, orderID uint64, limit, offset int) ([]entities.Attachment, error)
	FindByID(ctx context.Context, id uint64) (*entities.Attachment, error)
	DeleteAttachment(ctx context.Context, id uint64) error
	FindAttachmentsByOrderIDs(ctx context.Context, orderIDs []uint64) (map[uint64][]entities.Attachment, error)
}

type attachmentRepository struct {
	storage *pgxpool.Pool
}

func NewAttachmentRepository(storage *pgxpool.Pool) AttachmentRepositoryInterface {
	return &attachmentRepository{
		storage: storage,
	}
}

// <<<--- 2. ДОБАВЛЕНА РЕАЛИЗАЦИЯ НОВОГО МЕТОДА ---
func (r *attachmentRepository) FindAttachmentsByOrderIDs(ctx context.Context, orderIDs []uint64) (map[uint64][]entities.Attachment, error) {
	if len(orderIDs) == 0 {
		return make(map[uint64][]entities.Attachment), nil
	}

	query := `SELECT id, order_id, user_id, file_name, file_path, file_type, file_size, created_at FROM attachments WHERE order_id = ANY($1)`
	rows, err := r.storage.Query(ctx, query, orderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attachmentsMap := make(map[uint64][]entities.Attachment)
	for rows.Next() {
		var a entities.Attachment
		if err := rows.Scan(&a.ID, &a.OrderID, &a.UserID, &a.FileName, &a.FilePath, &a.FileType, &a.FileSize, &a.CreatedAt); err != nil {
			return nil, err
		}
		attachmentsMap[a.OrderID] = append(attachmentsMap[a.OrderID], a)
	}

	return attachmentsMap, rows.Err()
}

func (r *attachmentRepository) CreateInTx(ctx context.Context, tx pgx.Tx, attachment *entities.Attachment) (uint64, error) {
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

func (r *attachmentRepository) FindAllByOrderID(ctx context.Context, orderID uint64, limit, offset int) ([]entities.Attachment, error) {
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

func (r *attachmentRepository) FindByID(ctx context.Context, id uint64) (*entities.Attachment, error) {
	query := `SELECT id, file_path FROM attachments WHERE id = $1`
	var attachment entities.Attachment
	err := r.storage.QueryRow(ctx, query, id).Scan(&attachment.ID, &attachment.FilePath)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &attachment, nil
}

func (r *attachmentRepository) DeleteAttachment(ctx context.Context, id uint64) error {
	query := "DELETE FROM attachments WHERE id = $1"
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

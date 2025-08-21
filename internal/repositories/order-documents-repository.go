package repositories

import (
	"context"
	"fmt"
	"time"

	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	orderDocumentTableRepo  = "order_documents"
	orderDocumentFieldsRepo = "id, name, path, type, order_id, created_at, updated_at"
)

type OrderDocumentRepositoryInterface interface {
	GetOrderDocuments(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDocumentDTO, error)
	FindOrderDocument(ctx context.Context, id uint64) (*dto.OrderDocumentDTO, error)
	CreateOrderDocument(ctx context.Context, dto dto.CreateOrderDocumentDTO) error
	UpdateOrderDocument(ctx context.Context, id uint64, dto dto.UpdateOrderDocumentDTO) error
	DeleteOrderDocument(ctx context.Context, id uint64) error
}

type OrderDocumentRepository struct {
	storage *pgxpool.Pool
}

func NewOrderDocumentRepository(storage *pgxpool.Pool) OrderDocumentRepositoryInterface {
	return &OrderDocumentRepository{
		storage: storage,
	}
}

func (r *OrderDocumentRepository) GetOrderDocuments(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderDocumentDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		`, orderDocumentFieldsRepo, orderDocumentTableRepo)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orderDocuments := make([]dto.OrderDocumentDTO, 0)

	for rows.Next() {
		var orderDocument dto.OrderDocumentDTO
		var createdAt time.Time
		var updatedAt time.Time

		err := rows.Scan(
			&orderDocument.ID,
			&orderDocument.Name,
			&orderDocument.Path,
			&orderDocument.Type,
			&orderDocument.OrderID,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		createdAtLocal := createdAt.Local()
		updatedAtLocal := updatedAt.Local()

		orderDocument.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
		orderDocument.UpdatedAt = updatedAtLocal.Format("2006-01-02 15:04:05")

		orderDocuments = append(orderDocuments, orderDocument)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orderDocuments, nil
}

func (r *OrderDocumentRepository) FindOrderDocument(ctx context.Context, id uint64) (*dto.OrderDocumentDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		WHERE r.id = $1
	`, orderDocumentFieldsRepo, orderDocumentTableRepo)

	var orderDocument dto.OrderDocumentDTO
	var createdAt time.Time
	var updatedAt time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&orderDocument.ID,
		&orderDocument.Name,
		&orderDocument.Path,
		&orderDocument.Type,
		&orderDocument.OrderID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	createdAtLocal := createdAt.Local()
	updatedAtLocal := updatedAt.Local()

	orderDocument.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
	orderDocument.UpdatedAt = updatedAtLocal.Format("2006-01-02 15:04:05")

	return &orderDocument, nil
}

func (r *OrderDocumentRepository) CreateOrderDocument(ctx context.Context, dto dto.CreateOrderDocumentDTO) error {
	query := fmt.Sprintf(`
        INSERT INTO %s (name, path, type, order_id)
        VALUES ($1, $2, $3, $4)
    `, orderDocumentTableRepo)

	_, err := r.storage.Exec(ctx, query,
		dto.Name,
		dto.Path,
		dto.Type,
		dto.OrderID,
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *OrderDocumentRepository) UpdateOrderDocument(ctx context.Context, id uint64, dto dto.UpdateOrderDocumentDTO) error {
	query := fmt.Sprintf(`
        UPDATE %s
        SET name = $1, path = $2, type = $3, order_id = $4, updated_at = CURRENT_TIMESTAMP
        WHERE id = $5
    `, orderDocumentTableRepo)

	result, err := r.storage.Exec(ctx, query,
		dto.Name,
		dto.Path,
		dto.Type,
		dto.OrderID,
		id,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *OrderDocumentRepository) DeleteOrderDocument(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", orderDocumentTableRepo)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

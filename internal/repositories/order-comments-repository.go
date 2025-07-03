package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"request-system/internal/dto"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderCommentRepositoryInterface interface {
	CreateOrderComment(ctx context.Context, dto dto.CreateOrderCommentDTO) (int, error)
	CreateOrderCommentInTx(ctx context.Context, tx pgx.Tx, authorID int, dto dto.CreateOrderCommentDTO) error
	GetOrderComments(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderCommentDTO, uint64, error)
	FindOrderComment(ctx context.Context, id uint64) (*dto.OrderCommentDTO, error)
	UpdateOrderComment(ctx context.Context, id uint64, dto dto.UpdateOrderCommentDTO) error
	DeleteOrderComment(ctx context.Context, id uint64) error
}

type OrderCommentRepository struct {
	storage *pgxpool.Pool
}

func NewOrderCommentRepository(storage *pgxpool.Pool) OrderCommentRepositoryInterface {
	return &OrderCommentRepository{
		storage: storage,
	}
}

func (r *OrderCommentRepository) CreateOrderComment(ctx context.Context, dto dto.CreateOrderCommentDTO) (int, error) {
	authorID, ok := ctx.Value(contextkeys.UserIDKey).(int)
	if !ok || authorID == 0 {
		return 0, apperrors.ErrInvalidUserID
	}

	query := `INSERT INTO order_comments (message, status_id, order_id, user_id, created_at, updated_at) 
	          VALUES ($1, $2, $3, $4, NOW(), NOW()) RETURNING id`

	var newID int
	err := r.storage.QueryRow(ctx, query, dto.Message, dto.StatusID, dto.OrderID, authorID).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("ошибка создания комментария: %w", err)
	}
	return newID, nil
}

func (r *OrderCommentRepository) CreateOrderCommentInTx(ctx context.Context, tx pgx.Tx, authorID int, dto dto.CreateOrderCommentDTO) error {
	query := `INSERT INTO order_comments (order_id, user_id, message, status_id, created_at, updated_at) 
	          VALUES ($1, $2, $3, $4, NOW(), NOW())`

	_, err := tx.Exec(ctx, query, dto.OrderID, authorID, dto.Message, dto.StatusID)
	if err != nil {
		return fmt.Errorf("ошибка создания записи в 'order_comments': %w", err)
	}
	return nil
}

func (r *OrderCommentRepository) GetOrderComments(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderCommentDTO, uint64, error) {
	var total uint64
	if err := r.storage.QueryRow(ctx, `SELECT COUNT(*) FROM order_comments`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета комментариев: %w", err)
	}

	query := `
		SELECT 
			c.id, c.message, c.created_at, c.updated_at,
			s.id, s.name,
			o.id, o.name,
			u.id, u.fio
		FROM order_comments c
		LEFT JOIN statuses s ON c.status_id = s.id
		LEFT JOIN orders o ON c.order_id = o.id
		LEFT JOIN users u ON c.user_id = u.id
		ORDER BY c.created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка при запросе комментариев: %w", err)
	}
	defer rows.Close()

	var comments []dto.OrderCommentDTO
	for rows.Next() {
		var c dto.OrderCommentDTO
		var createdAt, updatedAt time.Time
		var statusName, orderName, authorFio sql.NullString
		var statusId, orderId, authorId sql.NullInt32

		err := rows.Scan(
			&c.ID, &c.Message, &createdAt, &updatedAt,
			&statusId, &statusName, &orderId, &orderName, &authorId, &authorFio,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка при сканировании комментария: %w", err)
		}

		if statusId.Valid {
			c.Status.ID = int(statusId.Int32)
		}
		if statusName.Valid {
			c.Status.Name = statusName.String
		}
		if orderId.Valid {
			c.Order.ID = int(orderId.Int32)
		}
		if orderName.Valid {
			c.Order.Name = orderName.String
		}
		if authorId.Valid {
			c.Author.ID = int(authorId.Int32)
		}
		if authorFio.Valid {
			c.Author.Fio = authorFio.String
		}

		c.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
		c.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
		comments = append(comments, c)
	}
	return comments, total, nil
}

func (r *OrderCommentRepository) FindOrderComment(ctx context.Context, id uint64) (*dto.OrderCommentDTO, error) {
	query := `
		SELECT 
			c.id, c.message, c.created_at, c.updated_at,
			s.id, s.name, o.id, o.name, u.id, u.fio
		FROM order_comments c
		LEFT JOIN statuses s ON c.status_id = s.id
		LEFT JOIN orders o ON c.order_id = o.id
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.id = $1`

	var comment dto.OrderCommentDTO
	var createdAt, updatedAt time.Time
	var statusName, orderName, authorFio sql.NullString
	var statusId, orderId, authorId sql.NullInt32

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&comment.ID, &comment.Message, &createdAt, &updatedAt,
		&statusId, &statusName, &orderId, &orderName, &authorId, &authorFio,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, err
	}

	if statusId.Valid {
		comment.Status.ID = int(statusId.Int32)
	}
	if statusName.Valid {
		comment.Status.Name = statusName.String
	}
	if orderId.Valid {
		comment.Order.ID = int(orderId.Int32)
	}
	if orderName.Valid {
		comment.Order.Name = orderName.String
	}
	if authorId.Valid {
		comment.Author.ID = int(authorId.Int32)
	}
	if authorFio.Valid {
		comment.Author.Fio = authorFio.String
	}

	comment.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	comment.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
	return &comment, nil
}

func (r *OrderCommentRepository) UpdateOrderComment(ctx context.Context, id uint64, dto dto.UpdateOrderCommentDTO) error {
	query := `UPDATE order_comments SET message = $1, status_id = $2, updated_at = NOW() WHERE id = $3`

	result, err := r.storage.Exec(ctx, query, dto.Message, dto.StatusID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}
	return nil
}

func (r *OrderCommentRepository) DeleteOrderComment(ctx context.Context, id uint64) error {
	_, err := r.storage.Exec(ctx, `DELETE FROM order_comments WHERE id = $1`, id)
	return err
}

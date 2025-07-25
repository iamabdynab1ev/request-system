package repositories
/*
import (
	"context"
	"database/sql"
	"fmt"
	"request-system/internal/dto"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderCommentRepositoryInterface interface {
	GetOrderComments(ctx context.Context, limit uint64, offset uint64) ([]dto.OrderCommentDTO, uint64, error)
	FindOrderComment(ctx context.Context, id uint64) (*dto.OrderCommentDTO, error)
	CreateOrderComment(ctx context.Context, dto dto.CreateOrderCommentDTO) (int, error)
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
		var statusID, orderID, authorID sql.NullInt32

		err := rows.Scan(
			&c.ID, &c.Message, &createdAt, &updatedAt,
			&statusID, &statusName, &orderID, &orderName, &authorID, &authorFio,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка при сканировании комментария: %w", err)
		}

		if statusID.Valid {
			c.Status.ID = int(statusID.Int32)
		}
		if statusName.Valid {
			c.Status.Name = statusName.String
		}
		if orderID.Valid {
			c.Order.ID = int(orderID.Int32)
		}
		if orderName.Valid {
			c.Order.Name = orderName.String
		}
		if authorID.Valid {
			c.Author.ID = int(authorID.Int32)
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
	var statusID, orderID, authorID sql.NullInt32

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&comment.ID, &comment.Message, &createdAt, &updatedAt,
		&statusID, &statusName, &orderID, &orderName, &authorID, &authorFio,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	if statusID.Valid {
		comment.Status.ID = int(statusID.Int32)
	}
	if statusName.Valid {
		comment.Status.Name = statusName.String
	}
	if orderID.Valid {
		comment.Order.ID = int(orderID.Int32)
	}
	if orderName.Valid {
		comment.Order.Name = orderName.String
	}
	if authorID.Valid {
		comment.Author.ID = int(authorID.Int32)
	}
	if authorFio.Valid {
		comment.Author.Fio = authorFio.String
	}

	comment.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	comment.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
	return &comment, nil
}

func (r *OrderCommentRepository) CreateOrderComment(ctx context.Context, dto dto.CreateOrderCommentDTO) (int, error) {
	authorID, ok := ctx.Value(contextkeys.UserIDKey).(int)
	if !ok || authorID == 0 {
		return 0, fmt.Errorf("не удалось определить автора комментария из токена")
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

func (r *OrderCommentRepository) UpdateOrderComment(ctx context.Context, id uint64, dto dto.UpdateOrderCommentDTO) error {
	query := `UPDATE order_comments SET message = $1, status_id = $2, updated_at = NOW() WHERE id = $3`

	result, err := r.storage.Exec(ctx, query, dto.Message, dto.StatusID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *OrderCommentRepository) DeleteOrderComment(ctx context.Context, id uint64) error {
	_, err := r.storage.Exec(ctx, `DELETE FROM order_comments WHERE id = $1`, id)
	return err
}
*/
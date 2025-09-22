package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv" // <<<--- 1. ДОБАВЛЯЕМ НУЖНЫЙ ИМПОРТ
	"strings"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const branchTable = "branches"

var (
	branchAllowedFilterFields = map[string]string{"status_id": "b.status_id"}
	branchAllowedSortFields   = map[string]bool{"id": true, "name": true, "created_at": true, "updated_at": true}
)

type BranchRepositoryInterface interface {
	GetBranches(ctx context.Context, filter types.Filter) ([]entities.Branch, uint64, error)
	FindBranch(ctx context.Context, id uint64) (*entities.Branch, error)
	CreateBranch(ctx context.Context, branch entities.Branch) (*entities.Branch, error)
	UpdateBranch(ctx context.Context, id uint64, branch entities.Branch) (*entities.Branch, error)
	DeleteBranch(ctx context.Context, id uint64) error
}

type BranchRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewBranchRepository(storage *pgxpool.Pool, logger *zap.Logger) BranchRepositoryInterface {
	return &BranchRepository{storage: storage, logger: logger}
}

func (r *BranchRepository) buildFilterQuery(filter types.Filter) (string, []interface{}) {
	args := make([]interface{}, 0)
	conditions := []string{}
	argCounter := 1

	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%"
		conditions = append(conditions, fmt.Sprintf("(LOWER(b.name) ILIKE $%d OR LOWER(b.short_name) ILIKE $%d)", argCounter, argCounter))
		args = append(args, searchPattern)
		argCounter++
	}

	for key, value := range filter.Filter {
		dbColumn, isAllowed := branchAllowedFilterFields[key]
		if !isAllowed {
			continue
		}

		// Парсер всегда дает string, поэтому работаем только с этим типом.
		if itemStr, ok := value.(string); ok && itemStr != "" {
			// НОВАЯ ЛОГИКА: Проверяем, есть ли в строке запятая
			if strings.Contains(itemStr, ",") {
				// Если есть, это IN-запрос (например, "10, 5")
				items := strings.Split(itemStr, ",")
				placeholders := []string{}

				for _, item := range items {
					val, err := strconv.Atoi(strings.TrimSpace(item))
					if err != nil {
						continue // Пропускаем невалидные части, например, пустые строки
					}
					placeholders = append(placeholders, fmt.Sprintf("$%d", argCounter))
					args = append(args, val)
					argCounter++
				}

				if len(placeholders) > 0 {
					conditions = append(conditions, fmt.Sprintf("%s IN (%s)", dbColumn, strings.Join(placeholders, ",")))
				}
			} else {
				// Если запятой нет, это обычный запрос с одним значением
				val, err := strconv.Atoi(strings.TrimSpace(itemStr))
				if err != nil {
					continue // Пропускаем, если значение не является числом
				}
				conditions = append(conditions, fmt.Sprintf("%s = $%d", dbColumn, argCounter))
				args = append(args, val)
				argCounter++
			}
		}
	}

	var whereClause string
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}
	return whereClause, args
}

func (r *BranchRepository) scanBranch(row pgx.Row) (*entities.Branch, error) {
	var b entities.Branch
	var s entities.Status

	// Создаем временные переменные для полей, которые могут быть NULL
	var emailIndex sql.NullString

	err := row.Scan(
		&b.ID, &b.Name, &b.ShortName, &b.Address, &b.PhoneNumber,
		&b.Email,
		&emailIndex, // Сканируем значение email_index в sql.NullString
		&b.OpenDate, &b.StatusID,
		&b.CreatedAt, &b.UpdatedAt,
		&s.ID, &s.Name,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		r.logger.Error("Failed to scan branch row", zap.Error(err))
		return nil, err
	}
	if emailIndex.Valid {
		b.EmailIndex = emailIndex.String
	}

	if s.ID > 0 {
		b.Status = &s
	}
	return &b, nil
}

func (r *BranchRepository) GetBranches(ctx context.Context, filter types.Filter) ([]entities.Branch, uint64, error) {
	whereClause, args := r.buildFilterQuery(filter)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s b %s", branchTable, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Branch{}, 0, nil
	}

	orderByClause := "ORDER BY b.id DESC"

	limitClause := ""
	if filter.WithPagination {
		argCounter := len(args) + 1
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", argCounter, argCounter+1)
		args = append(args, filter.Limit, filter.Offset)
	}

	query := fmt.Sprintf(`
		SELECT
			b.id, b.name, b.short_name, b.address, b.phone_number, b.email, b.email_index,
			b.open_date, b.status_id, b.created_at, b.updated_at,
			COALESCE(s.id, 0), COALESCE(s.name, '')
		FROM branches b
		LEFT JOIN statuses s ON b.status_id = s.id
		%s %s %s
	`, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	branches := make([]entities.Branch, 0)
	for rows.Next() {
		branch, err := r.scanBranch(rows)
		if err != nil {
			return nil, 0, err
		}
		branches = append(branches, *branch)
	}
	return branches, total, rows.Err()
}

func (r *BranchRepository) FindBranch(ctx context.Context, id uint64) (*entities.Branch, error) {
	query := `
		SELECT
			b.id, b.name, b.short_name, b.address, b.phone_number, b.email, b.email_index,
			b.open_date, b.status_id, b.created_at, b.updated_at,
			COALESCE(s.id, 0), COALESCE(s.name, '')
		FROM branches b
		LEFT JOIN statuses s ON b.status_id = s.id
		WHERE b.id = $1
	`
	return r.scanBranch(r.storage.QueryRow(ctx, query, id))
}

func (r *BranchRepository) CreateBranch(ctx context.Context, branch entities.Branch) (*entities.Branch, error) {
	query := `
		INSERT INTO branches (name, short_name, address, phone_number, email, email_index, open_date, status_id)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	var newID uint64
	err := r.storage.QueryRow(ctx, query,
		branch.Name, branch.ShortName, branch.Address, branch.PhoneNumber,
		branch.Email, branch.EmailIndex, branch.OpenDate, branch.StatusID,
	).Scan(&newID)
	if err != nil {
		return nil, err
	}
	return r.FindBranch(ctx, newID)
}

func (r *BranchRepository) UpdateBranch(ctx context.Context, id uint64, branch entities.Branch) (*entities.Branch, error) {
	query := `
		UPDATE branches
		SET name = $1, short_name = $2, address = $3, phone_number = $4, email = $5,
		    email_index = $6, open_date = $7, status_id = $8, updated_at = NOW()
		WHERE id = $9
	`
	result, err := r.storage.Exec(ctx, query,
		branch.Name, branch.ShortName, branch.Address, branch.PhoneNumber,
		branch.Email, branch.EmailIndex, branch.OpenDate, branch.StatusID,
		id)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, apperrors.ErrNotFound
	}
	return r.FindBranch(ctx, id)
}

func (r *BranchRepository) DeleteBranch(ctx context.Context, id uint64) error {
	query := `DELETE FROM branches WHERE id = $1`
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

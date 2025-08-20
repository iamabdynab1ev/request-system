package repositories

import (
	"context"
	"errors"
	"fmt"
	"request-system/internal/entities" // <-- ИЗМЕНЕНИЕ: работаем с entity
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types" // <-- ИЗМЕНЕНИЕ: принимаем types.Filter
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const branchTable = "branches"

// +++ НАЧАЛО НОВОГО БЛОКА: Безопасность и унификация +++
var branchAllowedFilterFields = map[string]bool{
	"status_id": true,
}
var branchAllowedSortFields = map[string]bool{
	"id":         true,
	"name":       true,
	"created_at": true,
	"open_date":  true,
}

// --- КОНЕЦ НОВОГО БЛОКА ---

type BranchRepositoryInterface interface {
	// ИЗМЕНЕНИЕ: Сигнатура метода GetBranches полностью меняется
	GetBranches(ctx context.Context, filter types.Filter) ([]entities.Branch, uint64, error)
	FindBranch(ctx context.Context, id uint64) (*entities.Branch, error)
	CreateBranch(ctx context.Context, dto entities.Branch) (uint64, error)
	UpdateBranch(ctx context.Context, id uint64, dto entities.Branch) error
	DeleteBranch(ctx context.Context, id uint64) error
}

type BranchRepository struct {
	storage *pgxpool.Pool
}

func NewBranchRepository(storage *pgxpool.Pool) BranchRepositoryInterface {
	return &BranchRepository{
		storage: storage,
	}
}

// Новая универсальная функция для сканирования, чтобы не дублировать код
func scanBranch(row pgx.Row) (*entities.Branch, error) {
	var branch entities.Branch
	var status entities.Status
	err := row.Scan(
		&branch.ID, &branch.Name, &branch.ShortName, &branch.Address, &branch.PhoneNumber,
		&branch.Email, &branch.EmailIndex, &branch.OpenDate, &branch.StatusID,
		&branch.CreatedAt, &branch.UpdatedAt,
		&status.ID, &status.Name, // Сканируем также данные статуса
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка сканирования строки филиала: %w", err)
	}
	branch.Status = &status // Присваиваем объект статуса
	return &branch, nil
}

// >>> НАЧАЛО ИЗМЕНЕНИЙ: ПОЛНОСТЬЮ ЗАМЕНЯЕМ СТАРЫЙ МЕТОД GetBranches <<<
func (r *BranchRepository) GetBranches(ctx context.Context, filter types.Filter) ([]entities.Branch, uint64, error) {
	allArgs := make([]interface{}, 0)
	conditions := []string{}
	placeholderNum := 1

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		searchCondition := fmt.Sprintf("(b.name ILIKE $%d OR b.short_name ILIKE $%d OR b.address ILIKE $%d)",
			placeholderNum, placeholderNum, placeholderNum) // Используем один плейсхолдер для оптимизации
		conditions = append(conditions, searchCondition)
		allArgs = append(allArgs, searchPattern)
		placeholderNum++
	}

	for key, value := range filter.Filter {
		if !branchAllowedFilterFields[key] {
			continue // Пропускаем поля, которых нет в "белом списке"
		}

		if strVal, ok := value.(string); ok && strings.Contains(strVal, ",") {
			items := strings.Split(strVal, ",")
			placeholders := make([]string, len(items))
			for i, item := range items {
				placeholders[i] = fmt.Sprintf("$%d", placeholderNum)
				allArgs = append(allArgs, item)
				placeholderNum++
			}
			conditions = append(conditions, fmt.Sprintf("b.%s IN (%s)", key, strings.Join(placeholders, ",")))
		} else {
			// Обработка одного значения
			conditions = append(conditions, fmt.Sprintf("b.%s = $%d", key, placeholderNum))
			allArgs = append(allArgs, value)
			placeholderNum++
		}
	}
	joinClause := fmt.Sprintf("%s b LEFT JOIN %s s ON b.status_id = s.id", branchTable, statusTable)
	var whereClause string
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}
	// >>> ИСПРАВЛЕНИЕ ЗДЕСЬ. Мы говорим "FROM branches b", чтобы PostgreSQL понял, что b - это алиас <<<
	countQuery := fmt.Sprintf("SELECT COUNT(b.id) FROM %s b %s", branchTable, whereClause)
	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, allArgs...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return []entities.Branch{}, 0, nil
	}

	orderByClause := "ORDER BY b.id DESC" // Сортировка по умолчанию
	if len(filter.Sort) > 0 {
		var sortParts []string
		for field, direction := range filter.Sort {
			if branchAllowedSortFields[field] {
				safeDirection := "ASC"
				if strings.ToLower(direction) == "desc" {
					safeDirection = "DESC"
				}
				sortParts = append(sortParts, fmt.Sprintf("b.%s %s", field, safeDirection))
			}
		}
		if len(sortParts) > 0 {
			orderByClause = "ORDER BY " + strings.Join(sortParts, ", ")
		}
	}

	limitClause := ""
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", placeholderNum, placeholderNum+1)
		allArgs = append(allArgs, filter.Limit, filter.Offset)
	}

	selectFields := "b.id, b.name, b.short_name, b.address, b.phone_number, b.email, b.email_index, b.open_date, b.status_id, b.created_at, b.updated_at, s.id, s.name"
	mainQuery := fmt.Sprintf("SELECT %s FROM %s %s %s %s", selectFields, joinClause, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, mainQuery, allArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	branches := make([]entities.Branch, 0)
	for rows.Next() {
		branch, err := scanBranch(rows)
		if err != nil {
			return nil, 0, err
		}
		branches = append(branches, *branch)
	}
	return branches, totalCount, rows.Err()
}

func (r *BranchRepository) FindBranch(ctx context.Context, id uint64) (*entities.Branch, error) {
	query := fmt.Sprintf(`
        SELECT
            b.id, b.name, b.short_name, b.address, b.phone_number, b.email, b.email_index,
            b.open_date, b.status_id, b.created_at, b.updated_at,
            s.id as status_id, s.name as status_name
        FROM %s b LEFT JOIN %s s ON b.status_id = s.id
        WHERE b.id = $1 AND b.deleted_at IS NULL
    `, branchTable, statusTable)

	row := r.storage.QueryRow(ctx, query, id)
	return scanBranch(row)
}

func (r *BranchRepository) CreateBranch(ctx context.Context, branch entities.Branch) (uint64, error) {
	query := `
		INSERT INTO branches (name, short_name, address, phone_number, email, email_index, open_date, status_id) 
		VALUES($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`

	var createdID uint64
	err := r.storage.QueryRow(ctx, query,
		branch.Name, branch.ShortName, branch.Address, branch.PhoneNumber,
		branch.Email, branch.EmailIndex, branch.OpenDate, branch.StatusID,
	).Scan(&createdID)

	if err != nil {
		return 0, fmt.Errorf("ошибка при создании филиала: %w", err)
	}
	return createdID, nil
}

func (r *BranchRepository) UpdateBranch(ctx context.Context, id uint64, branch entities.Branch) error {
	query := `
		UPDATE branches SET 
			name = $1, short_name = $2, address = $3, phone_number = $4, 
			email = $5, email_index = $6, open_date = $7, status_id = $8,
			updated_at = NOW()
		WHERE id = $9 AND deleted_at IS NULL`

	result, err := r.storage.Exec(ctx, query,
		branch.Name, branch.ShortName, branch.Address, branch.PhoneNumber,
		branch.Email, branch.EmailIndex, branch.OpenDate, branch.StatusID,
		id,
	)

	if err != nil {
		return fmt.Errorf("ошибка обновления филиала с id %d: %w", id, err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *BranchRepository) DeleteBranch(ctx context.Context, id uint64) error {
	query := `UPDATE branches SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("ошибка удаления филиала с id %d: %w", id, err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

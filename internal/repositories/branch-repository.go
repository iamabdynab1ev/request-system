// Файл: internal/repositories/branch-repository.go
// СКОПИРУЙТЕ И ПОЛНОСТЬЮ ЗАМЕНИТЕ СОДЕРЖИМОЕ
package repositories

import (
	"context"
	"errors"
	"fmt"
	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const branchTable = "branches"

type BranchRepositoryInterface interface {
	GetBranches(ctx context.Context, limit uint64, offset uint64) ([]dto.BranchDTO, uint64, error)
	FindBranch(ctx context.Context, id uint64) (*dto.BranchDTO, error)
	CreateBranch(ctx context.Context, dto dto.CreateBranchDTO) (uint64, error)
	UpdateBranch(ctx context.Context, id uint64, dto dto.UpdateBranchDTO) error
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

func (r *BranchRepository) GetBranches(ctx context.Context, limit, offset uint64) ([]dto.BranchDTO, uint64, error) {
	var total uint64
	err := r.storage.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", branchTable)).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета филиалов: %w", err)
	}

	if total == 0 {
		return []dto.BranchDTO{}, 0, nil
	}

	query := fmt.Sprintf(`
        SELECT
            b.id, b.name, b.short_name, b.address, b.phone_number, b.email, b.email_index,
            b.open_date, b.created_at, b.updated_at,
            s.id, s.name
        FROM %s b LEFT JOIN %s s ON s.id = b.status_id
        ORDER BY b.id DESC LIMIT $1 OFFSET $2
    `, branchTable, statusTable)

	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка получения списка филиалов: %w", err)
	}
	defer rows.Close()

	var branches []dto.BranchDTO
	for rows.Next() {
		var branch dto.BranchDTO
		var status dto.ShortStatusDTO
		var openDate, createdAt, updatedAt *time.Time

		if err := rows.Scan(
			&branch.ID, &branch.Name, &branch.ShortName, &branch.Address, &branch.PhoneNumber,
			&branch.Email, &branch.EmailIndex, &openDate, &createdAt, &updatedAt,
			&status.ID, &status.Name,
		); err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования строки филиала: %w", err)
		}

		// ИСПРАВЛЕНО: Присваиваем в поле `Status`, а не `StatusID`
		branch.Status = &status
		if createdAt != nil {
			branch.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
		}
		if openDate != nil {
			branch.OpenDate = openDate.Format("2006-01-02 15:04:05")
		}
		if updatedAt != nil {
			branch.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")
		}
		branches = append(branches, branch)
	}
	return branches, total, nil
}

func (r *BranchRepository) FindBranch(ctx context.Context, id uint64) (*dto.BranchDTO, error) {
	query := fmt.Sprintf(`
        SELECT
            b.id, b.name, b.short_name, b.address, b.phone_number, b.email, b.email_index,
            b.open_date, b.created_at, b.updated_at,
            s.id, s.name
        FROM %s b LEFT JOIN %s s ON b.status_id = s.id
        WHERE b.id = $1
    `, branchTable, statusTable)

	var branch dto.BranchDTO
	var status dto.ShortStatusDTO
	var openDate, createdAt, updatedAt *time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&branch.ID, &branch.Name, &branch.ShortName, &branch.Address, &branch.PhoneNumber,
		&branch.Email, &branch.EmailIndex, &openDate, &createdAt, &updatedAt,
		&status.ID, &status.Name,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка поиска филиала по id %d: %w", id, err)
	}

	// ИСПРАВЛЕНО: Присваиваем в поле `Status`, а не `StatusID`
	branch.Status = &status
	if createdAt != nil {
		branch.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
	}
	if openDate != nil {
		branch.OpenDate = openDate.Format("2006-01-02 15:04:05")
	}
	if updatedAt != nil {
		branch.UpdatedAt = updatedAt.Format("2006-01-02 15:04:05")
	}
	return &branch, nil
}

func (r *BranchRepository) CreateBranch(ctx context.Context, dto dto.CreateBranchDTO) (uint64, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s (name, short_name, address, phone_number, email, email_index, open_date, status_id) 
		VALUES($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id
		`, branchTable)

	var createdID uint64
	openDate, err := time.Parse("2006-01-02", dto.OpenDate)
	if err != nil {
		return 0, fmt.Errorf("неверный формат даты: %w", err)
	}

	err = r.storage.QueryRow(ctx, query,
		dto.Name, dto.ShortName, dto.Address, dto.PhoneNumber,
		dto.Email, dto.EmailIndex, openDate, dto.StatusID,
	).Scan(&createdID)
	if err != nil {
		return 0, fmt.Errorf("ошибка при создании филиала: %w", err)
	}
	return createdID, nil
}

func (r *BranchRepository) UpdateBranch(ctx context.Context, id uint64, dto dto.UpdateBranchDTO) error {
	updates := make([]string, 0)
	args := make([]interface{}, 0)
	argID := 1

	if dto.Name != "" {
		updates = append(updates, fmt.Sprintf("name = $%d", argID))
		args = append(args, dto.Name)
		argID++
	}
	if dto.ShortName != "" {
		updates = append(updates, fmt.Sprintf("short_name = $%d", argID))
		args = append(args, dto.ShortName)
		argID++
	}
	if dto.Address != "" {
		updates = append(updates, fmt.Sprintf("address = $%d", argID))
		args = append(args, dto.Address)
		argID++
	}
	if dto.PhoneNumber != "" {
		updates = append(updates, fmt.Sprintf("phone_number = $%d", argID))
		args = append(args, dto.PhoneNumber)
		argID++
	}
	if dto.Email != "" {
		updates = append(updates, fmt.Sprintf("email = $%d", argID))
		args = append(args, dto.Email)
		argID++
	}
	if dto.EmailIndex != "" {
		updates = append(updates, fmt.Sprintf("email_index = $%d", argID))
		args = append(args, dto.EmailIndex)
		argID++
	}
	if dto.OpenDate != "" {
		// ИЗМЕНЕНО: Обновлен формат для парсинга
		openDate, err := time.Parse("2006-01-02", dto.OpenDate)
		if err != nil {
			return fmt.Errorf("неверный формат даты для обновления: %w", err)
		}
		updates = append(updates, fmt.Sprintf("open_date = $%d", argID))
		args = append(args, openDate)
		argID++
	}
	if dto.StatusID != 0 {
		updates = append(updates, fmt.Sprintf("status_id = $%d", argID))
		args = append(args, dto.StatusID)
		argID++
	}

	if len(updates) == 0 {
		return nil
	}
	updates = append(updates, fmt.Sprintf("updated_at = $%d", argID))
	args = append(args, time.Now())
	argID++
	args = append(args, id)

	query := fmt.Sprintf(`UPDATE %s SET %s WHERE id = $%d`, branchTable, strings.Join(updates, ", "), argID)
	result, err := r.storage.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("ошибка обновления филиала с id %d: %w", id, err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *BranchRepository) DeleteBranch(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", branchTable)
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("ошибка удаления филиала с id %d: %w", id, err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

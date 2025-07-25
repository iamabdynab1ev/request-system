package repositories

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const branchTableForJoinFinal = "branches"
const branchFieldsForJoinFinal = "branches.id, branches.name, branches.short_name, branches.address, branches.phone_number, branches.email, branches.email_index, branches.open_date, branches.created_at, branches.updated_at"
const statusTableForBranchFinalRepo = "statuses"
const statusFieldsShortForBranchFinalRepo = "s.id, s.name"

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
	err := r.storage.QueryRow(ctx, `SELECT COUNT(*) FROM branches`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count branches: %w", err)
	}

	rows, err := r.storage.Query(ctx, `
        SELECT b.id, b.name, b.address, s.id AS status_id, s.name AS status_name
        FROM branches b
        LEFT JOIN statuses s ON s.id = b.status_id
        ORDER BY b.id DESC
        LIMIT $1 OFFSET $2
    `, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get branches: %w", err)
	}
	defer rows.Close()

	var branches []dto.BranchDTO
	for rows.Next() {
		var branch dto.BranchDTO
		if err := rows.Scan(
			&branch.ID,
			&branch.Name,
			&branch.Address,
			&branch.Status.ID,
			&branch.Status.Name,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan branch: %w", err)
		}
		branches = append(branches, branch)
	}

	return branches, total, nil
}

func (r *BranchRepository) FindBranch(ctx context.Context, id uint64) (*dto.BranchDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s,
			%s
		FROM %s branches
		LEFT JOIN %s s ON branches.status_id = s.id
		WHERE branches.id = $1
	`,
		orderDocumentFieldsRepo,
		branchFieldsForJoinFinal,
		statusTableForBranchFinalRepo,
		statusFieldsShortForBranchFinalRepo,
	)
	var branch dto.BranchDTO
	var status dto.ShortStatusDTO
	var createdAt, openDate *time.Time
	var updateAt *time.Time
	err := r.storage.QueryRow(ctx, query, id).Scan(
		&branch.ID,
		&branch.Name,
		&branch.ShortName,
		&branch.Address,
		&branch.PhoneNumber,
		&branch.Email,
		&branch.EmailIndex,
		&openDate,
		&createdAt,
		&updateAt,
		&status.ID,
		&status.Name,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка поиска ветки по идентификатору %d: %w", id, err)
	}
	branch.CreatedAt = createdAt.Format("2006-01-02 15:04:05")
	branch.OpenDate = openDate.Format("2006-01-02 15:04:05")
	if updateAt != nil {
		branch.UpdatedAt = updateAt.Format("2006-01-02 15:04:05")
	}
	branch.Status = status
	return &branch, nil
}
func (r *BranchRepository) CreateBranch(ctx context.Context, dto dto.CreateBranchDTO) (uint64, error) {
	query := fmt.Sprintf(`
		INSERT INTO %s (name, short_name, address, phone_number, email, email_index, open_date, status_id) 
		VALUES($1, $2, $3, $4, $5, $6, $7, $8) 
		`, statusTableForBranchFinalRepo)

	var createdID uint64
	openDate, err := time.Parse("2006-01-02 15:04:05", dto.OpenDate)
	if err != nil {
		return 0, fmt.Errorf("invalid open_date format: %w", err)
	}
	err = r.storage.QueryRow(ctx, query,
		dto.Name,
		dto.ShortName,
		dto.Address,
		dto.PhoneNumber,
		dto.Email,
		dto.EmailIndex,
		openDate,
		dto.StatusID,
	).Scan(&createdID)

	if err != nil {
		return 0, fmt.Errorf("ошибка при создании филиала: %w", err)
	}
	return createdID, nil
}
func (r *BranchRepository) UpdateBranch(ctx context.Context, id uint64, dto dto.UpdateBranchDTO) error {
	var openDate time.Time
	var err error
	if dto.OpenDate != "" {
		openDate, err = time.Parse("2006-01-02 15:04:05", dto.OpenDate)
		if err != nil {
			return fmt.Errorf("invalid open_date format for update: %w", err)
		}
	}
	query := fmt.Sprintf(`
		UPDATE %s
		SET name = $1, short_name = $2, address = $3, phone_number = $4, email = $5, email_index = $6, open_date = $7, status_id = $8
		WHERE id = $9
		`,
		statusTableForBranchFinalRepo)

	result, err := r.storage.Exec(ctx, query,
		dto.Name,
		dto.ShortName,
		dto.Address,
		dto.PhoneNumber,
		dto.Email,
		dto.EmailIndex,
		openDate,
		dto.StatusID,
		id,
	)
	if err != nil {
		return fmt.Errorf("error updating branch by id %d: %w", id, err)
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
func (r *BranchRepository) DeleteBranch(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", statusTableForBranchFinalRepo)
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting branch by id %d: %w", id, err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

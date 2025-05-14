package repositories

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	BRANCH_TABLE  = "branches"
	BRANCH_FIELDS = "id, name, short_name, address, phone_number, email, email_index, open_date, status_id, created_at"
)

type BranchRepositoryInterface interface {
	GetBranches(ctx context.Context, limit uint64, offset uint64) ([]dto.BranchDTO, error)
	FindBranch(ctx context.Context, id uint64) (*dto.BranchDTO, error)
	CreateBranch(ctx context.Context, dto dto.CreateBranchDTO) error
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

func (r *BranchRepository) GetBranches(ctx context.Context, limit uint64, offset uint64) ([]dto.BranchDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		`, BRANCH_FIELDS, BRANCH_TABLE)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	branches := make([]dto.BranchDTO, 0)

	for rows.Next() {
		var branch dto.BranchDTO
		var createdAt time.Time
		var openDate time.Time

		err := rows.Scan(
			&branch.ID,
			&branch.Name,
			&branch.ShortName,
			&branch.Address,
			&branch.PhoneNumber,
			&branch.Email,
			&branch.EmailIndex,
            &openDate,
			&branch.StatusID,
			&createdAt,
		)

		if err != nil {
			return nil, err
		}

		branch.CreatedAt = createdAt.Format("2006-01-02, 15:04:05")
        branch.OpenDate = openDate.Format("2006-01-02, 15:04:05") // Форматируем open_date

		branches = append(branches, branch)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return branches, nil
}

func (r *BranchRepository) FindBranch(ctx context.Context, id uint64) (*dto.BranchDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		WHERE r.id = $1
	`, BRANCH_FIELDS, BRANCH_TABLE)

	var branch dto.BranchDTO
	var createdAt time.Time
    var openDate time.Time

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&branch.ID,
		&branch.Name,
		&branch.ShortName,
		&branch.Address,
		&branch.PhoneNumber,
		&branch.Email,
		&branch.EmailIndex,
        &openDate,
		&branch.StatusID,
		&createdAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, err
	}

	branch.CreatedAt = createdAt.Format("2006-01-02, 15:04:05")
    branch.OpenDate = openDate.Format("2006-01-02, 15:04:05") // Форматируем open_date

	return &branch, nil
}

func (r *BranchRepository) CreateBranch(ctx context.Context, dto dto.CreateBranchDTO) error {
	query := fmt.Sprintf("INSERT INTO %s (name, short_name, address, phone_number, email, email_index, open_date, status_id) VALUES($1, $2, $3, $4, $5, $6, $7, $8)", BRANCH_TABLE)

	_, err := r.storage.Exec(ctx, query,
        dto.Name, dto.ShortName, dto.Address, dto.PhoneNumber,
        dto.Email, dto.EmailIndex, dto.OpenDate, dto.StatusID,
    )
	if err != nil {
		return err
	}

	return nil
}

func (r *BranchRepository) UpdateBranch(ctx context.Context, id uint64, dto dto.UpdateBranchDTO) error {
	query := fmt.Sprintf("UPDATE %s SET name = $1, short_name = $2, address = $3, phone_number = $4, email = $5, email_index = $6, open_date = $7, status_id = $8 WHERE id = $9", BRANCH_TABLE)

	result, err := r.storage.Exec(ctx, query,
        dto.Name, dto.ShortName, dto.Address, dto.PhoneNumber,
        dto.Email, dto.EmailIndex, dto.OpenDate, dto.StatusID, id,
    )
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}

func (r *BranchRepository) DeleteBranch(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", BRANCH_TABLE)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}
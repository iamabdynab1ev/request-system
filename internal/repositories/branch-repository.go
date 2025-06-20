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
	BRANCH_TABLE_FOR_JOIN_FINAL               = "branches"
	BRANCH_FIELDS_FOR_JOIN_FINAL              = "branches.id, branches.name, branches.short_name, branches.address, branches.phone_number, branches.email, branches.email_index, branches.open_date, branches.created_at, branches.updated_at"
	STATUS_TABLE_FOR_BRANCH_FINAL_REPO        = "statuses"
	STATUS_FIELDS_SHORT_FOR_BRANCH_FINAL_REPO = "s.id, s.name"
)

type BranchRepositoryInterface interface {
	GetBranches(ctx context.Context, limit uint64, offset uint64) (interface{}, error)
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

func (r *BranchRepository) GetBranches(ctx context.Context, limit uint64, offset uint64) (interface{}, error) {

	data, _, err := FetchDataAndCount(ctx, r.storage, Params{
		Table:   "branches",
		Columns: "branches.*, status.id AS status_id, status.name AS status_name",
		Relations: []Join{
			{Table: "statuses", Alias: "status", OnLeft: "branches.status_id", OnRight: "status.id", JoinType: "LEFT"}},
		WithPg: true,
		Limit:  limit,
		Offset: offset,
	})

	return data, err
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
		BRANCH_FIELDS_FOR_JOIN_FINAL,
		STATUS_FIELDS_SHORT_FOR_BRANCH_FINAL_REPO,
		BRANCH_TABLE_FOR_JOIN_FINAL,
		STATUS_TABLE_FOR_BRANCH_FINAL_REPO,
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
			return nil, utils.ErrorNotFound
		}
		return nil, fmt.Errorf("Oшибка поиска ветки по идентификатору %d: %w", id, err)
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
		`, BRANCH_TABLE_FOR_JOIN_FINAL)

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
		BRANCH_TABLE_FOR_JOIN_FINAL)

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
		return utils.ErrorNotFound
	}

	return nil
}

func (r *BranchRepository) DeleteBranch(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", BRANCH_TABLE_FOR_JOIN_FINAL)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting branch by id %d: %w", id, err)
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}

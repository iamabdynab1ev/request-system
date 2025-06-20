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
	OFFICE_TABLE_FOR_JOIN_FINAL  = "offices"
	OFFICE_FIELDS_FOR_JOIN_FINAL = "o.id, o.name, o.address, o.open_date, o.branches_id, o.status_id, o.created_at, o.updated_at"

	BRANCH_TABLE_FOR_OFFICE_JOIN_FINAL_REPO        = "branches"
	BRANCH_FIELDS_SHORT_FOR_OFFICE_JOIN_FINAL_REPO = "b.id, b.name, b.short_name"

	STATUS_TABLE_FOR_OFFICE_FINAL_REPO        = "statuses"
	STATUS_FIELDS_SHORT_FOR_OFFICE_FINAL_REPO = "s.id, s.name"
)

type OfficeRepositoryInterface interface {
	GetOffices(ctx context.Context, limit uint64, offset uint64) ([]dto.OfficeDTO, error)
	FindOffice(ctx context.Context, id uint64) (*dto.OfficeDTO, error)
	CreateOffice(ctx context.Context, dto dto.CreateOfficeDTO) error
	UpdateOffice(ctx context.Context, id uint64, dto dto.UpdateOfficeDTO) error
	DeleteOffice(ctx context.Context, id uint64) error
}

type OfficeRepository struct {
	storage *pgxpool.Pool
}

func NewOfficeRepository(storage *pgxpool.Pool) OfficeRepositoryInterface {
	return &OfficeRepository{
		storage: storage,
	}
}

func (r *OfficeRepository) GetOffices(ctx context.Context, limit uint64, offset uint64) ([]dto.OfficeDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s,
			%s,
            %s
		FROM %s o
		LEFT JOIN %s b ON o.branches_id = b.id
        LEFT JOIN %s s ON o.status_id = s.id
		`,
		OFFICE_FIELDS_FOR_JOIN_FINAL,
		BRANCH_FIELDS_SHORT_FOR_OFFICE_JOIN_FINAL_REPO,
		STATUS_FIELDS_SHORT_FOR_OFFICE_FINAL_REPO,
		OFFICE_TABLE_FOR_JOIN_FINAL,
		BRANCH_TABLE_FOR_OFFICE_JOIN_FINAL_REPO,
		STATUS_TABLE_FOR_OFFICE_FINAL_REPO,
	)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	offices := make([]dto.OfficeDTO, 0, 20)

	for rows.Next() {
		var office dto.OfficeDTO
		var branch dto.ShortBranchDTO
		var status dto.ShortStatusDTO

		var openDate time.Time
		var createdAt time.Time
		var updatedAt time.Time
		var branchesIdFromOfficeTable int
		var statusIdFromOfficeTable int

		err := rows.Scan(
			&office.ID,
			&office.Name,
			&office.Address,
			&openDate,

			&branchesIdFromOfficeTable,
			&statusIdFromOfficeTable,

			&createdAt,
			&updatedAt,

			&branch.ID,
			&branch.Name,
			&branch.ShortName,

			&status.ID,
			&status.Name,
		)

		if err != nil {
			return nil, err
		}

		office.OpenDate = openDate.Format("2006-01-02")
		office.CreatedAt = createdAt.Format("2006-01-02, 15:04:05")
		office.UpdatedAt = updatedAt.Format("2006-01-02, 15:04:05")

		office.Branch = branch
		office.Status = status

		offices = append(offices, office)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return offices, nil
}

func (r *OfficeRepository) FindOffice(ctx context.Context, id uint64) (*dto.OfficeDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s,
            %s,
			%s
		FROM %s o
		LEFT JOIN %s b ON o.branches_id = b.id
		LEFT JOIN %s s ON o.status_id = s.id
		WHERE o.id = $1
	`,
		OFFICE_FIELDS_FOR_JOIN_FINAL,
		BRANCH_FIELDS_SHORT_FOR_OFFICE_JOIN_FINAL_REPO,
		STATUS_FIELDS_SHORT_FOR_OFFICE_FINAL_REPO,
		OFFICE_TABLE_FOR_JOIN_FINAL,
		BRANCH_TABLE_FOR_OFFICE_JOIN_FINAL_REPO,
		STATUS_TABLE_FOR_OFFICE_FINAL_REPO,
	)

	var office dto.OfficeDTO
	var branch dto.ShortBranchDTO
	var status dto.ShortStatusDTO
	var openDate time.Time
	var createdAt time.Time
	var updatedAt time.Time
	var branchesIdFromOfficeTable int
	var statusIdFromOfficeTable int

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&office.ID,
		&office.Name,
		&office.Address,
		&openDate,
		&branchesIdFromOfficeTable,
		&statusIdFromOfficeTable,

		&createdAt,
		&updatedAt,

		&branch.ID,
		&branch.Name,
		&branch.ShortName,

		&status.ID,
		&status.Name,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, err
	}

	office.OpenDate = openDate.Format("2006-01-02")
	office.CreatedAt = createdAt.Format("2006-01-02, 15:04:05")
	office.UpdatedAt = updatedAt.Format("2006-01-02, 15:04:05")
	office.Branch = branch
	office.Status = status

	return &office, nil
}

func (r *OfficeRepository) CreateOffice(ctx context.Context, dto dto.CreateOfficeDTO) error {
	query := fmt.Sprintf(`
        INSERT INTO %s (name, address, open_date, branches_id, status_id)
        VALUES ($1, $2, $3, $4, $5)
    `, OFFICE_TABLE_FOR_JOIN_FINAL)

	openDate, err := time.Parse("2006-01-02", dto.OpenDate)
	if err != nil {
		return fmt.Errorf("invalid open_date format: %w", err)
	}

	_, err = r.storage.Exec(ctx, query,
		dto.Name,
		dto.Address,
		openDate,
		dto.BranchesID,
		dto.StatusID,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *OfficeRepository) UpdateOffice(ctx context.Context, id uint64, dto dto.UpdateOfficeDTO) error {
	var parsedOpenDate time.Time
	var err error

	if dto.OpenDate != "" {
		parsedOpenDate, err = time.Parse("2006-01-02", dto.OpenDate)
		if err != nil {
			return fmt.Errorf("invalid open_date format: %w", err)
		}
	}

	query := fmt.Sprintf(`
        UPDATE %s
        SET name = $1, address = $2, open_date = $3, branches_id = $4, status_id = $5, updated_at = CURRENT_TIMESTAMP
        WHERE id = $6
    `, OFFICE_TABLE_FOR_JOIN_FINAL)

	result, err := r.storage.Exec(ctx, query,
		dto.Name,
		dto.Address,
		parsedOpenDate,
		dto.BranchesID,
		dto.StatusID,
		id,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}
	return nil
}

func (r *OfficeRepository) DeleteOffice(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", OFFICE_TABLE_FOR_JOIN_FINAL)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}

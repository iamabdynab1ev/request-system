// Файл: internal/repositories/office-repository.go
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

const officeTable = "offices"

type OfficeRepositoryInterface interface {
	GetOffices(ctx context.Context, limit uint64, offset uint64) ([]dto.OfficeDTO, uint64, error)
	FindOffice(ctx context.Context, id uint64) (*dto.OfficeDTO, error)
	CreateOffice(ctx context.Context, dto dto.CreateOfficeDTO) (uint64, error)
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

func (r *OfficeRepository) GetOffices(ctx context.Context, limit uint64, offset uint64) ([]dto.OfficeDTO, uint64, error) {
	var total uint64
	err := r.storage.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", officeTable)).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета офисов: %w", err)
	}

	if total == 0 {
		return []dto.OfficeDTO{}, 0, nil
	}

	query := fmt.Sprintf(`
		SELECT
			o.id, o.name, o.address, o.open_date, o.created_at, o.updated_at,
			b.id, b.name, b.short_name,
            s.id, s.name
		FROM %s o
		LEFT JOIN %s b ON o.branch_id = b.id
        LEFT JOIN %s s ON o.status_id = s.id
		ORDER BY o.id DESC LIMIT $1 OFFSET $2
		`, officeTable, branchTable, statusTable)

	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var offices []dto.OfficeDTO
	for rows.Next() {
		var office dto.OfficeDTO
		var branch dto.ShortBranchDTO
		var status dto.ShortStatusDTO
		if err := rows.Scan(
			&office.ID, &office.Name, &office.Address, &office.OpenDate, &office.CreatedAt, &office.UpdatedAt,
			&branch.ID, &branch.Name, &branch.ShortName,
			&status.ID, &status.Name,
		); err != nil {
			return nil, 0, err
		}
		office.Branch = &branch
		office.Status = &status
		offices = append(offices, office)
	}
	return offices, total, nil
}

func (r *OfficeRepository) FindOffice(ctx context.Context, id uint64) (*dto.OfficeDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			o.id, o.name, o.address, o.open_date, o.created_at, o.updated_at,
			b.id, b.name, b.short_name,
			s.id, s.name
		FROM %s o
		LEFT JOIN %s b ON o.branch_id = b.id
		LEFT JOIN %s s ON o.status_id = s.id
		WHERE o.id = $1
	`, officeTable, branchTable, statusTable)

	var office dto.OfficeDTO
	var branch dto.ShortBranchDTO
	var status dto.ShortStatusDTO
	err := r.storage.QueryRow(ctx, query, id).Scan(
		&office.ID, &office.Name, &office.Address, &office.OpenDate, &office.CreatedAt, &office.UpdatedAt,
		&branch.ID, &branch.Name, &branch.ShortName,
		&status.ID, &status.Name,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	office.Branch = &branch
	office.Status = &status
	return &office, nil
}

func (r *OfficeRepository) CreateOffice(ctx context.Context, dto dto.CreateOfficeDTO) (uint64, error) {
	query := fmt.Sprintf(`
        INSERT INTO %s (name, address, open_date, branch_id, status_id)
        VALUES ($1, $2, $3, $4, $5) RETURNING id
    `, officeTable)

	openDate, err := time.Parse("2006-01-02", dto.OpenDate)
	if err != nil {
		return 0, fmt.Errorf("invalid open_date format: %w", err)
	}

	var newID uint64
	err = r.storage.QueryRow(ctx, query, dto.Name, dto.Address, openDate, dto.BranchID, dto.StatusID).Scan(&newID)
	if err != nil {
		return 0, err
	}
	return newID, nil
}

func (r *OfficeRepository) UpdateOffice(ctx context.Context, id uint64, dto dto.UpdateOfficeDTO) error {
	updates := make([]string, 0)
	args := make([]interface{}, 0)
	argID := 1

	if dto.Name != "" {
		updates = append(updates, fmt.Sprintf("name = $%d", argID))
		args = append(args, dto.Name)
		argID++
	}
	if dto.Address != "" {
		updates = append(updates, fmt.Sprintf("address = $%d", argID))
		args = append(args, dto.Address)
		argID++
	}
	if dto.BranchID != 0 {
		updates = append(updates, fmt.Sprintf("branch_id = $%d", argID))
		args = append(args, dto.BranchID)
		argID++
	}
	if dto.StatusID != 0 {
		updates = append(updates, fmt.Sprintf("status_id = $%d", argID))
		args = append(args, dto.StatusID)
		argID++
	}
	if dto.OpenDate != "" {
		openDate, err := time.Parse("2006-01-02", dto.OpenDate)
		if err != nil {
			return fmt.Errorf("invalid open_date format: %w", err)
		}
		updates = append(updates, fmt.Sprintf("open_date = $%d", argID))
		args = append(args, openDate)
		argID++
	}

	if len(updates) == 0 {
		return nil
	}
	updates = append(updates, fmt.Sprintf("updated_at = $%d", argID))
	args = append(args, time.Now())
	argID++
	args = append(args, id)

	query := fmt.Sprintf(`UPDATE %s SET %s WHERE id = $%d`, officeTable, strings.Join(updates, ", "), argID)
	result, err := r.storage.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *OfficeRepository) DeleteOffice(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", officeTable)
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

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
	PERMISSION_TABLE  = "permissions"
	PERMISSION_FIELDS = "id, name, description, created_at, updated_at"
)

type PermissionRepositoryInterface interface {
	GetPermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.PermissionDTO, error)
	FindPermission(ctx context.Context, id uint64) (*dto.PermissionDTO, error)
	CreatePermission(ctx context.Context, dto dto.CreatePermissionDTO) error
	UpdatePermission(ctx context.Context, id uint64, dto dto.UpdatePermissionDTO) error
	DeletePermission(ctx context.Context, id uint64) error
}

type PermissionRepository struct{
	storage *pgxpool.Pool
}

func NewPermissionRepository(storage *pgxpool.Pool) PermissionRepositoryInterface {

	return &PermissionRepository{
		storage: storage,
	}
}

func (r *PermissionRepository) GetPermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.PermissionDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		`, PERMISSION_FIELDS, PERMISSION_TABLE)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	permissions := make([]dto.PermissionDTO, 0)

	for rows.Next() {
		var permission dto.PermissionDTO
		var createdAt time.Time
        var updatedAt time.Time

		err := rows.Scan(
			&permission.ID,
			&permission.Name,
            &permission.Description,
			&createdAt,
            &updatedAt,
		)

		if err != nil {
			return nil, err
		}

        createdAtLocal := createdAt.Local()
       

		permission.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
        

		permissions = append(permissions, permission)
	}

	if err:= rows.Err(); err != nil {
		return nil, err
	}
	return permissions, nil
}

func (r *PermissionRepository) FindPermission(ctx context.Context, id uint64) (*dto.PermissionDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		WHERE r.id = $1
	`, PERMISSION_FIELDS, PERMISSION_TABLE)

	var permission dto.PermissionDTO
	var createdAt time.Time
    var updatedAt time.Time


	err := r.storage.QueryRow(ctx, query, id).Scan(
		&permission.ID,
		&permission.Name,
        &permission.Description,
		&createdAt,
        &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, err
	}

    createdAtLocal := createdAt.Local()
   

	permission.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
   
	return &permission, nil
}

func (r *PermissionRepository) CreatePermission(ctx context.Context, dto dto.CreatePermissionDTO) error {
	query := fmt.Sprintf(`
        INSERT INTO %s (name, description)
        VALUES ($1, $2)
    `, PERMISSION_TABLE)

	_, err := r.storage.Exec(ctx, query,
		dto.Name,
        dto.Description,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *PermissionRepository) UpdatePermission(ctx context.Context, id uint64, dto dto.UpdatePermissionDTO) error {
	query := fmt.Sprintf(`
        UPDATE %s
        SET name = $1, description = $2, updated_at = CURRENT_TIMESTAMP
        WHERE id = $3
    `, PERMISSION_TABLE)

	result, err := r.storage.Exec(ctx, query,
		dto.Name,
        dto.Description,
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

func (r *PermissionRepository) DeletePermission(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", PERMISSION_TABLE)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}
package repositories

import (
	"context"
	"errors"
	"fmt"

	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"

	// Добавляем импорт
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	PermissionTable  = "permissions"
	PermissionFields = "id, name, description, created_at, updated_at"
)

type PermissionRepositoryInterface interface {
	GetPermissions(ctx context.Context, limit uint64, offset uint64, search string) ([]dto.PermissionDTO, uint64, error)
	FindPermissionByID(ctx context.Context, id uint64) (*dto.PermissionDTO, error)
	CreatePermission(ctx context.Context, dto dto.CreatePermissionDTO) (*dto.PermissionDTO, error)
	UpdatePermission(ctx context.Context, id uint64, dto dto.UpdatePermissionDTO) (*dto.PermissionDTO, error)
	DeletePermission(ctx context.Context, id uint64) error
	GetPermissionsNamesByRoleID(ctx context.Context, roleID uint64) ([]string, error)
}

type PermissionRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewPermissionRepository(storage *pgxpool.Pool, logger *zap.Logger) PermissionRepositoryInterface {
	return &PermissionRepository{
		storage: storage,
		logger:  logger,
	}
}

func (r *PermissionRepository) GetPermissionsNamesByRoleID(ctx context.Context, roleID uint64) ([]string, error) {
	query := `SELECT p.name FROM public.permissions p JOIN public.role_permissions rp ON p.id = rp.permission_id WHERE rp.role_id = $1`
	rows, err := r.storage.Query(ctx, query, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (r *PermissionRepository) GetPermissions(ctx context.Context, limit uint64, offset uint64, search string) ([]dto.PermissionDTO, uint64, error) {
	var total uint64
	var args []interface{}
	whereClause := ""

	if search != "" {
		whereClause = "WHERE name ILIKE $1 OR description ILIKE $1"
		args = append(args, "%"+search+"%")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", PermissionTable, whereClause)
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета привилегий: %w", err)
	}

	if total == 0 {
		return []dto.PermissionDTO{}, 0, nil
	}

	query := fmt.Sprintf(`SELECT %s FROM %s %s ORDER BY id LIMIT $%d OFFSET $%d`,
		PermissionFields, PermissionTable, whereClause, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var permissions []dto.PermissionDTO
	for rows.Next() {
		var p dto.PermissionDTO
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan permission: %w", err)
		}
		permissions = append(permissions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return permissions, total, nil
}

func (r *PermissionRepository) FindPermissionByID(ctx context.Context, id uint64) (*dto.PermissionDTO, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE id = $1`, PermissionFields, PermissionTable)
	var p dto.PermissionDTO
	err := r.storage.QueryRow(ctx, query, id).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *PermissionRepository) CreatePermission(ctx context.Context, req dto.CreatePermissionDTO) (*dto.PermissionDTO, error) {
	query := fmt.Sprintf(`INSERT INTO %s (name, description) VALUES ($1, $2) RETURNING %s`, PermissionTable, PermissionFields)
	var p dto.PermissionDTO
	err := r.storage.QueryRow(ctx, query, req.Name, req.Description).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	return &p, err
}

func (r *PermissionRepository) UpdatePermission(ctx context.Context, id uint64, req dto.UpdatePermissionDTO) (*dto.PermissionDTO, error) {
	query := fmt.Sprintf(`UPDATE %s SET name = $1, description = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3 RETURNING %s`, PermissionTable, PermissionFields)
	var p dto.PermissionDTO
	err := r.storage.QueryRow(ctx, query, req.Name, req.Description, id).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *PermissionRepository) DeletePermission(ctx context.Context, id uint64) error {
	tx, err := r.storage.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err = tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			r.logger.Error("ошибка при откате транзакции", zap.Error(err))
		}
	}()

	if _, err = tx.Exec(ctx, "DELETE FROM public.role_permissions WHERE permission_id = $1", id); err != nil {
		return err
	}
	result, err := tx.Exec(ctx, "DELETE FROM permissions WHERE id = $1", id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return tx.Commit(ctx)
}

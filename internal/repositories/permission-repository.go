package repositories

import (
	"context"
	"errors"
	"fmt"
	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const PermissionTable = "permissions"
const PermissionFields = "id, name, description, created_at, updated_at"

type PermissionRepositoryInterface interface {
	GetPermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.PermissionDTO, uint64, error)
	FindPermissionByID(ctx context.Context, id uint64) (*dto.PermissionDTO, error)
	CreatePermission(ctx context.Context, dto dto.CreatePermissionDTO) (*dto.PermissionDTO, error)
	UpdatePermission(ctx context.Context, id uint64, dto dto.UpdatePermissionDTO) (*dto.PermissionDTO, error)
	DeletePermission(ctx context.Context, id uint64) error
	BeginTx(ctx context.Context) (pgx.Tx, error)

	// НОВОЕ: Метод для получения списка имен привилегий по ID роли
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
	query := `
        SELECT p.name
        FROM public.permissions p
        JOIN public.role_permissions rp ON p.id = rp.permission_id
        WHERE rp.role_id = $1
    `
	rows, err := r.storage.Query(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса имен привилегий для роли %d: %w", roleID, err)
	}
	defer rows.Close()

	var permissions []string // Слайс для хранения названий привилегий (строк)
	for rows.Next() {
		var permName string
		if err := rows.Scan(&permName); err != nil {
			return nil, fmt.Errorf("ошибка сканирования имени привилегии: %w", err)
		}
		permissions = append(permissions, permName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка после итерации по именам привилегий: %w", err)
	}
	return permissions, nil
}

func (r *PermissionRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

func (r *PermissionRepository) GetPermissions(ctx context.Context, limit uint64, offset uint64) ([]dto.PermissionDTO, uint64, error) {
	var total uint64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", PermissionTable)
	if err := r.storage.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета привилегий: %w", err)
	}

	query := fmt.Sprintf(`SELECT %s FROM %s ORDER BY id LIMIT $1 OFFSET $2`, PermissionFields, PermissionTable)
	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	permissions := make([]dto.PermissionDTO, 0)
	for rows.Next() {
		var permission dto.PermissionDTO
		var createdAt, updatedAt time.Time
		err := rows.Scan(&permission.ID, &permission.Name, &permission.Description, &createdAt, &updatedAt)
		if err != nil {
			return nil, 0, err
		}

		permissions = append(permissions, permission)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return permissions, total, nil
}

func (r *PermissionRepository) FindPermissionByID(ctx context.Context, id uint64) (*dto.PermissionDTO, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s u WHERE u.id = $1`, PermissionFields, PermissionTable)
	var permission dto.PermissionDTO
	var createdAt, updatedAt time.Time
	err := r.storage.QueryRow(ctx, query, id).Scan(&permission.ID, &permission.Name, &permission.Description, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	return &permission, nil
}

func (r *PermissionRepository) CreatePermission(ctx context.Context, req dto.CreatePermissionDTO) (*dto.PermissionDTO, error) {
	query := fmt.Sprintf(`INSERT INTO %s (name, description) VALUES ($1, $2) RETURNING %s`, PermissionTable, PermissionFields)
	var permission dto.PermissionDTO
	var createdAt, updatedAt time.Time
	err := r.storage.QueryRow(ctx, query, req.Name, req.Description).Scan(&permission.ID, &permission.Name, &permission.Description, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	return &permission, nil
}

func (r *PermissionRepository) UpdatePermission(ctx context.Context, id uint64, req dto.UpdatePermissionDTO) (*dto.PermissionDTO, error) {
	query := fmt.Sprintf(`UPDATE %s SET name = $1, description = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3 RETURNING %s`, PermissionTable, PermissionFields)
	var permission dto.PermissionDTO
	var createdAt, updatedAt time.Time
	err := r.storage.QueryRow(ctx, query, req.Name, req.Description, id).Scan(&permission.ID, &permission.Name, &permission.Description, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &permission, nil
}

func (r *PermissionRepository) DeletePermission(ctx context.Context, id uint64) error {
	tx, err := r.storage.Begin(ctx)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
			r.logger.Error("Ошибка при откате транзакции", zap.Error(rbErr))
		}
	}()

	_, err = tx.Exec(ctx, "DELETE FROM public.role_permissions WHERE permission_id = $1", id)
	if err != nil {
		return fmt.Errorf("ошибка удаления связанных записей из public.role_permissions: %w", err)
	}

	result, err := tx.Exec(ctx, "DELETE FROM permissions WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("ошибка удаления привилегии: %w", err)
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return tx.Commit(ctx)
}

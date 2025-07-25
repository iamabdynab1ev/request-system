// Package repositories предоставляет доступ к базе данных и содержит репозитории для работы с сущностью Permission.
// В этом пакете реализованы CRUD-операции, пагинация, поиск по ID и безопасное удаление с использованием транзакций.
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
}

type PermissionRepository struct {
	storage *pgxpool.Pool
}

func NewPermissionRepository(storage *pgxpool.Pool) PermissionRepositoryInterface {
	return &PermissionRepository{
		storage: storage,
	}
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

		permission.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
		permission.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
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

	permission.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	permission.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
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
	permission.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	permission.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")

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
	permission.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	permission.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")

	return &permission, nil
}

func (r *PermissionRepository) DeletePermission(ctx context.Context, id uint64) error {
	tx, err := r.storage.Begin(ctx)
	if err != nil {
		return fmt.Errorf("не удалось начать транзакцию: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "DELETE FROM role_permissions WHERE permission_id = $1", id)
	if err != nil {
		return fmt.Errorf("ошибка удаления связанных записей из role_permissions: %w", err)
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

package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"

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
	FindPermissionByName(ctx context.Context, name string) (*dto.PermissionDTO, error)
	GetAllUserPermissionsNames(ctx context.Context, userID uint64) ([]string, error)
	GetAllPermissionSourcesForUser(ctx context.Context, userID uint64) ([]dto.PermissionSource, error)
	GetFinalUserPermissionIDs(ctx context.Context, userID uint64) ([]uint64, error)
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

func (r *PermissionRepository) FindPermissionByName(ctx context.Context, name string) (*dto.PermissionDTO, error) {
	query := `SELECT id, name, description, created_at, updated_at FROM permissions WHERE name = $1 LIMIT 1`
	var p dto.PermissionDTO
	err := r.storage.QueryRow(ctx, query, name).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// Этот метод нужен для быстрой авторизации
func (r *PermissionRepository) GetAllUserPermissionsNames(ctx context.Context, userID uint64) ([]string, error) {
	query := `
		SELECT p.name FROM permissions p WHERE p.id IN (
			SELECT permission_id FROM user_permissions WHERE user_id = $1
			UNION
			SELECT rp.permission_id FROM role_permissions rp
			JOIN user_roles ur ON rp.role_id = ur.role_id WHERE ur.user_id = $1
		) AND p.id NOT IN (
			SELECT permission_id FROM user_permission_denials WHERE user_id = $1
		)
	`
	// --- ЛОГИРОВАНИЕ ---
	r.logger.Info("Executing GetAllUserPermissionsNames",
		zap.String("query", query),
		zap.Uint64("userID", userID),
	)
	// --- КОНЕЦ ЛОГИРОВАНИЯ ---

	rows, err := r.storage.Query(ctx, query, userID)
	if err != nil {
		r.logger.Error("Ошибка при выполнении GetAllUserPermissionsNames", zap.Uint64("userID", userID), zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	permissions, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		r.logger.Error("Ошибка сканирования финального списка прав", zap.Uint64("userID", userID), zap.Error(err))
		return nil, err
	}

	// --- ЛОГИРОВАНИЕ РЕЗУЛЬТАТА ---
	r.logger.Info("GetAllUserPermissionsNames result",
		zap.Uint64("userID", userID),
		zap.Int("permissions_count", len(permissions)), // <-- Добавим количество
		zap.Strings("permissions", permissions),
	)
	// --- КОНЕЦ ЛОГИРОВАНИЯ ---

	return permissions, nil
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

	query := fmt.Sprintf(`SELECT %s FROM %s %s ORDER BY id`, PermissionFields, PermissionTable, whereClause)
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}

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
	if err != nil {
		r.logger.Error("Ошибка создания привилегии", zap.Error(err))
		return nil, err
	}
	return &p, nil
}

func (r *PermissionRepository) UpdatePermission(ctx context.Context, id uint64, req dto.UpdatePermissionDTO) (*dto.PermissionDTO, error) {
	var sb strings.Builder
	args := make([]interface{}, 0)
	argID := 1

	sb.WriteString("UPDATE permissions SET ")
	if req.Name != "" {
		sb.WriteString(fmt.Sprintf("name = $%d, ", argID))
		args = append(args, req.Name)
		argID++
	}
	if req.Description != "" {
		sb.WriteString(fmt.Sprintf("description = $%d, ", argID))
		args = append(args, req.Description)
		argID++
	}
	if len(args) == 0 {
		return r.FindPermissionByID(ctx, id)
	}
	sb.WriteString(fmt.Sprintf("updated_at = $%d ", argID))
	args = append(args, time.Now())
	argID++
	sb.WriteString(fmt.Sprintf("WHERE id = $%d RETURNING %s", argID, PermissionFields))
	args = append(args, id)

	var p dto.PermissionDTO
	err := r.storage.QueryRow(ctx, sb.String(), args...).Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		r.logger.Error("Ошибка обновления привилегии", zap.Uint64("id", id), zap.Error(err))
		return nil, err
	}
	return &p, nil
}

func (r *PermissionRepository) DeletePermission(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", PermissionTable)
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		r.logger.Error("Ошибка удаления привилегии", zap.Uint64("id", id), zap.Error(err))
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *PermissionRepository) GetAllPermissionSourcesForUser(ctx context.Context, userID uint64) ([]dto.PermissionSource, error) {
	query := `
		SELECT rp.permission_id, 'role' AS source
		FROM role_permissions rp
		JOIN user_roles ur ON rp.role_id = ur.role_id
		WHERE ur.user_id = $1
		UNION ALL
		SELECT up.permission_id, 'direct' AS source
		FROM user_permissions up
		WHERE up.user_id = $1
		UNION ALL
		SELECT upd.permission_id, 'denied' AS source
		FROM user_permission_denials upd
		WHERE upd.user_id = $1;
	`

	rows, err := r.storage.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []dto.PermissionSource
	for rows.Next() {
		var ps dto.PermissionSource
		if err := rows.Scan(&ps.PermissionID, &ps.Source); err != nil {
			return nil, fmt.Errorf("scan permission source: %w", err)
		}
		results = append(results, ps)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (r *PermissionRepository) GetFinalUserPermissionIDs(ctx context.Context, userID uint64) ([]uint64, error) {
	query := `
		SELECT p.id FROM permissions p WHERE p.id IN (
			SELECT permission_id FROM user_permissions WHERE user_id = $1
			UNION
			SELECT rp.permission_id FROM role_permissions rp
			JOIN user_roles ur ON rp.role_id = ur.role_id WHERE ur.user_id = $1
		) AND p.id NOT IN (
			SELECT permission_id FROM user_permission_denials WHERE user_id = $1
		)
	`
	rows, err := r.storage.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowTo[uint64])
}

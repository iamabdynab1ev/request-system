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

type RoleRepositoryInterface interface {
	GetRoles(ctx context.Context, limit uint64, offset uint64) ([]dto.RoleDTO, uint64, error)
	FindRoleByID(ctx context.Context, id uint64) (*dto.RoleDTO, error)
	CreateRoleInTx(ctx context.Context, tx pgx.Tx, dto dto.CreateRoleDTO) (int, error)
	UpdateRoleInTx(ctx context.Context, tx pgx.Tx, id uint64, dto dto.UpdateRoleDTO) error
	LinkPermissionsToRoleInTx(ctx context.Context, tx pgx.Tx, roleID int, permissionIDs []int) error
	UnlinkAllPermissionsFromRoleInTx(ctx context.Context, tx pgx.Tx, roleID uint64) error
	DeleteRole(ctx context.Context, id uint64) error
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type RoleRepository struct {
	storage *pgxpool.Pool
}

func NewRoleRepository(storage *pgxpool.Pool) RoleRepositoryInterface {
	return &RoleRepository{
		storage: storage,
	}
}

func (r *RoleRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

func (r *RoleRepository) GetRoles(ctx context.Context, limit uint64, offset uint64) ([]dto.RoleDTO, uint64, error) {
	var total uint64
	err := r.storage.QueryRow(ctx, "SELECT COUNT(*) FROM roles").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета ролей: %w", err)
	}

	query := `SELECT id, name, description, status_id, created_at, updated_at FROM roles ORDER BY id LIMIT $1 OFFSET $2`
	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка получения списка ролей: %w", err)
	}
	defer rows.Close()

	roles := make([]dto.RoleDTO, 0)
	for rows.Next() {
		var role dto.RoleDTO
		var createdAt, updatedAt time.Time
		err = rows.Scan(
			&role.ID,
			&role.Name,
			&role.Description,
			&role.StatusID,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования строки роли: %w", err)
		}
		role.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
		role.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
		roles = append(roles, role)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("ошибка после итерации по ролям: %w", err)
	}

	return roles, total, nil
}

func (r *RoleRepository) FindRoleByID(ctx context.Context, id uint64) (*dto.RoleDTO, error) {
	role := &dto.RoleDTO{}
	var createdAt, updatedAt time.Time

	queryRole := `SELECT id, name, description, status_id, created_at, updated_at FROM roles WHERE id = $1`
	err := r.storage.QueryRow(ctx, queryRole, id).Scan(&role.ID, &role.Name, &role.Description, &role.StatusID, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка поиска роли: %w", err)
	}
	role.CreatedAt = createdAt.Local().Format("2006-01-02 15:04:05")
	role.UpdatedAt = updatedAt.Local().Format("2006-01-02 15:04:05")
	queryPerms := `
		SELECT p.id, p.name, p.description FROM permissions p
		INNER JOIN role_permissions rp ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.name`
	rows, err := r.storage.Query(ctx, queryPerms, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения прав для роли: %w", err)
	}
	defer rows.Close()

	permissions := make([]dto.PermissionDTO, 0)
	for rows.Next() {
		var p dto.PermissionDTO
		if err = rows.Scan(&p.ID, &p.Name, &p.Description); err != nil {
			return nil, fmt.Errorf("ошибка сканирования права: %w", err)
		}
		permissions = append(permissions, p)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка после итерации по правам: %w", err)
	}

	role.Permissions = permissions

	return role, nil
}

func (r *RoleRepository) CreateRoleInTx(ctx context.Context, tx pgx.Tx, dto dto.CreateRoleDTO) (int, error) {
	var newID int
	query := `INSERT INTO roles (name, description, status_id) VALUES ($1, $2, $3) RETURNING id`
	err := tx.QueryRow(ctx, query, dto.Name, dto.Description, dto.StatusID).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("ошибка создания роли в транзакции: %w", err)
	}
	return newID, nil
}

func (r *RoleRepository) LinkPermissionsToRoleInTx(ctx context.Context, tx pgx.Tx, roleID int, permissionIDs []int) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	rows := make([][]interface{}, len(permissionIDs))
	for i, permID := range permissionIDs {
		rows[i] = []interface{}{roleID, permID}
	}
	_, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"role_permissions"},
		[]string{"role_id", "permission_id"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("ошибка привязки прав к роли: %w", err)
	}
	return nil
}

func (r *RoleRepository) UnlinkAllPermissionsFromRoleInTx(ctx context.Context, tx pgx.Tx, roleID uint64) error {
	_, err := tx.Exec(ctx, "DELETE FROM role_permissions WHERE role_id = $1", roleID)
	if err != nil {
		return fmt.Errorf("ошибка отвязки старых прав от роли: %w", err)
	}
	return nil
}

func (r *RoleRepository) UpdateRoleInTx(ctx context.Context, tx pgx.Tx, id uint64, dto dto.UpdateRoleDTO) error {
	var queryBuilder strings.Builder
	args := pgx.NamedArgs{"id": id}

	queryBuilder.WriteString("UPDATE roles SET updated_at = NOW()")

	if dto.Name != "" {
		queryBuilder.WriteString(", name = @name")
		args["name"] = dto.Name
	}
	if dto.Description != "" {
		queryBuilder.WriteString(", description = @description")
		args["description"] = dto.Description
	}
	if dto.StatusID != nil {
		queryBuilder.WriteString(", status_id = @status_id")
		args["status_id"] = *dto.StatusID
	}

	if len(args) == 1 {
		return nil
	}

	queryBuilder.WriteString(" WHERE id = @id")

	result, err := tx.Exec(ctx, queryBuilder.String(), args)
	if err != nil {
		return fmt.Errorf("ошибка обновления роли: %w", err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *RoleRepository) DeleteRole(ctx context.Context, id uint64) error {
	result, err := r.storage.Exec(ctx, "DELETE FROM roles WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("ошибка удаления роли: %w", err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

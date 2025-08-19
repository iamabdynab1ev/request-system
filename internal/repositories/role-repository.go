package repositories

import (
	"context"
	"errors"
	"fmt"
	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RoleRepositoryInterface interface {
	GetRoles(ctx context.Context, limit uint64, offset uint64) ([]dto.RoleDTO, uint64, error)
	FindByID(ctx context.Context, id uint64) (*dto.RoleDTO, error)
	CreateRoleInTx(ctx context.Context, tx pgx.Tx, dto dto.CreateRoleDTO) (uint64, error)
	UpdateRoleInTx(ctx context.Context, tx pgx.Tx, id uint64, dto dto.UpdateRoleDTO) error
	LinkPermissionsToRoleInTx(ctx context.Context, tx pgx.Tx, roleID uint64, permissionIDs []uint64) error
	UnlinkAllPermissionsFromRoleInTx(ctx context.Context, tx pgx.Tx, roleID uint64) error
	DeleteRole(ctx context.Context, id uint64) error
	BeginTx(ctx context.Context) (pgx.Tx, error)
}

type RoleRepository struct {
	storage *pgxpool.Pool
}

func NewRoleRepository(storage *pgxpool.Pool) RoleRepositoryInterface {
	return &RoleRepository{storage: storage}
}

func (r *RoleRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

func (r *RoleRepository) GetRoles(ctx context.Context, limit uint64, offset uint64) ([]dto.RoleDTO, uint64, error) {
	var total uint64
	if err := r.storage.QueryRow(ctx, "SELECT COUNT(*) FROM public.roles").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета ролей: %w", err)
	}

	if total == 0 {
		return []dto.RoleDTO{}, 0, nil
	}

	query := `
		SELECT
			r.id,
			r.name,
			r.description,
			r.status_id,
			r.created_at,
			r.updated_at,
			COALESCE(ARRAY_AGG(rp.permission_id) FILTER (WHERE rp.permission_id IS NOT NULL), '{}') AS permissions
		FROM
			public.roles r
		LEFT JOIN
			public.role_permissions rp ON r.id = rp.role_id
		GROUP BY
			r.id
		ORDER BY
			r.id
		LIMIT $1 OFFSET $2;
	`
	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка получения списка ролей: %w", err)
	}
	defer rows.Close()

	roles := make([]dto.RoleDTO, 0)
	for rows.Next() {
		var role dto.RoleDTO
		if err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.Description,
			&role.StatusID,
			&role.CreatedAt,
			&role.UpdatedAt,
			&role.Permissions, // Теперь это []uint64, и ошибки не будет
		); err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования строки роли: %w", err)
		}

		// Эта проверка нужна и полезна здесь
		if len(role.Permissions) == 1 && role.Permissions[0] == 0 {
			role.Permissions = []uint64{}
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("ошибка после итерации по ролям: %w", err)
	}
	return roles, total, nil
}

func (r *RoleRepository) FindByID(ctx context.Context, id uint64) (*dto.RoleDTO, error) {
	role := &dto.RoleDTO{}
	// Сначала получаем саму роль
	queryRole := `SELECT id, name, description, status_id, created_at, updated_at FROM roles WHERE id = $1`
	err := r.storage.QueryRow(ctx, queryRole, id).Scan(&role.ID, &role.Name, &role.Description, &role.StatusID, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка поиска роли: %w", err)
	}

	// Теперь получаем только ID разрешений
	queryPerms := `SELECT permission_id FROM public.role_permissions WHERE role_id = $1 ORDER BY permission_id`
	rows, err := r.storage.Query(ctx, queryPerms, id)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения ID прав для роли: %w", err)
	}
	defer rows.Close()

	permissionIDs := make([]uint64, 0)
	for rows.Next() {
		var permID uint64
		if err := rows.Scan(&permID); err != nil {
			return nil, fmt.Errorf("ошибка сканирования ID права: %w", err)
		}
		permissionIDs = append(permissionIDs, permID)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка после итерации по ID прав: %w", err)
	}

	// Присваиваем слайс чисел полю, которое тоже является слайсом чисел. Все работает.
	role.Permissions = permissionIDs
	return role, nil
}
func (r *RoleRepository) CreateRoleInTx(ctx context.Context, tx pgx.Tx, dto dto.CreateRoleDTO) (uint64, error) {
	var newID uint64
	query := `INSERT INTO roles (name, description, status_id) VALUES ($1, $2, $3) RETURNING id`
	err := tx.QueryRow(ctx, query, dto.Name, dto.Description, dto.StatusID).Scan(&newID)
	return newID, err
}

func (r *RoleRepository) LinkPermissionsToRoleInTx(ctx context.Context, tx pgx.Tx, roleID uint64, permissionIDs []uint64) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	rows := make([][]interface{}, len(permissionIDs))
	for i, permID := range permissionIDs {
		rows[i] = []interface{}{roleID, permID}
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"public", "role_permissions"}, []string{"role_id", "permission_id"}, pgx.CopyFromRows(rows))
	return err
}

func (r *RoleRepository) UnlinkAllPermissionsFromRoleInTx(ctx context.Context, tx pgx.Tx, roleID uint64) error {
	_, err := tx.Exec(ctx, "DELETE FROM role_permissions WHERE role_id = $1", roleID)
	return err
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
	if dto.StatusID != 0 {
		queryBuilder.WriteString(", status_id = @status_id")
		args["status_id"] = dto.StatusID
	}

	if len(args) == 1 {
		return nil
	}

	queryBuilder.WriteString(" WHERE id = @id")

	result, err := tx.Exec(ctx, queryBuilder.String(), args)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

func (r *RoleRepository) DeleteRole(ctx context.Context, id uint64) error {
	result, err := r.storage.Exec(ctx, "DELETE FROM roles WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

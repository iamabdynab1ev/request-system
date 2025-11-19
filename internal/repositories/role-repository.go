package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const roleTable = "roles"

var roleAllowedFilterFields = map[string]string{
	"status_id": "r.status_id",
}

var roleAllowedSortFields = map[string]string{
	"id":         "r.id",
	"name":       "r.name",
	"created_at": "r.created_at",
}

type RoleRepositoryInterface interface {
	GetRoles(ctx context.Context, filter types.Filter) ([]entities.Role, uint64, error)
	CountRoles(ctx context.Context, filter types.Filter) (uint64, error)
	FindRoleByID(ctx context.Context, id uint64) (*entities.Role, []uint64, error)
	CreateRoleInTx(ctx context.Context, tx pgx.Tx, role entities.Role) (uint64, error)
	UpdateRoleInTx(ctx context.Context, tx pgx.Tx, role entities.Role) error
	LinkPermissionsToRoleInTx(ctx context.Context, tx pgx.Tx, roleID uint64, permissionIDs []uint64) error
	UnlinkAllPermissionsFromRoleInTx(ctx context.Context, tx pgx.Tx, roleID uint64) error
	DeleteRole(ctx context.Context, id uint64) error
	BeginTx(ctx context.Context) (pgx.Tx, error)
	FindByName(ctx context.Context, tx pgx.Tx, name string) (*entities.Role, error)
}

type RoleRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewRoleRepository(storage *pgxpool.Pool, logger *zap.Logger) RoleRepositoryInterface {
	return &RoleRepository{storage: storage, logger: logger}
}

func (r *RoleRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

func (r *RoleRepository) buildFilterQuery(filter types.Filter) (string, []interface{}) {
	args := make([]interface{}, 0)
	conditions := []string{}
	argCounter := 1

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		conditions = append(conditions, fmt.Sprintf("(r.name ILIKE $%d OR r.description ILIKE $%d)", argCounter, argCounter))
		args = append(args, searchPattern)
		argCounter++
	}

	for key, value := range filter.Filter {
		if dbColumn, ok := roleAllowedFilterFields[key]; ok {
			if strVal, ok := value.(string); ok && strings.Contains(strVal, ",") {
				items := strings.Split(strVal, ",")
				placeholders := make([]string, len(items))
				for i, item := range items {
					placeholders[i] = fmt.Sprintf("$%d", argCounter)
					args = append(args, item)
					argCounter++
				}
				conditions = append(conditions, fmt.Sprintf("%s IN (%s)", dbColumn, strings.Join(placeholders, ",")))
			} else {
				conditions = append(conditions, fmt.Sprintf("%s = $%d", dbColumn, argCounter))
				args = append(args, value)
				argCounter++
			}
		}
	}

	var whereClause string
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}
	return whereClause, args
}

func (r *RoleRepository) GetRoles(ctx context.Context, filter types.Filter) ([]entities.Role, uint64, error) {
	whereClause, args := r.buildFilterQuery(filter)

	total, err := r.CountRoles(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Role{}, 0, nil
	}

	orderByClause := "ORDER BY r.id DESC"
	if len(filter.Sort) > 0 {
		var sortParts []string
		for field, direction := range filter.Sort {
			if dbColumn, ok := roleAllowedSortFields[field]; ok {
				safeDirection := "ASC"
				if strings.ToLower(direction) == "desc" {
					safeDirection = "DESC"
				}
				sortParts = append(sortParts, fmt.Sprintf("%s %s", dbColumn, safeDirection))
			}
		}
		if len(sortParts) > 0 {
			orderByClause = "ORDER BY " + strings.Join(sortParts, ", ")
		}
	}

	limitClause := ""
	argCounter := len(args) + 1
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", argCounter, argCounter+1)
		args = append(args, filter.Limit, filter.Offset)
	}

	query := fmt.Sprintf(`
		SELECT r.id, r.name, r.description, r.status_id, r.created_at, r.updated_at,
			COALESCE(
				(SELECT json_agg(rp.permission_id) FROM role_permissions rp WHERE rp.role_id = r.id),
				'[]'::json
			) AS permission_ids
		FROM
			roles r %s %s %s
	`, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("ошибка получения списка ролей с правами", zap.Error(err), zap.String("query", query))
		return nil, 0, err
	}
	defer rows.Close()

	roles := make([]entities.Role, 0)
	for rows.Next() {
		var role entities.Role
		var createdAt, updatedAt time.Time
		var permissionIDsBytes []byte

		err := rows.Scan(
			&role.ID, &role.Name, &role.Description, &role.StatusID,
			&createdAt, &updatedAt, &permissionIDsBytes,
		)
		if err != nil {
			r.logger.Error("ошибка сканирования строки роли", zap.Error(err))
			return nil, 0, err
		}

		if err := json.Unmarshal(permissionIDsBytes, &role.Permissions); err != nil {
			r.logger.Error("ошибка распаковки permission_ids из JSON", zap.Error(err))
			role.Permissions = []uint64{}
		}

		role.CreatedAt = &createdAt
		role.UpdatedAt = &updatedAt
		roles = append(roles, role)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("ошибка после итерации по ролям", zap.Error(err))
		return nil, 0, err
	}

	return roles, total, nil
}

func (r *RoleRepository) CountRoles(ctx context.Context, filter types.Filter) (uint64, error) {
	whereClause, args := r.buildFilterQuery(filter)
	countQuery := fmt.Sprintf("SELECT COUNT(r.id) FROM %s r %s", roleTable, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		r.logger.Error("ошибка подсчета ролей", zap.Error(err), zap.String("query", countQuery))
		return 0, err
	}
	return total, nil
}

func (r *RoleRepository) FindRoleByID(ctx context.Context, id uint64) (*entities.Role, []uint64, error) {
	var role entities.Role
	var createdAt, updatedAt time.Time

	queryRole := `SELECT id, name, description, status_id, created_at, updated_at FROM roles WHERE id = $1`
	err := r.storage.QueryRow(ctx, queryRole, id).Scan(&role.ID, &role.Name, &role.Description, &role.StatusID, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	role.CreatedAt, role.UpdatedAt = &createdAt, &updatedAt

	queryPerms := `SELECT permission_id FROM role_permissions WHERE role_id = $1`
	rows, err := r.storage.Query(ctx, queryPerms, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	permissionIDs, err := pgx.CollectRows(rows, pgx.RowTo[uint64])
	if err != nil {
		return nil, nil, err
	}

	return &role, permissionIDs, nil
}

func (r *RoleRepository) CreateRoleInTx(ctx context.Context, tx pgx.Tx, role entities.Role) (uint64, error) {
	query := `INSERT INTO roles (name, description, status_id) VALUES ($1, $2, $3) RETURNING id`
	var newID uint64
	err := tx.QueryRow(ctx, query, role.Name, role.Description, role.StatusID).Scan(&newID)
	return newID, err
}

func (r *RoleRepository) UpdateRoleInTx(ctx context.Context, tx pgx.Tx, role entities.Role) error {
	query := `UPDATE roles SET name = $1, description = $2, status_id = $3, updated_at = NOW() WHERE id = $4`
	result, err := tx.Exec(ctx, query, role.Name, role.Description, role.StatusID, role.ID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *RoleRepository) LinkPermissionsToRoleInTx(ctx context.Context, tx pgx.Tx, roleID uint64, permissionIDs []uint64) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	rows := make([][]interface{}, len(permissionIDs))
	for i, permID := range permissionIDs {
		rows[i] = []interface{}{roleID, permID}
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"role_permissions"}, []string{"role_id", "permission_id"}, pgx.CopyFromRows(rows))
	return err
}

func (r *RoleRepository) UnlinkAllPermissionsFromRoleInTx(ctx context.Context, tx pgx.Tx, roleID uint64) error {
	_, err := tx.Exec(ctx, "DELETE FROM role_permissions WHERE role_id = $1", roleID)
	return err
}

func (r *RoleRepository) DeleteRole(ctx context.Context, id uint64) error {
	query := `DELETE FROM roles WHERE id = $1`
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *RoleRepository) FindByName(ctx context.Context, tx pgx.Tx, name string) (*entities.Role, error) {
	// Выбираем только те поля, которые нам нужны для назначения роли.
	query := `SELECT id, name, status_id FROM roles WHERE name = $1`

	var role entities.Role
	err := tx.QueryRow(ctx, query, name).Scan(&role.ID, &role.Name, &role.StatusID)

	if errors.Is(err, pgx.ErrNoRows) {
		// Если роль не найдена, возвращаем специальную ошибку ErrNotFound.
		// Это не системная ошибка, а ожидаемый результат.
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		// А это уже непредвиденная ошибка базы данных. Ее нужно логировать.
		r.logger.Error("ошибка поиска роли по имени", zap.String("name", name), zap.Error(err))
		return nil, err
	}

	return &role, nil
}

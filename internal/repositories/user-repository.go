package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
)

const (
	userTable = "users"
)

var (
	userSelectFields = []string{
		"u.id", "u.fio", "u.email", "u.phone_number", "u.password", "u.position_id", "u.status_id", "s.code as status_code",
		"u.photo_url", "u.branch_id", "u.department_id", "u.office_id", "u.otdel_id",
		"u.created_at", "u.updated_at", "u.deleted_at", "u.must_change_password", "u.is_head",
	}
	userAllowedFilterFields = map[string]string{
		"status_id":     "u.status_id",
		"department_id": "u.department_id",
		"branch_id":     "u.branch_id",
		"position":      "u.position_id",
	}
	userAllowedSortFields = map[string]bool{"id": true, "fio": true, "created_at": true, "updated_at": true}
)

type UserRepositoryInterface interface {
	// --- Основной CRUD ---
	GetUsers(ctx context.Context, filter types.Filter) ([]entities.User, uint64, error)
	FindUserByID(ctx context.Context, id uint64) (*entities.User, error)
	FindUserByIDInTx(ctx context.Context, tx pgx.Tx, id uint64) (*entities.User, error)
	CreateUser(ctx context.Context, tx pgx.Tx, user *entities.User) (uint64, error)
	UpdateUser(ctx context.Context, tx pgx.Tx, payload dto.UpdateUserDTO) error
	DeleteUser(ctx context.Context, id uint64) error

	// --- Методы для Аутентификации и Авторизации ---
	FindUserByEmailOrLogin(ctx context.Context, login string) (*entities.User, error)
	FindUserByPhone(ctx context.Context, phone string) (*entities.User, error)
	UpdatePassword(ctx context.Context, userID uint64, newPasswordHash string) error
	UpdatePasswordAndClearFlag(ctx context.Context, userID uint64, newPasswordHash string) error

	// --- Методы для Работы с Ролями и Правами ---
	SyncUserRoles(ctx context.Context, tx pgx.Tx, userID uint64, roleIDs []uint64) error
	GetRolesByUserID(ctx context.Context, userID uint64) ([]dto.ShortRoleDTO, error)
	GetRolesByUserIDs(ctx context.Context, userIDs []uint64) (map[uint64][]dto.ShortRoleDTO, error)
	FindUserIDsByRoleID(ctx context.Context, roleID uint64) ([]uint64, error)
	SyncUserDirectPermissions(ctx context.Context, tx pgx.Tx, userID uint64, permissionIDs []uint64) error
	SyncUserDeniedPermissions(ctx context.Context, tx pgx.Tx, userID uint64, permissionIDs []uint64) error
	GetPermissionListsForUI(ctx context.Context, userID uint64) (currentIDs, unavailableIDs []uint64, err error)

	// --- Методы-хелперы для Сервисов ---
	BeginTx(ctx context.Context) (pgx.Tx, error)
	FindUsersByIDs(ctx context.Context, userIDs []uint64) (map[uint64]entities.User, error)
	IsHeadExistsInDepartment(ctx context.Context, departmentID uint64, excludeUserID uint64) (bool, error)
	FindHighestPositionInDepartment(ctx context.Context, tx pgx.Tx, departmentID uint64) (*entities.Position, error)
	FindActiveUsersByPositionCode(ctx context.Context, tx pgx.Tx, code string, departmentID uint64, otdelID *uint64) ([]entities.User, error)
}

type UserRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewUserRepository(storage *pgxpool.Pool, logger *zap.Logger) UserRepositoryInterface {
	return &UserRepository{storage: storage, logger: logger}
}

func (r *UserRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.storage.Begin(ctx)
}

// scanUserEntity остается без изменений, как в вашем файле
func scanUserEntity(row pgx.Row) (*entities.User, error) {
	var user entities.User
	var createdAt, updatedAt, deletedAt sql.NullTime
	err := row.Scan(
		&user.ID, &user.Fio, &user.Email, &user.PhoneNumber, &user.Password,
		&user.PositionID, &user.StatusID, &user.StatusCode, &user.PhotoURL,
		&user.BranchID, &user.DepartmentID, &user.OfficeID, &user.OtdelID,
		&createdAt, &updatedAt, &deletedAt, &user.MustChangePassword, &user.IsHead,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	if createdAt.Valid {
		user.CreatedAt = &createdAt.Time
	}
	if updatedAt.Valid {
		user.UpdatedAt = &updatedAt.Time
	}
	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}
	return &user, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, tx pgx.Tx, entity *entities.User) (uint64, error) {
	query := `INSERT INTO users (fio, email, phone_number, password, position_id, status_id, branch_id, 
								 department_id, office_id, otdel_id, photo_url, must_change_password, is_head,
								 created_at, updated_at) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id`

	var createdID uint64
	err := tx.QueryRow(ctx, query,
		entity.Fio, entity.Email, entity.PhoneNumber, entity.Password, entity.PositionID,
		entity.StatusID, entity.BranchID, entity.DepartmentID, entity.OfficeID,
		entity.OtdelID, entity.PhotoURL, entity.MustChangePassword, entity.IsHead,
		entity.CreatedAt, entity.UpdatedAt,
	).Scan(&createdID)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			switch pgErr.Code {
			case "23505":
				if strings.Contains(pgErr.ConstraintName, "users_email_key") {
					return 0, apperrors.NewHttpError(http.StatusConflict, "Email уже используется", err, nil)
				}
				if strings.Contains(pgErr.ConstraintName, "users_phone_number_key") {
					return 0, apperrors.NewHttpError(http.StatusConflict, "Номер телефона уже используется", err, nil)
				}
			}
		}
		return 0, err
	}
	return createdID, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, tx pgx.Tx, payload dto.UpdateUserDTO) error {
	builder := sq.Update(userTable).
		PlaceholderFormat(sq.Dollar).
		Where(sq.Eq{"id": payload.ID, "deleted_at": nil}).
		Set("updated_at", time.Now())

	if payload.Fio != nil {
		builder = builder.Set("fio", *payload.Fio)
	}
	if payload.Email != nil {
		builder = builder.Set("email", *payload.Email)
	}
	if payload.PhoneNumber != nil {
		builder = builder.Set("phone_number", *payload.PhoneNumber)
	}
	if payload.PositionID != nil {
		builder = builder.Set("position_id", *payload.PositionID)
	}
	if payload.StatusID != nil {
		builder = builder.Set("status_id", *payload.StatusID)
	}
	if payload.BranchID != nil {
		builder = builder.Set("branch_id", *payload.BranchID)
	}
	if payload.DepartmentID != nil {
		builder = builder.Set("department_id", *payload.DepartmentID)
	}
	if payload.OfficeID != nil {
		builder = builder.Set("office_id", payload.OfficeID)
	}
	if payload.OtdelID != nil {
		builder = builder.Set("otdel_id", payload.OtdelID)
	}
	if payload.PhotoURL != nil {
		builder = builder.Set("photo_url", payload.PhotoURL)
	}
	if payload.IsHead != nil {
		builder = builder.Set("is_head", *payload.IsHead)
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return err
	}
	r.logger.Info("Executing UpdateUser", zap.String("query", query), zap.Any("args", args))
	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			switch pgErr.Code {
			case "23505":
				if strings.Contains(pgErr.ConstraintName, "users_email_key") {
					return apperrors.NewHttpError(http.StatusConflict, "Email уже используется", err, nil)
				}
				if strings.Contains(pgErr.ConstraintName, "users_phone_number_key") {
					return apperrors.NewHttpError(http.StatusConflict, "Номер телефона уже используется", err, nil)
				}
			}
		}
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *UserRepository) SyncUserRoles(ctx context.Context, tx pgx.Tx, userID uint64, roleIDs []uint64) error {
	_, err := tx.Exec(ctx, "DELETE FROM user_roles WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("ошибка при удалении старых ролей пользователя: %w", err)
	}
	if len(roleIDs) == 0 {
		return nil
	}
	rows := make([][]interface{}, len(roleIDs))
	for i, roleID := range roleIDs {
		rows[i] = []interface{}{userID, roleID}
	}
	_, err = tx.CopyFrom(ctx, pgx.Identifier{"user_roles"}, []string{"user_id", "role_id"}, pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("ошибка при вставке новых ролей пользователя: %w", err)
	}
	return nil
}

func (r *UserRepository) GetUsers(ctx context.Context, filter types.Filter) ([]entities.User, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	// --- Count builder ---
	countBuilder := psql.Select("COUNT(u.id)").From("users u").Join("statuses s ON u.status_id = s.id").
		Where(sq.Eq{"u.deleted_at": nil})

	// --- Select builder ---
	selectBuilder := psql.Select(userSelectFields...).From("users u").
		Join("statuses s ON u.status_id = s.id").
		Where(sq.Eq{"u.deleted_at": nil})

	// --- Фильтрация ---
	for key, value := range filter.Filter {
		column := "u." + key // допустим, имя поля совпадает с БД
		if strVal, ok := value.(string); ok && strings.Contains(strVal, ",") {
			selectBuilder = selectBuilder.Where(sq.Eq{column: strings.Split(strVal, ",")})
			countBuilder = countBuilder.Where(sq.Eq{column: strings.Split(strVal, ",")})
		} else {
			selectBuilder = selectBuilder.Where(sq.Eq{column: value})
			countBuilder = countBuilder.Where(sq.Eq{column: value})
		}
	}

	// --- Поиск по всем текстовым полям ---
	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%"
		searchCondition := sq.Or{
			sq.ILike{"u.fio": searchPattern},
			sq.ILike{"u.email": searchPattern},
			sq.ILike{"u.phone_number": searchPattern},
			sq.ILike{"u.photo_url": searchPattern},
		}
		selectBuilder = selectBuilder.Where(searchCondition)
		countBuilder = countBuilder.Where(searchCondition)
	}

	// --- Count query ---
	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}
	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return []entities.User{}, 0, nil
	}

	// --- Сортировка ---
	if len(filter.Sort) > 0 {
		for field, direction := range filter.Sort {
			safeDirection := "ASC"
			if strings.ToUpper(direction) == "DESC" {
				safeDirection = "DESC"
			}
			selectBuilder = selectBuilder.OrderBy(fmt.Sprintf("u.%s %s", field, safeDirection))
		}
	} else {
		selectBuilder = selectBuilder.OrderBy("u.id DESC")
	}

	// --- Пагинация ---
	if filter.WithPagination {
		selectBuilder = selectBuilder.Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	}

	// --- Main query ---
	query, args, err := selectBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := make([]entities.User, 0)
	for rows.Next() {
		user, err := scanUserEntity(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, *user)
	}

	return users, totalCount, rows.Err()
}

func (r *UserRepository) FindUserByID(ctx context.Context, id uint64) (*entities.User, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select(userSelectFields...).
		From("users u").
		Join("statuses s ON u.status_id = s.id").
		Where(sq.Eq{"u.id": id, "u.deleted_at": nil}).ToSql()
	if err != nil {
		return nil, err
	}
	row := r.storage.QueryRow(ctx, query, args...)
	return scanUserEntity(row)
}

// Новый метод, работающий в транзакции
func (r *UserRepository) FindUserByIDInTx(ctx context.Context, tx pgx.Tx, id uint64) (*entities.User, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select(userSelectFields...).
		From("users u").
		Join("statuses s ON u.status_id = s.id").
		Where(sq.Eq{"u.id": id, "u.deleted_at": nil}).ToSql()
	if err != nil {
		return nil, err
	}
	row := tx.QueryRow(ctx, query, args...) // Используем tx
	return scanUserEntity(row)
}

func (r *UserRepository) FindUserByEmailOrLogin(ctx context.Context, login string) (*entities.User, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select(userSelectFields...).
		From("users u").
		Join("statuses s ON u.status_id = s.id").
		Where(sq.Eq{"LOWER(u.email)": strings.ToLower(login), "u.deleted_at": nil}).ToSql()
	if err != nil {
		return nil, err
	}
	row := r.storage.QueryRow(ctx, query, args...)
	return scanUserEntity(row)
}

func (r *UserRepository) FindUserByPhone(ctx context.Context, phone string) (*entities.User, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select(userSelectFields...).
		From("users u").
		Join("statuses s ON u.status_id = s.id").
		Where(sq.Eq{"u.phone_number": phone, "u.deleted_at": nil}).ToSql()
	if err != nil {
		return nil, err
	}
	row := r.storage.QueryRow(ctx, query, args...)
	return scanUserEntity(row)
}

func (r *UserRepository) DeleteUser(ctx context.Context, id uint64) error {
	query := `UPDATE users SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *UserRepository) UpdatePasswordAndClearFlag(ctx context.Context, userID uint64, newPasswordHash string) error {
	query := `UPDATE users SET password = $1, must_change_password = FALSE, updated_at = NOW() WHERE id = $2`
	result, err := r.storage.Exec(ctx, query, newPasswordHash, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uint64, newPasswordHash string) error {
	query := `UPDATE users SET password = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.storage.Exec(ctx, query, newPasswordHash, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *UserRepository) FindUserIDsByRoleID(ctx context.Context, roleID uint64) ([]uint64, error) {
	query := `SELECT user_id FROM public.user_roles WHERE role_id = $1`
	rows, err := r.storage.Query(ctx, query, roleID)
	if err != nil {
		r.logger.Error("Ошибка получения ID пользователей по ID роли", zap.Uint64("roleID", roleID), zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var ids []uint64
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *UserRepository) GetRolesByUserIDs(ctx context.Context, userIDs []uint64) (map[uint64][]dto.ShortRoleDTO, error) {
	if len(userIDs) == 0 {
		return make(map[uint64][]dto.ShortRoleDTO), nil
	}
	query := `SELECT ur.user_id, r.id, r.name FROM roles r
			  JOIN user_roles ur ON r.id = ur.role_id
			  WHERE ur.user_id = ANY($1) ORDER BY ur.user_id, r.name`

	rows, err := r.storage.Query(ctx, query, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	userRolesMap := make(map[uint64][]dto.ShortRoleDTO)
	for rows.Next() {
		var userID uint64
		var role dto.ShortRoleDTO
		if err := rows.Scan(&userID, &role.ID, &role.Name); err != nil {
			return nil, err
		}
		userRolesMap[userID] = append(userRolesMap[userID], role)
	}
	return userRolesMap, nil
}

func (r *UserRepository) GetRolesByUserID(ctx context.Context, userID uint64) ([]dto.ShortRoleDTO, error) {
	query := `SELECT r.id, r.name FROM roles r
			  JOIN user_roles ur ON r.id = ur.role_id
			  WHERE ur.user_id = $1 ORDER BY r.name`
	rows, err := r.storage.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []dto.ShortRoleDTO
	for rows.Next() {
		var role dto.ShortRoleDTO
		if err := rows.Scan(&role.ID, &role.Name); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *UserRepository) FindUsersByIDs(ctx context.Context, userIDs []uint64) (map[uint64]entities.User, error) {
	if len(userIDs) == 0 {
		return make(map[uint64]entities.User), nil
	}
	query := `SELECT ` + strings.Join(userSelectFields, ", ") + `
			  FROM users u 
			  JOIN statuses s ON u.status_id = s.id
			  WHERE u.id = ANY($1) AND u.deleted_at IS NULL`
	rows, err := r.storage.Query(ctx, query, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	usersMap := make(map[uint64]entities.User)
	for rows.Next() {
		user, err := scanUserEntity(rows)
		if err != nil {
			return nil, err
		}
		usersMap[user.ID] = *user
	}
	return usersMap, rows.Err()
}

func (r *UserRepository) IsHeadExistsInDepartment(ctx context.Context, departmentID uint64, excludeUserID uint64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE department_id = $1 AND is_head = TRUE AND id != $2 AND deleted_at IS NULL)`
	err := r.storage.QueryRow(ctx, query, departmentID, excludeUserID).Scan(&exists)
	if err != nil {
		r.logger.Error("UserRepository.IsHeadExistsInDepartment: ошибка при проверке", zap.Error(err))
		return false, err
	}
	return exists, nil
}

func (r *UserRepository) SyncUserDirectPermissions(ctx context.Context, tx pgx.Tx, userID uint64, permissionIDs []uint64) error {
	_, err := tx.Exec(ctx, "DELETE FROM user_permissions WHERE user_id = $1", userID)
	if err != nil {
		return err
	}

	if len(permissionIDs) == 0 {
		return nil
	}

	rows := make([][]interface{}, len(permissionIDs))
	for i, permID := range permissionIDs {
		rows[i] = []interface{}{userID, permID}
	}
	_, err = tx.CopyFrom(ctx, pgx.Identifier{"user_permissions"}, []string{"user_id", "permission_id"}, pgx.CopyFromRows(rows))
	return err
}

func (r *UserRepository) SyncUserDeniedPermissions(ctx context.Context, tx pgx.Tx, userID uint64, permissionIDs []uint64) error {
	_, err := tx.Exec(ctx, "DELETE FROM user_permission_denials WHERE user_id = $1", userID)
	if err != nil {
		return err
	}

	if len(permissionIDs) == 0 {
		return nil
	}

	rows := make([][]interface{}, len(permissionIDs))
	for i, permID := range permissionIDs {
		rows[i] = []interface{}{userID, permID}
	}
	_, err = tx.CopyFrom(ctx, pgx.Identifier{"user_permission_denials"}, []string{"user_id", "permission_id"}, pgx.CopyFromRows(rows))
	return err
}

func (r *UserRepository) GetPermissionListsForUI(ctx context.Context, userID uint64) ([]uint64, []uint64, error) {
	query := `
		WITH all_perms AS (
			SELECT id FROM permissions
		),
		user_final_perms AS (
			SELECT p.id
			FROM permissions p
			LEFT JOIN role_permissions rp ON rp.permission_id = p.id
			LEFT JOIN user_roles ur ON ur.role_id = rp.role_id AND ur.user_id = $1
			LEFT JOIN user_permissions up ON up.permission_id = p.id AND up.user_id = $1
			LEFT JOIN user_permission_denials upd ON upd.permission_id = p.id AND upd.user_id = $1
			WHERE (ur.user_id IS NOT NULL OR up.user_id IS NOT NULL) AND upd.permission_id IS NULL
		)
		SELECT
			(SELECT COALESCE(array_agg(id ORDER BY id), '{}') FROM user_final_perms) AS current_permission_ids,
			(SELECT COALESCE(array_agg(id ORDER BY id), '{}') FROM all_perms WHERE id NOT IN (SELECT id FROM user_final_perms)) AS unavailable_permission_ids;
	`
	var currentIDs, unavailableIDs []uint64
	err := r.storage.QueryRow(ctx, query, userID).Scan(&currentIDs, &unavailableIDs)
	return currentIDs, unavailableIDs, err
}

func (r *UserRepository) FindHighestPositionInDepartment(ctx context.Context, tx pgx.Tx, departmentID uint64) (*entities.Position, error) {
	query := `
        SELECT p.id, p.name, p.code, p.level, p.status_id, p.created_at, p.updated_at
        FROM positions p
        JOIN users u ON u.position_id = p.id
        WHERE u.department_id = $1 AND u.deleted_at IS NULL AND p.status_id = 2 -- Ищем только по активным должностям
        ORDER BY p.level DESC
        LIMIT 1
    `
	var pos entities.Position
	var code sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := tx.QueryRow(ctx, query, departmentID).Scan(&pos.Id, &pos.Name, &code, &pos.Level, &pos.StatusID, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка поиска высшей должности: %w", err)
	}

	if code.Valid {
		pos.Code = &code.String
	}
	if createdAt.Valid {
		pos.CreatedAt = &createdAt.Time
	}
	if updatedAt.Valid {
		pos.UpdatedAt = &updatedAt.Time
	}

	return &pos, nil
}

func (r *UserRepository) FindActiveUsersByPosition(ctx context.Context, tx pgx.Tx, positionID uint64, departmentID uint64) ([]entities.User, error) {
	// ВАЖНО: Убедитесь, что в таблице statuses код для активного юзера - "ACTIVE"
	query := `SELECT ` + strings.Join(userSelectFields, ", ") + `
        FROM users u 
        JOIN statuses s ON u.status_id = s.id
        WHERE u.position_id = $1 
          AND u.department_id = $2 
          AND s.code = 'ACTIVE' -- Используем код 'ACTIVE' из вашего сидера, не 'USER_ACTIVE'
          AND u.deleted_at IS NULL
        ORDER BY u.id`

	rows, err := tx.Query(ctx, query, positionID, departmentID)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса активных пользователей по должности: %w", err)
	}
	defer rows.Close()
	users := make([]entities.User, 0)
	for rows.Next() {
		user, err := scanUserEntity(rows)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования пользователя при поиске по должности: %w", err)
		}
		users = append(users, *user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка итерации по списку пользователей: %w", err)
	}

	return users, nil
}

func (r *UserRepository) FindActiveUsersByPositionCode(ctx context.Context, tx pgx.Tx, code string, departmentID uint64, otdelID *uint64) ([]entities.User, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	builder := psql.Select(userSelectFields...).
		From("users u").
		Join("positions p ON u.position_id = p.id").
		Join("statuses s ON u.status_id = s.id").
		Where(sq.Eq{
			"p.code":          code,
			"u.department_id": departmentID,
			"s.code":          "ACTIVE",
			"u.deleted_at":    nil,
		}).
		OrderBy("p.level DESC")

	if otdelID != nil {
		builder = builder.Where(sq.Eq{"u.otdel_id": *otdelID})
	}

	query, args, err := builder.ToSql()
	if err != nil {
		r.logger.Error("Ошибка сборки SQL для FindActiveUsersByPositionCode", zap.Error(err))
		return nil, err
	}
	r.logger.Debug("Executing FindActiveUsersByPositionCode", zap.String("query", query), zap.Any("args", args))

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("Ошибка выполнения запроса FindActiveUsersByPositionCode", zap.Error(err))
		return nil, err
	}

	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.User])
	if err != nil {
		r.logger.Error("Ошибка сканирования результатов в FindActiveUsersByPositionCode", zap.Error(err))
		return nil, fmt.Errorf("ошибка сканирования юзеров по коду должности: %w", err)
	}

	return users, nil
}

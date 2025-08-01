package repositories

import (
	"context"
	"errors"
	"fmt"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const userTableRepo = "users"

const userSelectFieldsForEntityRepo = "u.id, u.fio, u.email, u.phone_number, u.password, u.position, u.status_id, u.role_id, u.branch_id, u.department_id, u.office_id, u.otdel_id, r.name as role_name, u.created_at, u.updated_at, u.deleted_at"
const userJoinClauseRepo = "users u JOIN roles r ON u.role_id = r.id"

var userAllowedFilterFields = map[string]bool{
	"status_id":     true,
	"branch_id":     true,
	"department_id": true,
	"role_id":       true,
	"position":      true,
}

var userAllowedSortFields = map[string]bool{
	"id":         true,
	"fio":        true,
	"created_at": true,
	"updated_at": true,
}

type UserRepositoryInterface interface {
	GetUsers(ctx context.Context, filter types.Filter) ([]entities.User, uint64, error)
	FindUser(ctx context.Context, id uint64) (*entities.User, error)
	CreateUser(ctx context.Context, entity *entities.User) (*entities.User, error)
	UpdateUser(ctx context.Context, entity *entities.User) (*entities.User, error)
	DeleteUser(ctx context.Context, id uint64) error
	FindUserByEmailOrLogin(ctx context.Context, login string) (*entities.User, error)
	FindUserByPhone(ctx context.Context, phone string) (*entities.User, error)
	UpdatePassword(ctx context.Context, userID uint64, newPasswordHash string) error
	FindUserByID(ctx context.Context, id uint64) (*entities.User, error)
	FindHeadByDepartment(ctx context.Context, departmentID uint64) (*entities.User, error)
	FindHeadByDepartmentInTx(ctx context.Context, tx pgx.Tx, departmentID uint64) (*entities.User, error)
}

type UserRepository struct {
	storage *pgxpool.Pool
}

func NewUserRepository(storage *pgxpool.Pool) UserRepositoryInterface {
	return &UserRepository{
		storage: storage,
	}
}

func (r *UserRepository) scanUser(row pgx.Row, user *entities.User) error {
	return row.Scan(
		&user.ID, &user.Fio, &user.Email, &user.PhoneNumber, &user.Password,
		&user.Position, &user.StatusID, &user.RoleID, &user.BranchID,
		&user.DepartmentID, &user.OfficeID, &user.OtdelID, &user.RoleName,
		&user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)
}

func (r *UserRepository) GetUsers(ctx context.Context, filter types.Filter) ([]entities.User, uint64, error) {
	var args []interface{}
	conditions := []string{"u.deleted_at IS NULL"}
	placeholderID := 1

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(u.fio ILIKE $%d OR u.email ILIKE $%d OR u.phone_number ILIKE $%d)", placeholderID, placeholderID, placeholderID))
		args = append(args, "%"+filter.Search+"%")
		placeholderID++
	}

	for key, value := range filter.Filter {
		if userAllowedFilterFields[key] {
			conditions = append(conditions, fmt.Sprintf("u.%s = $%d", key, placeholderID))
			args = append(args, value)
			placeholderID++
		}
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(u.id) FROM %s %s", userJoinClauseRepo, whereClause)

	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета пользователей: %w", err)
	}

	if totalCount == 0 {
		return []entities.User{}, 0, nil
	}

	orderByClause := "ORDER BY u.id DESC"
	if len(filter.Sort) > 0 {
		var sortParts []string
		for field, direction := range filter.Sort {
			if userAllowedSortFields[field] {
				sortParts = append(sortParts, fmt.Sprintf("u.%s %s", field, direction))
			}
		}
		if len(sortParts) > 0 {
			orderByClause = "ORDER BY " + strings.Join(sortParts, ", ")
		}
	}

	limitClause := ""
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", placeholderID, placeholderID+1)
		args = append(args, filter.Limit, filter.Offset)
	}

	mainQuery := fmt.Sprintf("SELECT %s FROM %s %s %s %s",
		userSelectFieldsForEntityRepo,
		userJoinClauseRepo,
		whereClause,
		orderByClause,
		limitClause,
	)

	rows, err := r.storage.Query(ctx, mainQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка получения пользователей: %w", err)
	}
	defer rows.Close()

	users := make([]entities.User, 0, filter.Limit)
	for rows.Next() {
		var user entities.User
		if err := r.scanUser(rows, &user); err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return users, totalCount, nil
}

func (r *UserRepository) FindHeadByDepartment(ctx context.Context, departmentID uint64) (*entities.User, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM %s
		WHERE u.department_id = $1 AND LOWER(TRIM(r.name)) = LOWER('User') AND u.deleted_at IS NULL
		LIMIT 1
	`, userSelectFieldsForEntityRepo, userJoinClauseRepo)

	var user entities.User
	row := r.storage.QueryRow(ctx, query, departmentID)
	err := r.scanUser(row, &user)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("ошибка поиска пользователя в отделе: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) FindHeadByDepartmentInTx(ctx context.Context, tx pgx.Tx, departmentID uint64) (*entities.User, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM %s
		WHERE u.department_id = $1 AND LOWER(TRIM(r.name)) = LOWER('User') AND u.deleted_at IS NULL
		LIMIT 1
	`, userSelectFieldsForEntityRepo, userJoinClauseRepo)
	var user entities.User
	row := tx.QueryRow(ctx, query, departmentID)
	err := r.scanUser(row, &user)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("ошибка поиска пользователя в отделе (в транзакции): %w", err)
	}
	return &user, nil
}

func (r *UserRepository) FindUserByID(ctx context.Context, id uint64) (*entities.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE u.id = $1 AND u.deleted_at IS NULL`, userSelectFieldsForEntityRepo, userJoinClauseRepo)
	var user entities.User
	row := r.storage.QueryRow(ctx, query, id)
	if err := r.scanUser(row, &user); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("ошибка поиска пользователя по ID: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) FindUserByEmailOrLogin(ctx context.Context, login string) (*entities.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE u.email = $1 AND u.deleted_at IS NULL`, userSelectFieldsForEntityRepo, userJoinClauseRepo)
	var user entities.User
	row := r.storage.QueryRow(ctx, query, login)
	if err := r.scanUser(row, &user); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("ошибка поиска пользователя по email/login: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) FindUser(ctx context.Context, id uint64) (*entities.User, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM %s WHERE u.id = $1 AND u.deleted_at IS NULL`, userSelectFieldsForEntityRepo, userJoinClauseRepo)
	var user entities.User
	row := r.storage.QueryRow(ctx, query, id)
	if err := r.scanUser(row, &user); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindUserByPhone(ctx context.Context, phone string) (*entities.User, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM %s WHERE phone_number = $1 AND deleted_at IS NULL`, userSelectFieldsForEntityRepo, userJoinClauseRepo)

	var user entities.User
	row := r.storage.QueryRow(ctx, query, phone)
	if err := r.scanUser(row, &user); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, fmt.Errorf("ошибка поиска пользователя по телефону: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uint64, newPasswordHash string) error {
	query := `UPDATE users SET password = $1, updated_at = NOW() WHERE id = $2`
	result, err := r.storage.Exec(ctx, query, newPasswordHash, userID)
	if err != nil {
		return fmt.Errorf("ошибка обновления пароля: %w", err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *UserRepository) CreateUser(ctx context.Context, entity *entities.User) (*entities.User, error) {
	query := fmt.Sprintf(`
        WITH ins AS (
            INSERT INTO %s (fio, email, phone_number, password, position, status_id, role_id, branch_id, department_id, office_id, otdel_id)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
            RETURNING id
        )
        SELECT %s FROM %s WHERE u.id = (SELECT id FROM ins)
    `, userTableRepo, userSelectFieldsForEntityRepo, userJoinClauseRepo)

	createdEntity := &entities.User{}
	row := r.storage.QueryRow(ctx, query,
		entity.Fio,
		entity.Email,
		entity.PhoneNumber,
		entity.Password,
		entity.Position,
		entity.StatusID,
		entity.RoleID,
		entity.BranchID,
		entity.DepartmentID,
		entity.OfficeID,
		entity.OtdelID,
	)

	if err := r.scanUser(row, createdEntity); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" {
				if strings.Contains(pgErr.ConstraintName, "users_email_key") {
					return nil, fmt.Errorf("email already exists: %w", apperrors.ErrBadRequest)
				}
				if strings.Contains(pgErr.ConstraintName, "users_phone_number_key") {
					return nil, fmt.Errorf("phone number already exists: %w", apperrors.ErrBadRequest)
				}
			}
			if pgErr.Code == "23503" {
				return nil, fmt.Errorf("foreign key constraint violation: %w", apperrors.ErrBadRequest)
			}
		}
		return nil, err
	}
	return createdEntity, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, entity *entities.User) (*entities.User, error) {
	query := fmt.Sprintf(`
		WITH upd AS (
			UPDATE %s
			SET fio = $1, email = $2, phone_number = $3, password = $4, position = $5, 
				status_id = $6, role_id = $7, branch_id = $8, department_id = $9, 
				office_id = $10, otdel_id = $11, updated_at = CURRENT_TIMESTAMP
			WHERE id = $12 AND deleted_at IS NULL
			RETURNING id
		)
		SELECT %s FROM %s WHERE u.id = (SELECT id FROM upd)
	`, userTableRepo, userSelectFieldsForEntityRepo, userJoinClauseRepo)

	updatedEntity := &entities.User{}
	row := r.storage.QueryRow(ctx, query,
		entity.Fio,
		entity.Email,
		entity.PhoneNumber,
		entity.Password,
		entity.Position,
		entity.StatusID,
		entity.RoleID,
		entity.BranchID,
		entity.DepartmentID,
		entity.OfficeID,
		entity.OtdelID,
		entity.ID,
	)

	if err := r.scanUser(row, updatedEntity); err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" {
				if strings.Contains(pgErr.ConstraintName, "users_email_key") {
					return nil, fmt.Errorf("email already exists: %w", apperrors.ErrBadRequest)
				}
				if strings.Contains(pgErr.ConstraintName, "users_phone_number_key") {
					return nil, fmt.Errorf("phone number already exists: %w", apperrors.ErrBadRequest)
				}
			}
			if pgErr.Code == "23503" {
				return nil, fmt.Errorf("foreign key constraint violation: %w", apperrors.ErrBadRequest)
			}
		}
		return nil, err
	}
	return updatedEntity, nil
}

func (r *UserRepository) DeleteUser(ctx context.Context, id uint64) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND deleted_at IS NULL
	`, userTableRepo)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

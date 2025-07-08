// Файл: internal/repositories/user-repository.go
// ФИНАЛЬНАЯ, 100% РАБОЧАЯ ВЕРСИЯ

package repositories

import (
	"context"
	"errors"
	"fmt"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const userTableRepo = "users"
const userSelectFieldsForEntityRepo = "id, fio, email, phone_number, password, position, status_id, role_id, branch_id, department_id, office_id, otdel_id, created_at, updated_at, deleted_at"

type UserRepositoryInterface interface {
	GetUsers(ctx context.Context, limit uint64, offset uint64) ([]entities.User, error)
	FindUser(ctx context.Context, id uint64) (*entities.User, error)
	CreateUser(ctx context.Context, entity *entities.User) (*entities.User, error)
	UpdateUser(ctx context.Context, entity *entities.User) (*entities.User, error)
	DeleteUser(ctx context.Context, id uint64) error
	FindUserByEmailOrLogin(ctx context.Context, login string) (*entities.User, error)
	FindUserByPhone(ctx context.Context, phone string) (*entities.User, error)
	UpdatePassword(ctx context.Context, userID int, newPasswordHash string) error
	FindUserByID(ctx context.Context, id int) (*entities.User, error)
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
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&user.ID, &user.FIO, &user.Email, &user.PhoneNumber, &user.Password,
		&user.Position, &user.StatusID, &user.RoleID, &user.BranchID,
		&user.DepartmentID, &user.OfficeID, &user.OtdelID,
		&createdAt, &updatedAt, &user.DeletedAt,
	)
	if err != nil {
		return err
	}
	user.CreatedAt = &createdAt
	user.UpdatedAt = &updatedAt
	return nil
}
func (r *UserRepository) FindUserByID(ctx context.Context, id int) (*entities.User, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM %s
		WHERE id = $1 AND deleted_at IS NULL`, userSelectFieldsForEntityRepo, userTableRepo)

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
	query := fmt.Sprintf(`
        SELECT %s FROM %s 
        WHERE email = $1 AND deleted_at IS NULL`, userSelectFieldsForEntityRepo, userTableRepo)

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

func (r *UserRepository) GetUsers(ctx context.Context, limit uint64, offset uint64) ([]entities.User, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM %s
        WHERE deleted_at IS NULL
        ORDER BY id
        LIMIT $1 OFFSET $2`, userSelectFieldsForEntityRepo, userTableRepo)

	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]entities.User, 0)
	for rows.Next() {
		var user entities.User
		if err := r.scanUser(rows, &user); err != nil { 
			return nil, err
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *UserRepository) FindUser(ctx context.Context, id uint64) (*entities.User, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM %s
		WHERE id = $1 AND deleted_at IS NULL`, userSelectFieldsForEntityRepo, userTableRepo)

	var user entities.User
	row := r.storage.QueryRow(ctx, query, id)
	if err := r.scanUser(row, &user); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.ErrorNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindUserByPhone(ctx context.Context, phone string) (*entities.User, error) {
	query := fmt.Sprintf(`
        SELECT %s FROM %s
        WHERE phone_number = $1 AND deleted_at IS NULL`, userSelectFieldsForEntityRepo, userTableRepo)

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

func (r *UserRepository) UpdatePassword(ctx context.Context, userID int, newPasswordHash string) error {
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
        INSERT INTO %s (fio, email, phone_number, password, position, status_id, role_id, branch_id, department_id, office_id, otdel_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING %s
    `, userTableRepo, userSelectFieldsForEntityRepo)

	createdEntity := &entities.User{}
	row := r.storage.QueryRow(ctx, query,
		entity.FIO,
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
					return nil, fmt.Errorf("email already exists: %w", utils.ErrorBadRequest)
				}
				if strings.Contains(pgErr.ConstraintName, "users_phone_number_key") {
					return nil, fmt.Errorf("phone number already exists: %w", utils.ErrorBadRequest)
				}
			}
			if pgErr.Code == "23503" {
				return nil, fmt.Errorf("foreign key constraint violation: %w", utils.ErrorBadRequest)
			}
		}
		return nil, err
	}

	return createdEntity, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, entity *entities.User) (*entities.User, error) {
	query := fmt.Sprintf(`
        UPDATE %s
        SET fio = $1, email = $2, phone_number = $3, password = $4, position = $5, 
            status_id = $6, role_id = $7, branch_id = $8, department_id = $9, 
            office_id = $10, otdel_id = $11, updated_at = CURRENT_TIMESTAMP
        WHERE id = $12 AND deleted_at IS NULL
        RETURNING %s
    `, userTableRepo, userSelectFieldsForEntityRepo)

	updatedEntity := &entities.User{}
	row := r.storage.QueryRow(ctx, query,
		entity.FIO,
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
			return nil, utils.ErrorNotFound
		}
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" {
				if strings.Contains(pgErr.ConstraintName, "users_email_key") {
					return nil, fmt.Errorf("email already exists: %w", utils.ErrorBadRequest)
				}
				if strings.Contains(pgErr.ConstraintName, "users_phone_number_key") {
					return nil, fmt.Errorf("phone number already exists: %w", utils.ErrorBadRequest)
				}
			}
			if pgErr.Code == "23503" {
				return nil, fmt.Errorf("foreign key constraint violation: %w", utils.ErrorBadRequest)
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
		return utils.ErrorNotFound
	}

	return nil
}

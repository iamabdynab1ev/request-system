package repositories

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/pkg/utils"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepositoryInterface описывает контракт для репозитория пользователей.
type UserRepositoryInterface interface {
	GetUsers(ctx context.Context, limit uint64, offset uint64) ([]*entities.User, error)
	FindUser(ctx context.Context, id uint64) (*entities.User, error)
	FindUserByEmail(ctx context.Context, email string) (*entities.User, error)
	CreateUser(ctx context.Context, entity *entities.User) (*entities.User, error)
	UpdateUser(ctx context.Context, userID uint64, dto dto.UpdateUserDTO) (*entities.User, error)
	DeleteUser(ctx context.Context, id uint64) error
	FindHeadByDepartmentID(ctx context.Context, departmentID int) (*entities.User, error)
	UserHasPermission(ctx context.Context, userID int, permissionName string) (bool, error)
}

// UserRepository реализует UserRepositoryInterface.
type UserRepository struct {
	storage *pgxpool.Pool
	sq      squirrel.StatementBuilderType
}

func NewUserRepository(storage *pgxpool.Pool) UserRepositoryInterface {
	return &UserRepository{
		storage: storage,
		sq:      squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

// scanUserRow - helper для сканирования полной строки пользователя.
// Принимает pgx.Row (интерфейс, который удовлетворяет и QueryRow, и одна строка из Query).
func (r *UserRepository) scanUserRow(row pgx.Row) (*entities.User, error) {
	var user entities.User
	err := row.Scan(
		&user.ID, &user.FIO, &user.Email, &user.Password, &user.PhoneNumber,
		&user.Position, &user.StatusID, &user.RoleID, &user.DepartmentID,
		&user.BranchID, &user.OfficeID, &user.OtdelID,
		&user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
		&user.StatusName, &user.RoleName, &user.DepartmentName,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, fmt.Errorf("failed to scan user row: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) GetUsers(ctx context.Context, limit uint64, offset uint64) ([]*entities.User, error) {
	query := `
		SELECT
			u.id, u.fio, u.email, u.password, u.phone_number, u.position,
			u.status_id, u.role_id, u.department_id,
			u.branch_id, u.office_id, u.otdel_id,
			u.created_at, u.updated_at, u.deleted_at,
			s.name as status_name, rl.name as role_name, d.name as department_name
		FROM users u
			LEFT JOIN statuses s ON u.status_id = s.id
			LEFT JOIN roles rl ON u.role_id = rl.id
			LEFT JOIN departments d ON u.department_id = d.id
        WHERE u.deleted_at IS NULL ORDER BY u.id ASC
        LIMIT $1 OFFSET $2
	`
	rows, err := r.storage.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()
	users := make([]*entities.User, 0)
	for rows.Next() {
		user := new(entities.User)
		err := rows.Scan(
			&user.ID, &user.FIO, &user.Email, &user.Password, &user.PhoneNumber,
			&user.Position, &user.StatusID, &user.RoleID, &user.DepartmentID,
			&user.BranchID, &user.OfficeID, &user.OtdelID,
			&user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
			&user.StatusName, &user.RoleName, &user.DepartmentName,
		)
		if err != nil {

			continue
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during user rows iteration: %w", err)
	}

	return users, nil
}
func (r *UserRepository) FindUser(ctx context.Context, id uint64) (*entities.User, error) {
	query := `
		SELECT
			u.id, u.fio, u.email, u.password, u.phone_number, u.position,
			u.status_id, u.role_id, u.department_id,
			u.branch_id, u.office_id, u.otdel_id,
			u.created_at, u.updated_at, u.deleted_at,
			s.name as status_name, rl.name as role_name, d.name as department_name
		FROM users u
			LEFT JOIN statuses s ON u.status_id = s.id
			LEFT JOIN roles rl ON u.role_id = rl.id
			LEFT JOIN departments d ON u.department_id = d.id
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`
	row := r.storage.QueryRow(ctx, query, id)
	return r.scanUserRow(row)
}

// FindUserByEmail находит пользователя по email. JOIN'ы здесь не нужны для логина.
func (r *UserRepository) FindUserByEmail(ctx context.Context, email string) (*entities.User, error) {
	query := `
		SELECT
			u.id, u.fio, u.email, u.password, u.phone_number, u.position,
			u.status_id, u.role_id, u.department_id,
			u.branch_id, u.office_id, u.otdel_id,
			u.created_at, u.updated_at, u.deleted_at,
			-- Заглушки, чтобы структура совпадала для scanUserRow
			'' as status_name, '' as role_name, '' as department_name
		FROM users u
		WHERE u.email = $1 AND u.deleted_at IS NULL
	`
	row := r.storage.QueryRow(ctx, query, email)
	return r.scanUserRow(row)
}

// CreateUser создает нового пользователя и возвращает его.
func (r *UserRepository) CreateUser(ctx context.Context, entity *entities.User) (*entities.User, error) {
	query := `
        INSERT INTO users (fio, email, phone_number, password, position, status_id, role_id, department_id, branch_id, office_id, otdel_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING id, fio, email, phone_number, password, position, status_id, role_id, department_id,
                  branch_id, office_id, otdel_id, created_at, updated_at, deleted_at,
                  '' as status_name, '' as role_name, '' as department_name
    `
	row := r.storage.QueryRow(ctx, query,
		entity.FIO, entity.Email, entity.PhoneNumber, entity.Password,
		entity.Position, entity.StatusID, entity.RoleID, entity.DepartmentID,
		entity.BranchID, entity.OfficeID, entity.OtdelID,
	)
	return r.scanUserRow(row)
}

// UpdateUser частично обновляет пользователя на основе DTO.
func (r *UserRepository) UpdateUser(ctx context.Context, userID uint64, dto dto.UpdateUserDTO) (*entities.User, error) {
	updateBuilder := r.sq.Update("users").
		Where(squirrel.Eq{"id": userID, "deleted_at": nil}).
		Set("updated_at", squirrel.Expr("CURRENT_TIMESTAMP")).
		Suffix(`RETURNING id, fio, email, phone_number, password, position, status_id, role_id, department_id,
                        branch_id, office_id, otdel_id, created_at, updated_at, deleted_at,
                        '' as status_name, '' as role_name, '' as department_name`)

	if dto.Fio != nil {
		updateBuilder = updateBuilder.Set("fio", *dto.Fio)
	}
	if dto.Email != nil {
		updateBuilder = updateBuilder.Set("email", *dto.Email)
	}
	if dto.PhoneNumber != nil {
		updateBuilder = updateBuilder.Set("phone_number", *dto.PhoneNumber)
	}
	if dto.Password != nil {
		updateBuilder = updateBuilder.Set("password", *dto.Password)
	}
	if dto.Position != nil {
		updateBuilder = updateBuilder.Set("position", *dto.Position)
	}
	if dto.StatusID != nil {
		updateBuilder = updateBuilder.Set("status_id", *dto.StatusID)
	}
	if dto.DepartmentID != nil {
		updateBuilder = updateBuilder.Set("department_id", *dto.DepartmentID)
	}
	if dto.BranchID != nil {
		updateBuilder = updateBuilder.Set("branch_id", *dto.BranchID)
	}
	if dto.OfficeID != nil {
		updateBuilder = updateBuilder.Set("office_id", *dto.OfficeID)
	}
	if dto.OtdelID != nil {
		updateBuilder = updateBuilder.Set("otdel_id", *dto.OtdelID)
	}

	query, args, err := updateBuilder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build update user query: %w", err)
	}

	row := r.storage.QueryRow(ctx, query, args...)
	return r.scanUserRow(row)
}

// DeleteUser "мягко" удаляет пользователя.
func (r *UserRepository) DeleteUser(ctx context.Context, id uint64) error {
	query := `UPDATE users SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}
	return nil
}

func (r *UserRepository) FindHeadByDepartmentID(ctx context.Context, departmentID int) (*entities.User, error) {

	const headRoleName = "User"

	query := `
		SELECT
			u.id, u.fio, u.email, u.password, u.phone_number, u.position,
			u.status_id, u.role_id, u.department_id,
			u.branch_id, u.office_id, u.otdel_id,
			u.created_at, u.updated_at, u.deleted_at,
			s.name as status_name, rl.name as role_name, d.name as department_name
		FROM users u
		JOIN roles rl ON u.role_id = rl.id
        LEFT JOIN statuses s ON u.status_id = s.id
        LEFT JOIN departments d ON u.department_id = d.id
		WHERE u.department_id = $1 AND rl.name = $2 AND u.deleted_at IS NULL
		LIMIT 1`

	row := r.storage.QueryRow(ctx, query, departmentID, headRoleName)
	return r.scanUserRow(row)
}

// UserHasPermission проверяет права пользователя.
func (r *UserRepository) UserHasPermission(ctx context.Context, userID int, permissionName string) (bool, error) {
	var exists bool
	query := `
         SELECT EXISTS (
             SELECT 1
             FROM users u
             JOIN role_permission rp ON u.role_id = rp.role_id
             JOIN permissions p ON rp.permission_id = p.id
             WHERE u.id = $1 AND p.name = $2 AND u.deleted_at IS NULL
         )`
	err := r.storage.QueryRow(ctx, query, userID, permissionName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("ошибка при проверке прав пользователя: %w", err)
	}
	return exists, nil
}

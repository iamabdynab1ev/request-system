package repositories

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	userTableRepo                 = "users"
	userSelectFieldsForEntityRepo = "u.id, u.fio, u.email, u.phone_number, u.password, u.position, u.status_id, u.photo_url, u.role_id, u.branch_id, u.department_id, u.office_id, u.otdel_id, r.name as role_name, u.created_at, u.updated_at, u.deleted_at"
	userJoinClauseRepo            = "users u JOIN roles r ON u.role_id = r.id"
)

var (
	userAllowedFilterFields = map[string]bool{"status_id": true, "department_id": true, "branch_id": true, "role_id": true, "position": true}
	userAllowedSortFields   = map[string]bool{"id": true, "fio": true, "created_at": true, "updated_at": true}
)

type UserRepositoryInterface interface {
	GetUsers(ctx context.Context, filter types.Filter, securityFilter string, securityArgs []interface{}) ([]entities.User, uint64, error)
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
	FindByEmail(ctx context.Context, email string) (*entities.User, error)
}

type UserRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewUserRepository(storage *pgxpool.Pool, logger *zap.Logger) UserRepositoryInterface {
	return &UserRepository{storage: storage, logger: logger}
}

func scanUser(row pgx.Row) (*entities.User, error) {
	var user entities.User
	err := row.Scan(
		&user.ID, &user.Fio, &user.Email, &user.PhoneNumber, &user.Password,
		&user.Position, &user.StatusID, &user.PhotoURL,
		&user.RoleID, &user.BranchID,
		&user.DepartmentID, &user.OfficeID, &user.OtdelID, &user.RoleName,
		&user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetUsers(ctx context.Context, filter types.Filter, securityFilter string, securityArgs []interface{}) ([]entities.User, uint64, error) {
	// Эталонная реализация, как в OrderRepository

	allArgs := make([]interface{}, 0)
	conditions := []string{"u.deleted_at IS NULL"} // Используем алиас 'u' для таблицы users
	placeholderNum := 1

	// Шаг 1: Применяем фильтр безопасности, если он есть
	if securityFilter != "" {
		conditions = append(conditions, securityFilter)
		allArgs = append(allArgs, securityArgs...)
		placeholderNum += len(securityArgs)
	}

	// Шаг 2: Применяем текстовый поиск
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		// Правильный поиск по нескольким полям
		searchCondition := fmt.Sprintf("(u.fio ILIKE $%d OR u.email ILIKE $%d OR u.phone_number ILIKE $%d)",
			placeholderNum, placeholderNum+1, placeholderNum+2)
		conditions = append(conditions, searchCondition)
		allArgs = append(allArgs, searchPattern, searchPattern, searchPattern) // Передаем значение 3 раза для 3-х плейсхолдеров
		placeholderNum += 3
	}

	// Шаг 3: Применяем фильтры по конкретным полям
	for key, value := range filter.Filter {
		if !userAllowedFilterFields[key] {
			continue // Пропускаем поля, которых нет в "белом списке"
		}

		if strVal, ok := value.(string); ok && strings.Contains(strVal, ",") {
			// Обработка нескольких значений (например, filter[role_id]=1,2)
			items := strings.Split(strVal, ",")
			placeholders := make([]string, len(items))
			for i, item := range items {
				placeholders[i] = fmt.Sprintf("$%d", placeholderNum)
				allArgs = append(allArgs, item)
				placeholderNum++
			}
			conditions = append(conditions, fmt.Sprintf("u.%s IN (%s)", key, strings.Join(placeholders, ",")))
		} else {
			// Обработка одного значения
			conditions = append(conditions, fmt.Sprintf("u.%s = $%d", key, placeholderNum))
			allArgs = append(allArgs, value)
			placeholderNum++
		}
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	// Шаг 4: Считаем общее количество записей с учетом всех фильтров
	countQuery := fmt.Sprintf("SELECT COUNT(u.id) FROM %s %s", userJoinClauseRepo, whereClause)
	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, allArgs...).Scan(&totalCount); err != nil {
		r.logger.Error("ошибка подсчета пользователей", zap.Error(err), zap.String("query", countQuery))
		return nil, 0, err
	}

	if totalCount == 0 {
		return []entities.User{}, 0, nil
	}

	// Шаг 5: Добавляем сортировку
	orderByClause := "ORDER BY u.id DESC" // Сортировка по умолчанию
	if len(filter.Sort) > 0 {
		var sortParts []string
		for field, direction := range filter.Sort {
			if userAllowedSortFields[field] {
				// Убеждаемся, что direction - это "asc" или "desc", чтобы избежать SQL-инъекций
				safeDirection := "ASC"
				if strings.ToLower(direction) == "desc" {
					safeDirection = "DESC"
				}
				sortParts = append(sortParts, fmt.Sprintf("u.%s %s", field, safeDirection))
			}
		}
		if len(sortParts) > 0 {
			orderByClause = "ORDER BY " + strings.Join(sortParts, ", ")
		}
	}

	// Шаг 6: Добавляем пагинацию, если она включена
	limitClause := ""
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", placeholderNum, placeholderNum+1)
		allArgs = append(allArgs, filter.Limit, filter.Offset)
	}

	// Шаг 7: Собираем и выполняем основной запрос
	mainQuery := fmt.Sprintf("SELECT %s FROM %s %s %s %s", userSelectFieldsForEntityRepo, userJoinClauseRepo, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, mainQuery, allArgs...)
	if err != nil {
		r.logger.Error("ошибка получения пользователей", zap.Error(err), zap.String("query", mainQuery))
		return nil, 0, err
	}
	defer rows.Close()

	users := make([]entities.User, 0)
	for rows.Next() {
		user, err := scanUser(rows) // scanUser у вас уже есть и он правильный
		if err != nil {
			return nil, 0, err
		}
		users = append(users, *user)
	}
	return users, totalCount, rows.Err()
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*entities.User, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE u.email = $1 AND u.deleted_at IS NULL LIMIT 1", userSelectFieldsForEntityRepo, userJoinClauseRepo)
	row := r.storage.QueryRow(ctx, query, email)
	return scanUser(row)
}

func (r *UserRepository) FindHeadByDepartment(ctx context.Context, departmentID uint64) (*entities.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE u.department_id = $1 AND LOWER(TRIM(r.name)) = LOWER('User') AND u.deleted_at IS NULL LIMIT 1`, userSelectFieldsForEntityRepo, userJoinClauseRepo)
	row := r.storage.QueryRow(ctx, query, departmentID)
	return scanUser(row)
}

func (r *UserRepository) FindHeadByDepartmentInTx(ctx context.Context, tx pgx.Tx, departmentID uint64) (*entities.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE u.department_id = $1 AND LOWER(TRIM(r.name)) = LOWER('User') AND u.deleted_at IS NULL LIMIT 1`, userSelectFieldsForEntityRepo, userJoinClauseRepo)
	row := tx.QueryRow(ctx, query, departmentID)
	return scanUser(row)
}

func (r *UserRepository) FindUserByID(ctx context.Context, id uint64) (*entities.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE u.id = $1 AND u.deleted_at IS NULL`, userSelectFieldsForEntityRepo, userJoinClauseRepo)
	row := r.storage.QueryRow(ctx, query, id)
	return scanUser(row)
}

func (r *UserRepository) FindUserByEmailOrLogin(ctx context.Context, login string) (*entities.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE u.email = $1 AND u.deleted_at IS NULL`, userSelectFieldsForEntityRepo, userJoinClauseRepo)
	row := r.storage.QueryRow(ctx, query, login)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrInvalidCredentials
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) FindUser(ctx context.Context, id uint64) (*entities.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE u.id = $1 AND u.deleted_at IS NULL`, userSelectFieldsForEntityRepo, userJoinClauseRepo)
	row := r.storage.QueryRow(ctx, query, id)
	return scanUser(row)
}

func (r *UserRepository) FindUserByPhone(ctx context.Context, phone string) (*entities.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE u.phone_number = $1 AND u.deleted_at IS NULL`, userSelectFieldsForEntityRepo, userJoinClauseRepo)
	row := r.storage.QueryRow(ctx, query, phone)
	return scanUser(row)
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

func (r *UserRepository) CreateUser(ctx context.Context, entity *entities.User) (*entities.User, error) {
	query := fmt.Sprintf(`
        WITH ins AS (
            INSERT INTO %s (fio, email, phone_number, password, position, status_id, role_id, branch_id, department_id, office_id, otdel_id, photo_url)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING id
        ) SELECT %s FROM %s WHERE u.id = (SELECT id FROM ins)
    `, userTableRepo, userSelectFieldsForEntityRepo, userJoinClauseRepo)

	row := r.storage.QueryRow(ctx, query,
		entity.Fio, entity.Email, entity.PhoneNumber, entity.Password, entity.Position,
		entity.StatusID, entity.RoleID, entity.BranchID, entity.DepartmentID,
		entity.OfficeID, entity.OtdelID, entity.PhotoURL,
	)

	createdEntity, err := scanUser(row)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" {
				if strings.Contains(pgErr.ConstraintName, "users_email_key") {
					return nil, apperrors.NewHttpError(http.StatusBadRequest, "Email уже используется.", err)
				}
				if strings.Contains(pgErr.ConstraintName, "users_phone_number_key") {
					return nil, apperrors.NewHttpError(http.StatusBadRequest, "Номер телефона уже используется.", err)
				}
			}
			if pgErr.Code == "23503" {
				return nil, apperrors.NewHttpError(http.StatusBadRequest, "Нарушение внешнего ключа (неверный ID роли, отдела и т.д.).", err)
			}
		}
		return nil, err
	}
	return createdEntity, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, entity *entities.User) (*entities.User, error) {
	query := fmt.Sprintf(`
		WITH upd AS (
			UPDATE %s SET fio = $1, email = $2, phone_number = $3, password = $4, position = $5, 
			status_id = $6, role_id = $7, branch_id = $8, department_id = $9, office_id = $10, 
			otdel_id = $11, photo_url = $12, updated_at = CURRENT_TIMESTAMP
			WHERE id = $13 AND deleted_at IS NULL RETURNING id
		) SELECT %s FROM %s WHERE u.id = (SELECT id FROM upd)
	`, userTableRepo, userSelectFieldsForEntityRepo, userJoinClauseRepo)

	row := r.storage.QueryRow(ctx, query,
		entity.Fio, entity.Email, entity.PhoneNumber, entity.Password, entity.Position,
		entity.StatusID, entity.RoleID, entity.BranchID, entity.DepartmentID,
		entity.OfficeID, entity.OtdelID, entity.PhotoURL, entity.ID,
	)

	return scanUser(row)
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

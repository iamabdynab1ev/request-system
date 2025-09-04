package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

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
	userTableRepo = "users"
	// Обновляем SELECT и JOIN, добавляя s.code as status_code
	userSelectFieldsForEntityRepo = "u.id, u.fio, u.email, u.phone_number, u.password, u.position, u.status_id, s.code as status_code, u.photo_url, u.role_id, u.branch_id, u.department_id, u.office_id, u.otdel_id, r.name as role_name, u.created_at, u.updated_at, u.deleted_at, u.must_change_password"
	userJoinClauseRepo            = "users u JOIN roles r ON u.role_id = r.id JOIN statuses s ON u.status_id = s.id"
)

var (
	userAllowedFilterFields = map[string]bool{"status_id": true, "department_id": true, "branch_id": true, "role_id": true, "position": true}
	userAllowedSortFields   = map[string]bool{"id": true, "fio": true, "created_at": true, "updated_at": true}
)

type UserRepositoryInterface interface {
	GetUsers(ctx context.Context, filter types.Filter, securityFilter string, securityArgs []interface{}) ([]entities.User, uint64, error)
	FindUser(ctx context.Context, id uint64) (*entities.User, error)
	CreateUser(ctx context.Context, entity *entities.User) (*entities.User, error)
	UpdateUser(ctx context.Context, payload dto.UpdateUserDTO) (*entities.User, error)
	DeleteUser(ctx context.Context, id uint64) error
	FindUserByEmailOrLogin(ctx context.Context, login string) (*entities.User, error)
	FindUserByPhone(ctx context.Context, phone string) (*entities.User, error)
	UpdatePassword(ctx context.Context, userID uint64, newPasswordHash string) error
	UpdatePasswordAndClearFlag(ctx context.Context, userID uint64, newPasswordHash string) error
	FindUserByID(ctx context.Context, id uint64) (*entities.User, error)
	FindHeadByDepartment(ctx context.Context, departmentID uint64) (*entities.User, error)
	FindHeadByDepartmentInTx(ctx context.Context, tx pgx.Tx, departmentID uint64) (*entities.User, error)
	FindByEmail(ctx context.Context, email string) (*entities.User, error)
	FindUsersByIDs(ctx context.Context, userIDs []uint64) (map[uint64]entities.User, error)
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
	var createdAt, updatedAt, deletedAt sql.NullTime

	// Сканнер остается почти таким же
	err := row.Scan(
		&user.ID, &user.Fio, &user.Email, &user.PhoneNumber, &user.Password,
		&user.Position, &user.StatusID, &user.StatusCode, &user.PhotoURL,
		&user.RoleID, &user.BranchID,
		&user.DepartmentID, &user.OfficeID, &user.OtdelID, &user.RoleName,
		&createdAt, &updatedAt, &deletedAt, // Сканируем в sql.NullTime
		&user.MustChangePassword,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		// Важно залогировать ошибку сканирования, чтобы в будущем сразу видеть проблему
		fmt.Printf("SCAN ERROR: %v\n", err) // Временный лог для отладки
		return nil, err
	}

	// Теперь безопасно преобразуем sql.NullTime в *time.Time
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

func (r *UserRepository) FindUsersByIDs(ctx context.Context, userIDs []uint64) (map[uint64]entities.User, error) {
	if len(userIDs) == 0 {
		return make(map[uint64]entities.User), nil
	}

	// Делаем SELECT только на те поля, которые нужны в списке заявок: ID и ФИО
	// Мы не используем здесь userSelectFieldsForEntityRepo и JOIN, т.к. нам не нужны все данные
	query := `SELECT u.id, u.fio FROM users u WHERE u.id = ANY($1) AND u.deleted_at IS NULL`

	rows, err := r.storage.Query(ctx, query, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	usersMap := make(map[uint64]entities.User)
	for rows.Next() {
		var user entities.User
		// Сканируем только ID и ФИО, т.к. только их и запрашивали
		if err := rows.Scan(&user.ID, &user.Fio); err != nil {
			return nil, err
		}
		usersMap[user.ID] = user
	}

	return usersMap, rows.Err()
}

func (r *UserRepository) CreateUser(ctx context.Context, entity *entities.User) (*entities.User, error) {
	builder := sq.Insert(userTableRepo).
		PlaceholderFormat(sq.Dollar).
		Columns(
			"fio", "email", "phone_number", "password", "position", "status_id", "role_id",
			"branch_id", "department_id", "office_id", "otdel_id", "photo_url", "created_at",
			"updated_at", "must_change_password",
		)

	builder = builder.Values(
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
		entity.PhotoURL,
		entity.CreatedAt,
		entity.UpdatedAt,
		entity.MustChangePassword,
	).Suffix("RETURNING id, role_id, status_id")

	query, args, err := builder.ToSql()
	if err != nil {
		r.logger.Error("UserRepository.CreateUser: ошибка при сборке SQL-запроса", zap.Error(err))
		return nil, err
	}

	// Хитрый трюк для RETURNING с JOIN'ами
	var createdID, roleID, statusID uint64
	err = r.storage.QueryRow(ctx, query, args...).Scan(&createdID, &roleID, &statusID)
	if err != nil {
		// ... (блок обработки ошибок pgconn остается тот же)
		if pgErr, ok := err.(*pgconn.PgError); ok {
			switch pgErr.Code {
			case "23505":
				if strings.Contains(pgErr.ConstraintName, "users_email_key") {
					return nil, apperrors.NewHttpError(http.StatusBadRequest, "Email уже используется", err, map[string]interface{}{"email": entity.Email})
				}
				if strings.Contains(pgErr.ConstraintName, "users_phone_number_key") {
					return nil, apperrors.NewHttpError(http.StatusBadRequest, "Номер телефона уже используется", err, map[string]interface{}{"phone": entity.PhoneNumber})
				}
			case "23503":
				errorMessage := fmt.Sprintf("Неверный ID внешнего ключа: %s. Убедитесь, что связанные записи существуют.", pgErr.ConstraintName)
				return nil, apperrors.NewHttpError(http.StatusBadRequest, errorMessage, err, map[string]interface{}{"constraint": pgErr.ConstraintName})
			}
		}
		r.logger.Error("UserRepository.CreateUser: неожиданная ошибка при создании пользователя", zap.String("query", query), zap.Error(err))
		return nil, apperrors.ErrInternal
	}

	// После успешного INSERT делаем SELECT, чтобы получить все данные с JOIN'ами
	return r.FindUserByID(ctx, createdID)
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

func (r *UserRepository) GetUsers(ctx context.Context, filter types.Filter, securityFilter string, securityArgs []interface{}) ([]entities.User, uint64, error) {
	allArgs := make([]interface{}, 0)
	conditions := []string{"u.deleted_at IS NULL"}
	placeholderNum := 1

	if securityFilter != "" {
		for i := 0; i < strings.Count(securityFilter, "?"); i++ {
			securityFilter = strings.Replace(securityFilter, "?", fmt.Sprintf("$%d", placeholderNum), 1)
			placeholderNum++
		}
		conditions = append(conditions, securityFilter)
		allArgs = append(allArgs, securityArgs...)
	}

	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%"
		searchCondition := fmt.Sprintf("(u.fio ILIKE $%d OR u.email::text ILIKE $%d OR u.phone_number::text ILIKE $%d)",
			placeholderNum, placeholderNum+1, placeholderNum+2)
		conditions = append(conditions, searchCondition)
		allArgs = append(allArgs, searchPattern, searchPattern, searchPattern)
		placeholderNum += 3
	}

	for key, value := range filter.Filter {
		if !userAllowedFilterFields[key] {
			continue
		}
		switch v := value.(type) {
		case []string:
			placeholders := make([]string, len(v))
			for i, item := range v {
				placeholders[i] = fmt.Sprintf("$%d", placeholderNum)
				allArgs = append(allArgs, item)
				placeholderNum++
			}
			conditions = append(conditions, fmt.Sprintf("u.%s IN (%s)", key, strings.Join(placeholders, ",")))
		case string:
			conditions = append(conditions, fmt.Sprintf("u.%s = $%d", key, placeholderNum))
			allArgs = append(allArgs, v)
			placeholderNum++
		}
	}
	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(u.id) FROM %s %s", userJoinClauseRepo, whereClause)
	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, allArgs...).Scan(&totalCount); err != nil {
		r.logger.Error("ошибка подсчета пользователей", zap.Error(err), zap.String("query", countQuery))
		return nil, 0, err
	}

	if totalCount == 0 {
		return []entities.User{}, 0, nil
	}

	orderByClause := "ORDER BY u.id DESC"
	if len(filter.Sort) > 0 {
		var sortParts []string
		for field, direction := range filter.Sort {
			if userAllowedSortFields[field] {
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

	limitClause := ""
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", placeholderNum, placeholderNum+1)
		allArgs = append(allArgs, filter.Limit, filter.Offset)
	}

	mainQuery := fmt.Sprintf("SELECT %s FROM %s %s %s %s", userSelectFieldsForEntityRepo, userJoinClauseRepo, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, mainQuery, allArgs...)
	if err != nil {
		r.logger.Error("ошибка получения пользователей", zap.Error(err), zap.String("query", mainQuery))
		return nil, 0, err
	}
	defer rows.Close()

	users := make([]entities.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
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
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE LOWER(u.email) = LOWER($1) AND u.deleted_at IS NULL`, userSelectFieldsForEntityRepo, userJoinClauseRepo)

	row := r.storage.QueryRow(ctx, query, login) // Передаем login как есть, SQL сам сделает lowercase

	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.NewHttpError(http.StatusUnauthorized, "Неверные учетные данные", err, nil)
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

func (r *UserRepository) UpdateUser(ctx context.Context, payload dto.UpdateUserDTO) (*entities.User, error) {
	// Сначала получаем ID обновляемой записи
	updateBuilder := sq.Update(userTableRepo).
		PlaceholderFormat(sq.Dollar).
		Where(sq.Eq{"id": payload.ID, "deleted_at": nil}).
		Set("updated_at", sq.Expr("CURRENT_TIMESTAMP"))

	hasChanges := false
	if payload.Fio != nil {
		updateBuilder = updateBuilder.Set("fio", *payload.Fio)
		hasChanges = true
	}

	if payload.Email != nil {
		updateBuilder = updateBuilder.Set("email", *payload.Email)
		hasChanges = true
	}
	if payload.PhoneNumber != nil {
		updateBuilder = updateBuilder.Set("phone_number", *payload.PhoneNumber)
		hasChanges = true
	}
	if payload.Position != nil {
		updateBuilder = updateBuilder.Set("position", *payload.Position)
		hasChanges = true
	}
	if payload.StatusID != nil {
		updateBuilder = updateBuilder.Set("status_id", *payload.StatusID)
		hasChanges = true
	}
	if payload.RoleID != nil {
		updateBuilder = updateBuilder.Set("role_id", *payload.RoleID)
		hasChanges = true
	}
	if payload.BranchID != nil {
		updateBuilder = updateBuilder.Set("branch_id", *payload.BranchID)
		hasChanges = true
	}
	if payload.DepartmentID != nil {
		updateBuilder = updateBuilder.Set("department_id", *payload.DepartmentID)
		hasChanges = true
	}
	if payload.OfficeID != nil {
		updateBuilder = updateBuilder.Set("office_id", *payload.OfficeID)
		hasChanges = true
	}
	if payload.OtdelID != nil {
		updateBuilder = updateBuilder.Set("otdel_id", *payload.OtdelID)
		hasChanges = true
	}
	if payload.PhotoURL != nil {
		updateBuilder = updateBuilder.Set("photo_url", *payload.PhotoURL)
		hasChanges = true
	}

	if !hasChanges {
		return r.FindUser(ctx, payload.ID)
	}

	updateBuilder = updateBuilder.Suffix("RETURNING id")
	query, args, err := updateBuilder.ToSql()
	if err != nil {
		return nil, err
	}

	var updatedID uint64
	err = r.storage.QueryRow(ctx, query, args...).Scan(&updatedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		// ...(обработка pgconn ошибок как в CreateUser)
		if pgErr, ok := err.(*pgconn.PgError); ok {
			switch pgErr.Code {
			case "23505": // Конфликт уникального индекса
				if strings.Contains(pgErr.ConstraintName, "users_email_key") {
					return nil, apperrors.NewHttpError(http.StatusBadRequest, "Email уже используется", err, nil)
				}
				if strings.Contains(pgErr.ConstraintName, "users_phone_number_key") {
					return nil, apperrors.NewHttpError(http.StatusBadRequest, "Номер телефона уже используется", err, nil)
				}
			case "23503": // Ошибка внешнего ключа
				errorMessage := fmt.Sprintf("Неверный ID внешнего ключа: %s. Убедитесь, что связанные записи существуют.", pgErr.ConstraintName)
				return nil, apperrors.NewHttpError(http.StatusBadRequest, errorMessage, err, nil)
			}
		}

		return nil, err
	}
	return r.FindUserByID(ctx, updatedID)
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

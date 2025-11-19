package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
		"u.created_at", "u.updated_at", "u.deleted_at", "u.must_change_password", "u.is_head", "u.telegram_chat_id",
		"u.external_id", "u.source_system",
	}
	userAllowedFilterFields = map[string]string{
		"department_id": "u.department_id",
		"branch_id":     "u.branch_id",
		"position_id":   "u.position_id",
		"otdel_id":      "u.otdel_id",
		"office_id":     "u.office_id",
	}
	userAllowedSortFields = map[string]bool{
		"id":         true,
		"fio":        true,
		"created_at": true,
		"updated_at": true,
		"status_id":  true,
	}
)

type UserRepositoryInterface interface {
	//  --- Sync ---
	CreateFromSync(ctx context.Context, tx pgx.Tx, user entities.User) (uint64, error)
	UpdateFromSync(ctx context.Context, tx pgx.Tx, id uint64, user entities.User) error
	FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.User, error)
	// --- Основной CRUD ---
	GetUsers(ctx context.Context, filter types.Filter) ([]entities.User, uint64, error)
	FindUserByID(ctx context.Context, id uint64) (*entities.User, error)
	FindUserByIDInTx(ctx context.Context, tx pgx.Tx, id uint64) (*entities.User, error)
	CreateUser(ctx context.Context, tx pgx.Tx, user *entities.User) (uint64, error)
	UpdateUserFull(ctx context.Context, user *entities.User) error
	UpdateUser(ctx context.Context, tx pgx.Tx, payload dto.UpdateUserDTO, rawRequestBody []byte) error
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

	// --- Методы-хелперы для Сервисов ---
	FindActiveUsersByPositionTypeAndOrg(ctx context.Context, tx pgx.Tx, posType string, depID *uint64, otdelID *uint64) ([]entities.User, error)
	BeginTx(ctx context.Context) (pgx.Tx, error)
	FindUsersByIDs(ctx context.Context, userIDs []uint64) (map[uint64]entities.User, error)
	IsHeadExistsInDepartment(ctx context.Context, departmentID uint64, excludeUserID uint64) (bool, error)

	// --- Методы-хелперы для Telegram ---
	UpdateTelegramChatID(ctx context.Context, userID uint64, chatID int64) error
	FindUserByTelegramChatID(ctx context.Context, chatID int64) (*entities.User, error)
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
	var photoURL, externalID, sourceSystem sql.NullString
	var branchID, officeID, otdelID, positionID sql.NullInt64

	err := row.Scan(
		&user.ID, &user.Fio, &user.Email, &user.PhoneNumber, &user.Password,
		&positionID, &user.StatusID, &user.StatusCode, &photoURL, &branchID, &user.DepartmentID,
		&officeID, &otdelID, &createdAt, &updatedAt, &deletedAt,
		&user.MustChangePassword, &user.IsHead, &user.TelegramChatID,
		&externalID, &sourceSystem,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		// Для отладки добавляем лог
		log.Printf("!!! ОШИБКА в scanUserEntity: %v. Проверьте порядок полей в userSelectFields!", err)
		return nil, err
	}

	// Обрабатываем nullable поля
	if createdAt.Valid {
		user.CreatedAt = &createdAt.Time
	}
	if updatedAt.Valid {
		user.UpdatedAt = &updatedAt.Time
	}
	if deletedAt.Valid {
		user.DeletedAt = &deletedAt.Time
	}
	if photoURL.Valid {
		user.PhotoURL = &photoURL.String
	}

	// Конвертируем NullInt64 в *uint64
	if branchID.Valid {
		v := uint64(branchID.Int64)
		user.BranchID = &v
	}
	if officeID.Valid {
		v := uint64(officeID.Int64)
		user.OfficeID = &v
	}
	if otdelID.Valid {
		v := uint64(otdelID.Int64)
		user.OtdelID = &v
	}
	if positionID.Valid {
		v := uint64(positionID.Int64)
		user.PositionID = &v
	}
	if externalID.Valid {
		user.ExternalID = &externalID.String
	}
	if sourceSystem.Valid {
		user.SourceSystem = &sourceSystem.String
	}

	return &user, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, tx pgx.Tx, entity *entities.User) (uint64, error) {
	query := `INSERT INTO users (fio, email, phone_number, password, position_id, status_id, branch_id, 
								 department_id, office_id, otdel_id, photo_url, must_change_password, is_head,
								 created_at, updated_at) 
			  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW()) RETURNING id`
	var createdID uint64
	err := tx.QueryRow(ctx, query,
		entity.Fio, entity.Email, entity.PhoneNumber, entity.Password, entity.PositionID,
		entity.StatusID, entity.BranchID, entity.DepartmentID, entity.OfficeID,
		entity.OtdelID, entity.PhotoURL, entity.MustChangePassword, entity.IsHead,
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

func (r *UserRepository) CreateFromSync(ctx context.Context, tx pgx.Tx, user entities.User) (uint64, error) {
	query := `
		INSERT INTO users (fio, email, phone_number, password, status_id, position_id, department_id, 
		                   otdel_id, branch_id, office_id, external_id, source_system,
						   created_at, updated_at, must_change_password, is_head)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW(), false, false)
		RETURNING id`
	var newID uint64
	err := tx.QueryRow(ctx, query,
		user.Fio, user.Email, user.PhoneNumber, user.Password, user.StatusID, user.PositionID,
		user.DepartmentID, user.OtdelID, user.BranchID, user.OfficeID, user.ExternalID, user.SourceSystem,
	).Scan(&newID)
	// (Обработка ошибок, например, на дубликат email)
	return newID, err
}

func (r *UserRepository) findOneUser(ctx context.Context, querier Querier, where sq.Eq) (*entities.User, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	// Собираем запрос с JOIN'ом, как в ваших других методах
	query, args, err := psql.Select(userSelectFields...).
		From("users u").
		Join("statuses s ON u.status_id = s.id").
		Where(where).
		ToSql()
	if err != nil {
		r.logger.Error("Failed to build SQL for findOneUser", zap.Error(err))
		return nil, fmt.Errorf("ошибка сборки запроса для findOneUser: %w", err)
	}

	row := querier.QueryRow(ctx, query, args...)

	return scanUserEntity(row)
}

func (r *UserRepository) FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.User, error) {
	var querier Querier = r.storage
	if tx != nil {
		querier = tx
	}
	return r.findOneUser(ctx, querier, sq.Eq{"u.external_id": externalID, "u.source_system": sourceSystem})
}

func (r *UserRepository) UpdateFromSync(ctx context.Context, tx pgx.Tx, id uint64, user entities.User) error {
	query := `
		UPDATE users SET 
			fio = $1, email = $2, phone_number = $3, status_id = $4, position_id = $5,
			department_id = $6, otdel_id = $7, branch_id = $8, office_id = $9, 
			updated_at = NOW()
		WHERE id = $10`
	result, err := tx.Exec(ctx, query,
		user.Fio, user.Email, user.PhoneNumber, user.StatusID, user.PositionID,
		user.DepartmentID, user.OtdelID, user.BranchID, user.OfficeID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *UserRepository) UpdateUserFull(ctx context.Context, user *entities.User) error {
	query := `
		UPDATE users SET 
			fio = $1, 
			email = $2,
			department_id = $3,
			position_id = $4,
            updated_at = NOW()
		WHERE id = $5`

	result, err := r.storage.Exec(ctx, query,
		user.Fio,
		user.Email,
		user.DepartmentID,
		user.PositionID,
		user.ID,
	)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return apperrors.NewHttpError(http.StatusConflict, "Email уже используется", err, nil)
		}
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, tx pgx.Tx, payload dto.UpdateUserDTO, rawRequestBody []byte) error {
	builder := sq.Update(userTable).
		PlaceholderFormat(sq.Dollar).
		Where(sq.Eq{"id": payload.ID, "deleted_at": nil}).
		Set("updated_at", time.Now())

	wasFieldSent := func(key string) bool {
		var data map[string]interface{}
		_ = json.Unmarshal(rawRequestBody, &data)
		_, exists := data[key]
		return exists
	}

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
	if wasFieldSent("office_id") {
		builder = builder.Set("office_id", payload.OfficeID)
	}
	if wasFieldSent("otdel_id") {
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

	baseBuilder := psql.Select().From("users u").
		Join("statuses s ON u.status_id = s.id").
		Where(sq.Eq{"u.deleted_at": nil})

	// --- ИСПРАВЛЕННАЯ ФИЛЬТРАЦИЯ (ИСПОЛЬЗУЕМ "БЕЛЫЙ СПИСОК") ---
	if len(filter.Filter) > 0 {
		for key, value := range filter.Filter {
			// Проверяем, разрешено ли фильтровать по этому полю
			if dbField, ok := userAllowedFilterFields[key]; ok {
				// Обрабатываем фильтрацию по нескольким значениям (например, status_id=1,2,3)
				if strVal, ok := value.(string); ok && strings.Contains(strVal, ",") {
					baseBuilder = baseBuilder.Where(sq.Eq{dbField: strings.Split(strVal, ",")})
				} else {
					baseBuilder = baseBuilder.Where(sq.Eq{dbField: value})
				}
			} else {
				// Логируем или игнорируем попытку фильтрации по неразрешенному полю
				r.logger.Warn("Попытка фильтрации пользователей по неразрешенному полю", zap.String("field", key))
			}
		}
	}

	if filter.Search != "" {
		searchPattern := "%" + strings.ToLower(filter.Search) + "%"
		searchCondition := sq.Or{
			sq.ILike{"u.fio": searchPattern},
			sq.ILike{"u.email": searchPattern},
			sq.ILike{"u.phone_number": searchPattern},
		}
		baseBuilder = baseBuilder.Where(searchCondition)
	}

	// --- Запрос на общее количество (без пагинации и сортировки) ---
	countBuilder := baseBuilder.Columns("COUNT(u.id)")
	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка сборки запроса на количество пользователей: %w", err)
	}

	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("ошибка подсчета пользователей: %w", err)
	}
	if totalCount == 0 {
		return []entities.User{}, 0, nil
	}

	// --- Основной запрос с выборкой, сортировкой и пагинацией ---
	selectBuilder := baseBuilder.Columns(userSelectFields...)

	// --- ИСПРАВЛЕННАЯ И БЕЗОПАСНАЯ СОРТИРОВКА (ИСПОЛЬЗУЕМ "БЕЛЫЙ СПИСОК") ---
	if len(filter.Sort) > 0 {
		for field, direction := range filter.Sort {
			// Проверяем, разрешена ли сортировка по этому полю
			if _, ok := userAllowedSortFields[field]; ok {
				// Приводим направление к безопасному виду
				safeDirection := "ASC"
				if strings.ToUpper(direction) == "DESC" {
					safeDirection = "DESC"
				}
				// Добавляем префикс таблицы для избежания неоднозначности
				selectBuilder = selectBuilder.OrderBy(fmt.Sprintf("u.%s %s", field, safeDirection))
			} else {
				r.logger.Warn("Попытка сортировки пользователей по неразрешенному полю", zap.String("field", field))
			}
		}
	} else {
		// Сортировка по умолчанию
		selectBuilder = selectBuilder.OrderBy("u.id DESC")
	}

	if filter.WithPagination {
		selectBuilder = selectBuilder.Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	}

	query, args, err := selectBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка сборки основного запроса пользователей: %w", err)
	}

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("ошибка выполнения запроса пользователей: %w", err)
	}
	defer rows.Close()

	users := make([]entities.User, 0, filter.Limit)
	for rows.Next() {
		user, err := scanUserEntity(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка сканирования пользователя: %w", err)
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

func (r *UserRepository) FindActiveUsersByPositionTypeAndOrg(ctx context.Context, tx pgx.Tx, posType string, departmentID *uint64, otdelID *uint64) ([]entities.User, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	// КОММЕНТАРИЙ: Удалена логика получения человекочитаемого имени.
	// Мы будем использовать `posType` ("HEAD_OF_OTDEL", "SPECIALIST" и т.д.) напрямую.

	builder := psql.Select(userSelectFields...).
		From("users u").
		Join("statuses s ON u.status_id = s.id").
		Join("positions p ON u.position_id = p.id"). // Соединяем с должностями, чтобы получить их тип
		Where(sq.Eq{
			"s.code":       "ACTIVE", // Ищем только активных пользователей
			"u.deleted_at": nil,
			"p.type":       posType, // <-- ИСПРАВЛЕНО: Теперь ищем по системному полю `p.type`
		}).
		OrderBy("u.id")

	if departmentID != nil {
		builder = builder.Where(sq.Eq{"u.department_id": *departmentID})
	}
	if otdelID != nil {
		builder = builder.Where(sq.Eq{"u.otdel_id": *otdelID})
	}

	query, args, err := builder.ToSql()
	if err != nil {
		r.logger.Error("Failed to build SQL query for FindActiveUsersByPositionTypeAndOrg", zap.Error(err))
		return nil, err
	}

	// КОММЕНТАРИЙ: В `RuleEngineService` мы используем транзакцию (tx).
	// Нужно убедиться, что мы выполняем запрос именно в ней.
	var rows pgx.Rows
	if tx != nil {
		rows, err = tx.Query(ctx, query, args...)
	} else {
		// Запасной вариант, если транзакции нет (хотя в нашем случае она всегда есть)
		rows, err = r.storage.Query(ctx, query, args...)
	}

	if err != nil {
		r.logger.Error("Error querying users by position type", zap.String("query", query), zap.Error(err))
		return nil, fmt.Errorf("ошибка запроса пользователей по типу должности: %w", err)
	}
	defer rows.Close()

	var users []entities.User
	for rows.Next() {
		user, err := scanUserEntity(rows)
		if err != nil {
			r.logger.Error("Error scanning user entity", zap.Error(err))
			return nil, err
		}
		users = append(users, *user)
	}

	return users, rows.Err()
}

func (r *UserRepository) UpdateTelegramChatID(ctx context.Context, userID uint64, chatID int64) error {
	query := `UPDATE users SET telegram_chat_id = $1, updated_at = NOW() WHERE id = $2`

	tag, err := r.storage.Exec(ctx, query, chatID, userID)
	if err != nil {

		r.logger.Error(
			"UserRepository: Ошибка при выполнении SQL-запроса на обновление telegram_chat_id",
			zap.Uint64("userID", userID),
			zap.Int64("chatID", chatID),
			zap.Error(err),
		)
		return err
	}

	if tag.RowsAffected() == 0 {
		r.logger.Warn(
			"UserRepository: Попытка обновить telegram_chat_id для несуществующего пользователя",
			zap.Uint64("userID", userID),
		)
		return apperrors.ErrNotFound
	}

	return nil
}

func (r *UserRepository) FindUserByTelegramChatID(ctx context.Context, chatID int64) (*entities.User, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select(userSelectFields...).
		From("users u").
		Join("statuses s ON u.status_id = s.id").
		Where(sq.Eq{"u.telegram_chat_id": chatID, "u.deleted_at": nil}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, err
	}
	row := r.storage.QueryRow(ctx, query, args...)
	return scanUserEntity(row)
}

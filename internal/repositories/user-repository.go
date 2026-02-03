package repositories

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
"request-system/internal/infrastructure/bd"
	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
)

const userTable = "users"

var allowedUserFilters = map[string]string{
	"id":            "u.id",
	"department_id": "u.department_id",
	"branch_id":     "u.branch_id",
	"position_id":   "u.position_id",
	"otdel_id":      "u.otdel_id",
	"office_id":     "u.office_id",
	"status_id":     "u.status_id",
	"created_at":    "u.created_at",
	"fio":           "u.fio",
}
var userMap = map[string]string{
	"id":            "u.id",
	"fio":           "u.fio",
	"email":         "u.email",
	"username":      "u.username",
	"phone_number":  "u.phone_number",
	"status_id":     "u.status_id",
	"position_id":   "u.position_id",
	

	"department_id": "u.department_id",
	"departments_id": "u.department_id", 
	"otdel_id":      "u.otdel_id",
	"branch_id":     "u.branch_id",
	"office_id":     "u.office_id",
	
	"created_at":    "u.created_at",
	"is_head":       "u.is_head",
}

type UserRepositoryInterface interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	CreateFromSync(ctx context.Context, tx pgx.Tx, user entities.User) (uint64, error)
	UpdateFromSync(ctx context.Context, tx pgx.Tx, id uint64, user entities.User) error
	FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.User, error)

	GetUsers(ctx context.Context, filter types.Filter) ([]entities.User, uint64, error)
	FindUserByID(ctx context.Context, id uint64) (*entities.User, error)
	FindUserByIDInTx(ctx context.Context, tx pgx.Tx, id uint64) (*entities.User, error)
	CreateUser(ctx context.Context, tx pgx.Tx, user *entities.User) (uint64, error)
	UpdateUserFull(ctx context.Context, user *entities.User) error
	UpdateUser(ctx context.Context, tx pgx.Tx, user *entities.User) error
	DeleteUser(ctx context.Context, id uint64) error

	FindUserByEmailOrLogin(ctx context.Context, login string) (*entities.User, error)
	FindUserByPhone(ctx context.Context, phone string) (*entities.User, error)
	UpdatePassword(ctx context.Context, userID uint64, newPasswordHash string) error
	UpdatePasswordAndClearFlag(ctx context.Context, userID uint64, newPasswordHash string) error

	SyncUserRoles(ctx context.Context, tx pgx.Tx, userID uint64, roleIDs []uint64) error
	GetRolesByUserID(ctx context.Context, userID uint64) ([]dto.ShortRoleDTO, error)
	GetRolesByUserIDs(ctx context.Context, userIDs []uint64) (map[uint64][]dto.ShortRoleDTO, error)
	FindUserIDsByRoleID(ctx context.Context, roleID uint64) ([]uint64, error)
	SyncUserDirectPermissions(ctx context.Context, tx pgx.Tx, userID uint64, permissionIDs []uint64) error
	SyncUserDeniedPermissions(ctx context.Context, tx pgx.Tx, userID uint64, permissionIDs []uint64) error

	FindUsersByIDs(ctx context.Context, userIDs []uint64) (map[uint64]entities.User, error)
	IsHeadExistsInDepartment(ctx context.Context, departmentID uint64, excludeUserID uint64) (bool, error)

	UpdateTelegramChatID(ctx context.Context, userID uint64, chatID int64) error
	FindUserByTelegramChatID(ctx context.Context, chatID int64) (*entities.User, error)
	FindActiveUsersByBranch(ctx context.Context, tx pgx.Tx, posType string, branchID uint64, officeID *uint64) ([]entities.User, error)

	FindFirstActiveUserByPositionID(ctx context.Context, tx pgx.Tx, positionID uint64) (*entities.User, error)
	FindBossByOrgAndType(ctx context.Context, tx pgx.Tx, branchID *uint64, officeID *uint64, deptID uint64, otdelID *uint64, targetType constants.PositionType) (*entities.User, error)
	UserExistsByOrgAndType(ctx context.Context, tx pgx.Tx, branchID *uint64, officeID *uint64, deptID uint64, otdelID *uint64, posTypeName string) (bool, error)
	FindPositionIDByStructureAndType(ctx context.Context, tx pgx.Tx, branchID, officeID, deptID, otdelID *uint64, posType string) (uint64, error)
	GetPositionIDsByUserID(ctx context.Context, userID uint64) ([]uint64, error)
	GetPositionIDsByUserIDs(ctx context.Context, userIDs []uint64) (map[uint64][]uint64, error)
	SyncUserOtdels(ctx context.Context, tx pgx.Tx, userID uint64, otdelIDs []uint64) error
	GetOtdelIDsByUserID(ctx context.Context, userID uint64) ([]uint64, error)
	GetOtdelIDsByUserIDs(ctx context.Context, userIDs []uint64) (map[uint64][]uint64, error)
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

func (r *UserRepository) getQuerier(tx pgx.Tx) Querier {
	if tx != nil {
		return tx
	}
	return r.storage
}

func (r *UserRepository) buildBaseSelect() sq.SelectBuilder {
	return sq.Select(
		"u.*",
		"s.code as status_code",
		"COALESCE(p.type, '') as position_type",
		"p.name as position_name",
		"b.name as branch_name",
		"d.name as department_name",
		"ot.name as otdel_name",
		"off.name as office_name",
	).
		From("users u").
		LeftJoin("statuses s ON u.status_id = s.id").
		LeftJoin("positions p ON u.position_id = p.id").
		LeftJoin("branches b ON u.branch_id = b.id").
		LeftJoin("departments d ON u.department_id = d.id").
		LeftJoin("otdels ot ON u.otdel_id = ot.id").
		LeftJoin("offices off ON u.office_id = off.id")
}

// FindPositionIDByStructureAndType возвращает ID должности, проверяя Тип в таблице positions
func (r *UserRepository) FindPositionIDByStructureAndType(ctx context.Context, tx pgx.Tx, branchID, officeID, deptID, otdelID *uint64, posType string) (uint64, error) {
	query := `
		SELECT u.position_id
		FROM users u
		JOIN positions p ON u.position_id = p.id
		WHERE u.deleted_at IS NULL
	`

	// [FIX START] >>> Изменяем логику типа должности
	// Если мы ищем руководителя (любого уровня), разрешаем также тип "MANAGER"
	if isLeadershipPosition(posType) {
		query += " AND (p.type = $1 OR p.type = 'MANAGER')"
	} else {
		query += " AND p.type = $1"
	}
	// [FIX END] <<<

	args := []interface{}{posType}
	idx := 2

	if deptID != nil {
		query += fmt.Sprintf(" AND u.department_id = $%d", idx)
		args = append(args, *deptID)
		idx++
	}
	if otdelID != nil {
		query += fmt.Sprintf(" AND u.otdel_id = $%d", idx)
		args = append(args, *otdelID)
		idx++
	}
	if branchID != nil {
		query += fmt.Sprintf(" AND u.branch_id = $%d", idx)
		args = append(args, *branchID)
		idx++
	}
	if officeID != nil {
		query += fmt.Sprintf(" AND u.office_id = $%d", idx)
		args = append(args, *officeID)
		idx++
	}

	query += ` ORDER BY CASE WHEN p.type = $1 THEN 0 ELSE 1 END LIMIT 1`

	var posID uint64
	var row pgx.Row

	if tx != nil {
		row = tx.QueryRow(ctx, query, args...)
	} else {
		row = r.storage.QueryRow(ctx, query, args...)
	}

	err := row.Scan(&posID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("ошибка SQL при поиске PositionID: %w", err)
	}

	return posID, nil
}

func (r *UserRepository) GetUsers(ctx context.Context, filter types.Filter) ([]entities.User, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	// Вспомогательная функция для текстового поиска (ILIKE)
	// Ее нельзя засунуть в bd.ApplyListParams, т.к. тут сложная логика OR
	applySearch := func(b sq.SelectBuilder) sq.SelectBuilder {
		if filter.Search != "" {
			pat := "%" + filter.Search + "%"
			return b.Where(sq.Or{
				sq.ILike{"u.fio": pat},
				sq.ILike{"u.email": pat},
				sq.ILike{"u.username": pat},
			})
		}
		return b
	}

	// --- 1. COUNT (Считаем кол-во) ---
	countBuilder := psql.Select("count(*)").From("users u").
		LeftJoin("statuses s ON u.status_id = s.id").
		LeftJoin("positions p ON u.position_id = p.id").
		Where(sq.Eq{"u.deleted_at": nil}) // Исключаем удаленных

	// Применяем поиск по тексту
	countBuilder = applySearch(countBuilder)

	// Готовим фильтр для Count (отключаем сортировку и пагинацию)
	countFilter := filter
	countFilter.WithPagination = false
	countFilter.Sort = nil

	// >>> ИСПОЛЬЗУЕМ HELPER BD <<<
	countBuilder = bd.ApplyListParams(countBuilder, countFilter, userMap)

	var totalCount uint64
	sqlCount, argsCount, _ := countBuilder.ToSql()
	
	// Выполняем запрос
	if err := r.storage.QueryRow(ctx, sqlCount, argsCount...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return []entities.User{}, 0, nil
	}

	// --- 2. SELECT (Получаем данные) ---
	// buildBaseSelect - это твой метод, который уже был в репозитории (с JOIN-ами)
	selectBuilder := r.buildBaseSelect(). 
		Where(sq.Eq{"u.deleted_at": nil}).
		PlaceholderFormat(sq.Dollar)

	// Применяем поиск по тексту
	selectBuilder = applySearch(selectBuilder)

	// Дефолтная сортировка, если фронт не прислал
	if len(filter.Sort) == 0 {
		selectBuilder = selectBuilder.OrderBy("u.created_at DESC")
	}

	// >>> ИСПОЛЬЗУЕМ HELPER BD (Сортировка, фильтры и пагинация) <<<
	selectBuilder = bd.ApplyListParams(selectBuilder, filter, userMap)

	sqlSelect, argsSelect, err := selectBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.storage.Query(ctx, sqlSelect, argsSelect...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Используем CollectRows (у тебя он был раньше), он сам смапит поля
	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.User])
	return users, totalCount, err
}

func (r *UserRepository) CreateUser(ctx context.Context, tx pgx.Tx, u *entities.User) (uint64, error) {
	// --- (1) Обработка Основных Полей (Legacy Sync) ---
	// Если прислали список должностей, ставим первую главной
	if len(u.PositionIDs) > 0 && (u.PositionID == nil || *u.PositionID == 0) {
		first := u.PositionIDs[0]
		u.PositionID = &first
	}
	// Если прислали список отделов, ставим первый главным (для отображения)
	if len(u.OtdelIDs) > 0 && (u.OtdelID == nil || *u.OtdelID == 0) {
		first := u.OtdelIDs[0]
		u.OtdelID = &first
	}

	// --- (2) Вставка самого Юзера ---
	q := `INSERT INTO users (fio, email, phone_number, password, position_id, status_id, branch_id, 
		department_id, office_id, otdel_id, photo_url, must_change_password, is_head, username, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW()) RETURNING id`

	var id uint64
	err := tx.QueryRow(ctx, q,
		u.Fio, u.Email, u.PhoneNumber, u.Password, u.PositionID,
		u.StatusID, u.BranchID, u.DepartmentID, u.OfficeID,
		u.OtdelID, 
		u.PhotoURL, u.MustChangePassword, u.IsHead, u.Username,
	).Scan(&id)
	
	if err != nil {
		return 0, r.handlePgError(err)
	}
	
	if len(u.PositionIDs) > 0 {
		if err := r.SyncUserPositions(ctx, tx, id, u.PositionIDs); err != nil {
			return 0, err
		}
	} else if u.PositionID != nil {
	
		r.SyncUserPositions(ctx, tx, id, []uint64{*u.PositionID})
	}

	if len(u.OtdelIDs) > 0 {
		if err := r.SyncUserOtdels(ctx, tx, id, u.OtdelIDs); err != nil {
			return 0, err
		}
	} else if u.OtdelID != nil {
		r.SyncUserOtdels(ctx, tx, id, []uint64{*u.OtdelID})
	}

	return id, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, tx pgx.Tx, u *entities.User) error {
	b := sq.Update(userTable).
		PlaceholderFormat(sq.Dollar).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": u.ID, "deleted_at": nil}).
		// Основные поля
		Set("fio", u.Fio).
		Set("email", u.Email).
		Set("phone_number", u.PhoneNumber).
		Set("position_id", u.PositionID).
		Set("status_id", u.StatusID).
		Set("branch_id", u.BranchID).
		Set("department_id", u.DepartmentID).
		Set("office_id", u.OfficeID).
		Set("otdel_id", u.OtdelID). // Обновляем основной
		Set("photo_url", u.PhotoURL).
		Set("is_head", u.IsHead).
		Set("username", u.Username)

	if u.Password != "" {
		b = b.Set("password", u.Password)
	}

	sqlStr, args, err := b.ToSql()
	if err != nil { return err }

	if _, err := tx.Exec(ctx, sqlStr, args...); err != nil {
		return r.handlePgError(err)
	}

	// --- Сохранение списков ---

	// Должности
	if u.PositionIDs != nil { 
		if err := r.SyncUserPositions(ctx, tx, u.ID, u.PositionIDs); err != nil {
			return err
		}
	}

	if u.OtdelIDs != nil { 
		if err := r.SyncUserOtdels(ctx, tx, u.ID, u.OtdelIDs); err != nil {
			return err
		}
	}

	return nil
}
func (r *UserRepository) handlePgError(err error) error {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		switch pgErr.Code {
		case "23505": // unique_violation
			if strings.Contains(pgErr.ConstraintName, "email") {
				return apperrors.NewHttpError(http.StatusConflict, "Email уже используется", err, nil)
			}
			if strings.Contains(pgErr.ConstraintName, "phone") {
				return apperrors.NewHttpError(http.StatusConflict, "Номер телефона уже используется", err, nil)
			}
			if strings.Contains(pgErr.ConstraintName, "username") {
				return apperrors.NewHttpError(http.StatusConflict, "Этот логин AD уже привязан к другому пользователю", err, nil)
			}
		}
	}
	return err
}

func (r *UserRepository) FindUserByID(ctx context.Context, id uint64) (*entities.User, error) {
	return r.findOneUser(ctx, r.storage, sq.Eq{"u.id": id, "u.deleted_at": nil})
}

func (r *UserRepository) FindUserByIDInTx(ctx context.Context, tx pgx.Tx, id uint64) (*entities.User, error) {
	return r.findOneUser(ctx, tx, sq.Eq{"u.id": id, "u.deleted_at": nil})
}

func (r *UserRepository) DeleteUser(ctx context.Context, id uint64) error {
	_, err := r.storage.Exec(ctx, "UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL", id)
	return err
}

// --- Sync (для 1C и прочего, если используется) ---

func (r *UserRepository) CreateFromSync(ctx context.Context, tx pgx.Tx, u entities.User) (uint64, error) {
	q := `INSERT INTO users (fio, email, phone_number, password, status_id, position_id, department_id, 
		  otdel_id, branch_id, office_id, external_id, source_system, username, created_at, updated_at, must_change_password, is_head)
		  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW(), false, false) RETURNING id`

	var id uint64
	// В аргументах добавился u.Username 13-м параметром
	err := tx.QueryRow(ctx, q, u.Fio, u.Email, u.PhoneNumber, u.Password, u.StatusID, u.PositionID,
		u.DepartmentID, u.OtdelID, u.BranchID, u.OfficeID, u.ExternalID, u.SourceSystem, u.Username).Scan(&id)
	return id, err
}

// 2. Изменяем UpdateFromSync (Добавили username в SQL и в аргументы)
func (r *UserRepository) UpdateFromSync(ctx context.Context, tx pgx.Tx, id uint64, u entities.User) error {
	q := `UPDATE users SET fio=$1, email=$2, phone_number=$3, status_id=$4, position_id=$5,
		department_id=$6, otdel_id=$7, branch_id=$8, office_id=$9, username=$10, updated_at=NOW() WHERE id=$11`

	_, err := tx.Exec(ctx, q, u.Fio, u.Email, u.PhoneNumber, u.StatusID, u.PositionID,
		u.DepartmentID, u.OtdelID, u.BranchID, u.OfficeID, u.Username, id)
	return err
}

// 3. Изменяем FindUserByEmailOrLogin (Улучшаем поиск для логина)
func (r *UserRepository) FindUserByEmailOrLogin(ctx context.Context, login string) (*entities.User, error) {
	// Строим сложный запрос: (username = login ИЛИ email = login) И удален = NULL
	whereClause := sq.And{
		sq.Or{
			sq.Eq{"LOWER(u.email)": strings.ToLower(login)},
			// Убедись, что колонка в БД точно называется "username" (как в миграции)
			sq.Eq{"LOWER(u.username)": strings.ToLower(login)},
		},
		sq.Eq{"u.deleted_at": nil},
	}

	// Теперь передаем whereClause (который sq.And) в функцию, ожидающую interface{}
	return r.findOneUser(ctx, r.storage, whereClause)
}

func (r *UserRepository) FindByExternalID(ctx context.Context, tx pgx.Tx, externalID, source string) (*entities.User, error) {
	return r.findOneUser(ctx, r.getQuerier(tx), sq.Eq{"u.external_id": externalID, "u.source_system": source})
}

// --- Specific Finders ---
func (r *UserRepository) FindUserByPhone(ctx context.Context, phone string) (*entities.User, error) {
	return r.findOneUser(ctx, r.storage, sq.Eq{"u.phone_number": phone, "u.deleted_at": nil})
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uint64, hash string) error {
	_, err := r.storage.Exec(ctx, "UPDATE users SET password=$1, updated_at=NOW() WHERE id=$2", hash, userID)
	return err
}

func (r *UserRepository) UpdatePasswordAndClearFlag(ctx context.Context, userID uint64, hash string) error {
	_, err := r.storage.Exec(ctx, "UPDATE users SET password=$1, must_change_password=FALSE, updated_at=NOW() WHERE id=$2", hash, userID)
	return err
}

// UpdateUserFull (Legacy)
func (r *UserRepository) UpdateUserFull(ctx context.Context, u *entities.User) error {
	_, err := r.storage.Exec(ctx, `UPDATE users SET fio=$1, email=$2, department_id=$3, position_id=$4, updated_at=NOW() WHERE id=$5`,
		u.Fio, u.Email, u.DepartmentID, u.PositionID, u.ID)
	return r.handlePgError(err)
}

func (r *UserRepository) syncUserLinks(ctx context.Context, tx pgx.Tx, userID uint64, linkIDs []uint64, tableName, linkColumnName string) error {
	if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE user_id=$1", tableName), userID); err != nil {
		return err
	}
	if len(linkIDs) == 0 {
		return nil
	}
	rows := make([][]interface{}, len(linkIDs))
	for i, id := range linkIDs {
		rows[i] = []interface{}{userID, id}
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{tableName}, []string{"user_id", linkColumnName}, pgx.CopyFromRows(rows))
	return err
}

func (r *UserRepository) SyncUserRoles(ctx context.Context, tx pgx.Tx, userID uint64, roleIDs []uint64) error {
	return r.syncUserLinks(ctx, tx, userID, roleIDs, "user_roles", "role_id")
}

func (r *UserRepository) SyncUserDirectPermissions(ctx context.Context, tx pgx.Tx, userID uint64, pIDs []uint64) error {
	return r.syncUserLinks(ctx, tx, userID, pIDs, "user_permissions", "permission_id")
}

func (r *UserRepository) SyncUserDeniedPermissions(ctx context.Context, tx pgx.Tx, userID uint64, pIDs []uint64) error {
	return r.syncUserLinks(ctx, tx, userID, pIDs, "user_permission_denials", "permission_id")
}

// Helpers Read
func (r *UserRepository) FindUserIDsByRoleID(ctx context.Context, roleID uint64) ([]uint64, error) {
	rows, err := r.storage.Query(ctx, "SELECT user_id FROM user_roles WHERE role_id=$1", roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *UserRepository) GetRolesByUserIDs(ctx context.Context, userIDs []uint64) (map[uint64][]dto.ShortRoleDTO, error) {
	if len(userIDs) == 0 {
		return map[uint64][]dto.ShortRoleDTO{}, nil
	}
	q := `SELECT ur.user_id, r.id, r.name FROM roles r JOIN user_roles ur ON r.id = ur.role_id WHERE ur.user_id = ANY($1)`
	rows, err := r.storage.Query(ctx, q, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[uint64][]dto.ShortRoleDTO)
	for rows.Next() {
		var uid uint64
		var d dto.ShortRoleDTO
		if err := rows.Scan(&uid, &d.ID, &d.Name); err == nil {
			m[uid] = append(m[uid], d)
		} // dto uses Fio as Role Name map
	}
	return m, nil
}

func (r *UserRepository) GetRolesByUserID(ctx context.Context, userID uint64) ([]dto.ShortRoleDTO, error) {
	q := `SELECT r.id, r.name FROM roles r JOIN user_roles ur ON r.id = ur.role_id WHERE ur.user_id = $1`
	rows, err := r.storage.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []dto.ShortRoleDTO
	for rows.Next() {
		var d dto.ShortRoleDTO
		rows.Scan(&d.ID, &d.Name)
		list = append(list, d)
	}
	return list, nil
}

func (r *UserRepository) FindUsersByIDs(ctx context.Context, userIDs []uint64) (map[uint64]entities.User, error) {
	if len(userIDs) == 0 {
		return map[uint64]entities.User{}, nil
	}
	q := r.buildBaseSelect().
        Where(sq.Eq{"u.id": userIDs, "u.deleted_at": nil}).
        PlaceholderFormat(sq.Dollar)

	sqlStr, args, err := q.ToSql()
    if err != nil {
        return nil, err
    }

	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.User])
    if err != nil {
        return nil, err
    }

	m := make(map[uint64]entities.User)
	for _, u := range users {
		m[u.ID] = u
	}
	return m, nil
}

func (r *UserRepository) IsHeadExistsInDepartment(ctx context.Context, dID, excludeUID uint64) (bool, error) {
	var exists bool
	err := r.storage.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE department_id=$1 AND is_head=true AND id!=$2 AND deleted_at IS NULL)", dID, excludeUID).Scan(&exists)
	return exists, err
}

func (r *UserRepository) UpdateTelegramChatID(ctx context.Context, userID uint64, chatID int64) error {
	tag, err := r.storage.Exec(ctx, "UPDATE users SET telegram_chat_id=$1, updated_at=NOW() WHERE id=$2", chatID, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return err
}

func (r *UserRepository) FindUserByTelegramChatID(ctx context.Context, chatID int64) (*entities.User, error) {
	return r.findOneUser(ctx, r.storage, sq.Eq{"u.telegram_chat_id": chatID, "u.deleted_at": nil})
}

func (r *UserRepository) FindActiveUsersByBranch(ctx context.Context, tx pgx.Tx, posType string, bID uint64, offID *uint64) ([]entities.User, error) {
	q := sq.Select("u.*", "s.code as status_code").From("users u").
		Join("statuses s ON u.status_id = s.id").
		Join("positions p ON u.position_id = p.id").
		Where("UPPER(s.code) = 'ACTIVE'").
		Where(sq.Eq{"u.deleted_at": nil, "p.type": posType, "u.branch_id": bID}).
		PlaceholderFormat(sq.Dollar)
	if offID != nil {
		q = q.Where(sq.Eq{"u.office_id": *offID})
	}

	sqlStr, args, _ := q.ToSql()
	rows, err := tx.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[entities.User])
}

func (r *UserRepository) FindFirstActiveUserByPositionID(ctx context.Context, tx pgx.Tx, positionID uint64) (*entities.User, error) {
	query := `
		SELECT u.id, u.fio, u.email, u.phone_number, u.department_id, u.otdel_id, u.branch_id, u.office_id, u.position_id
		FROM users u
		JOIN statuses s ON u.status_id = s.id
		WHERE u.position_id = $1 AND s.code = 'ACTIVE' AND u.deleted_at IS NULL
		LIMIT 1
	`

	var row pgx.Row
	if tx != nil {
		row = tx.QueryRow(ctx, query, positionID)
	} else {
		row = r.storage.QueryRow(ctx, query, positionID)
	}

	var u entities.User
	err := row.Scan(
		&u.ID, &u.Fio, &u.Email, &u.PhoneNumber,
		&u.DepartmentID, &u.OtdelID, &u.BranchID, &u.OfficeID, &u.PositionID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// FindBossByOrgAndType: ГЛАВНЫЙ МЕТОД ПОИСКА. Ищет активного сотрудника по Типу должности в конкретной ветке.
func (r *UserRepository) FindBossByOrgAndType(ctx context.Context, tx pgx.Tx,
	branchID *uint64, officeID *uint64,
	deptID uint64, otdelID *uint64,
	targetType constants.PositionType,
) (*entities.User, error) {

	query := `
		SELECT DISTINCT u.id, u.fio, u.email, u.phone_number, u.department_id, u.otdel_id, u.branch_id, u.office_id, u.position_id
		FROM users u
		JOIN user_positions up ON u.id = up.user_id 
		JOIN positions p ON up.position_id = p.id
		JOIN statuses s ON u.status_id = s.id
		WHERE s.code = 'ACTIVE'
		  AND u.deleted_at IS NULL
	`

	// [FIX START] >>> Добавляем условие OR
	posTypeStr := string(targetType)
	// Если мы ищем босса, то Менеджер тоже подходит (согласно логике 1С)
	query += " AND (p.type = $1 OR p.type = 'MANAGER')"
	// [FIX END] <<<

	args := []interface{}{posTypeStr}
	argIdx := 2

	if deptID > 0 {
		query += ` AND u.department_id = $` + strconv.Itoa(argIdx)
		args = append(args, deptID)
		argIdx++
		// Если ищем конкретный отдел, добавляем фильтр
		if otdelID != nil {
			query += ` AND u.otdel_id = $` + strconv.Itoa(argIdx)
			args = append(args, *otdelID)
			argIdx++
		}
	}

	if branchID != nil {
		query += ` AND u.branch_id = $` + strconv.Itoa(argIdx)
		args = append(args, *branchID)
		argIdx++
		if officeID != nil {
			query += ` AND u.office_id = $` + strconv.Itoa(argIdx)
			args = append(args, *officeID)
			argIdx++
		}
	}

	query += ` ORDER BY CASE WHEN p.type = $1 THEN 0 ELSE 1 END, u.created_at ASC LIMIT 1`

	var row pgx.Row
	if tx != nil {
		row = tx.QueryRow(ctx, query, args...)
	} else {
		row = r.storage.QueryRow(ctx, query, args...)
	}

	var u entities.User
	err := row.Scan(
		&u.ID, &u.Fio, &u.Email, &u.PhoneNumber,
		&u.DepartmentID, &u.OtdelID, &u.BranchID, &u.OfficeID, &u.PositionID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Логируем или возвращаем ошибку более понятно
			return nil, nil // Не найден
		}
		return nil, err
	}

	return &u, nil
}

// UserExistsByOrgAndType проверяет существование пользователя (валидация при создании правил)
func (r *UserRepository) UserExistsByOrgAndType(ctx context.Context, tx pgx.Tx,
	branchID *uint64, officeID *uint64,
	deptID uint64, otdelID *uint64,
	posTypeName string,
) (bool, error) {
	query := `
        SELECT EXISTS(
            SELECT 1
            FROM users u
            JOIN statuses s ON u.status_id = s.id
            JOIN positions p ON u.position_id = p.id
            WHERE s.code = 'ACTIVE'
              AND u.deleted_at IS NULL
    `
	
	// [FIX START] >>>
	if isLeadershipPosition(posTypeName) {
		query += " AND (p.type = $1 OR p.type = 'MANAGER')"
	} else {
		query += " AND p.type = $1"
	}
	// [FIX END] <<<

	args := []interface{}{posTypeName}
	argIdx := 2

	if deptID > 0 {
		query += ` AND u.department_id = $` + strconv.Itoa(argIdx)
		args = append(args, deptID)
		argIdx++
		if otdelID != nil {
			query += ` AND u.otdel_id = $` + strconv.Itoa(argIdx)
			args = append(args, *otdelID)
			argIdx++
		}
	}

	if branchID != nil {
		query += ` AND u.branch_id = $` + strconv.Itoa(argIdx)
		args = append(args, *branchID)
		argIdx++
		if officeID != nil {
			query += ` AND u.office_id = $` + strconv.Itoa(argIdx)
			args = append(args, *officeID)
			argIdx++
		}
	}

	query += `)`

	var row pgx.Row
	if tx != nil {
		row = tx.QueryRow(ctx, query, args...)
	} else {
		row = r.storage.QueryRow(ctx, query, args...)
	}

	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

func (r *UserRepository) findOneUser(ctx context.Context, querier Querier, where interface{}) (*entities.User, error) {
	q := r.buildBaseSelect().Where(where).PlaceholderFormat(sq.Dollar)

	sqlStr, args, err := q.ToSql()
	if err != nil {
		return nil, fmt.Errorf("ошибка сборки SQL для findOneUser: %w", err)
	}

	rows, err := querier.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	user, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.User])
	return &user, err
}
func (r *UserRepository) SyncUserPositions(ctx context.Context, tx pgx.Tx, userID uint64, posIDs []uint64) error {

	if _, err := tx.Exec(ctx, "DELETE FROM user_positions WHERE user_id=$1", userID); err != nil {
		return err
	}
	if len(posIDs) == 0 {
		return nil
	}
	uniqueIDs := make(map[uint64]bool)
	var cleanIDs []uint64
	for _, id := range posIDs {
		if _, exists := uniqueIDs[id]; !exists {
			uniqueIDs[id] = true
			cleanIDs = append(cleanIDs, id)
		}
	}
	rows := make([][]interface{}, len(cleanIDs))
	for i, pid := range cleanIDs {
		rows[i] = []interface{}{userID, pid}
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"user_positions"}, []string{"user_id", "position_id"}, pgx.CopyFromRows(rows))
	return err
}
func (r *UserRepository) GetPositionIDsByUserID(ctx context.Context, userID uint64) ([]uint64, error) {
	rows, err := r.storage.Query(ctx, "SELECT position_id FROM user_positions WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uint64
	for rows.Next() {
		var pid uint64
		if err := rows.Scan(&pid); err == nil {
			ids = append(ids, pid)
		}
	}
	return ids, nil
}
func (r *UserRepository) GetPositionIDsByUserIDs(ctx context.Context, userIDs []uint64) (map[uint64][]uint64, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	query := `SELECT user_id, position_id FROM user_positions WHERE user_id = ANY($1)`
	
	rows, err := r.storage.Query(ctx, query, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uint64][]uint64)
	for rows.Next() {
		var uid, pid uint64
		if err := rows.Scan(&uid, &pid); err == nil {
			result[uid] = append(result[uid], pid)
		}
	}
	return result, nil
}
func (r *UserRepository) SyncUserOtdels(ctx context.Context, tx pgx.Tx, userID uint64, otdelIDs []uint64) error {
	// 1. Очистка старых (для надежности при update)
	if _, err := tx.Exec(ctx, "DELETE FROM user_otdels WHERE user_id=$1", userID); err != nil { return err }
	if len(otdelIDs) == 0 { return nil }

	// 2. Дедупликация (убираем повторы)
	uniq := make(map[uint64]bool); list := []uint64{}
	for _, id := range otdelIDs {
		if !uniq[id] { uniq[id] = true; list = append(list, id) }
	}

	// 3. Запись
	rows := make([][]interface{}, len(list))
	for i, id := range list { rows[i] = []interface{}{userID, id} }
	
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"user_otdels"}, []string{"user_id", "otdel_id"}, pgx.CopyFromRows(rows))
	return err
}
func (r *UserRepository) GetOtdelIDsByUserID(ctx context.Context, userID uint64) ([]uint64, error) {
	rows, err := r.storage.Query(ctx, "SELECT otdel_id FROM user_otdels WHERE user_id=$1", userID)
	if err != nil { return nil, err }
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err == nil { ids = append(ids, id) }
	}
	return ids, nil
}
func (r *UserRepository) GetOtdelIDsByUserIDs(ctx context.Context, userIDs []uint64) (map[uint64][]uint64, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	query := `SELECT user_id, otdel_id FROM user_otdels WHERE user_id = ANY($1)`
	rows, err := r.storage.Query(ctx, query, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uint64][]uint64)
	for rows.Next() {
		var uid, oid uint64
		if err := rows.Scan(&uid, &oid); err == nil {
			result[uid] = append(result[uid], oid)
		}
	}
	return result, nil
}
func isLeadershipPosition(posType string) bool {
	switch constants.PositionType(posType) {
	case constants.PositionTypeHeadOfOtdel,
		 constants.PositionTypeHeadOfDepartment,
		 constants.PositionTypeDeputyHeadOfDepartment, 
		 constants.PositionTypeBranchDirector,
		 constants.PositionTypeHeadOfOffice:
		return true
	default:
		return false
	}
}

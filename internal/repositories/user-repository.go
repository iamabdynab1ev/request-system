package repositories

import (
	"context"
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

type UserRepositoryInterface interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
	CreateFromSync(ctx context.Context, tx pgx.Tx, user entities.User) (uint64, error)
	UpdateFromSync(ctx context.Context, tx pgx.Tx, id uint64, user entities.User) error
	FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.User, error)

	GetUsers(ctx context.Context, filter types.Filter) ([]entities.User, uint64, error)
	FindUserByID(ctx context.Context, id uint64) (*entities.User, error)
	FindUserByIDInTx(ctx context.Context, tx pgx.Tx, id uint64) (*entities.User, error)
	CreateUser(ctx context.Context, tx pgx.Tx, user *entities.User) (uint64, error)
	UpdateUserFull(ctx context.Context, user *entities.User) error // Legacy
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

	FindActiveUsersByPositionTypeAndOrg(ctx context.Context, tx pgx.Tx, posType string, depID *uint64, otdelID *uint64) ([]entities.User, error)
	FindUsersByIDs(ctx context.Context, userIDs []uint64) (map[uint64]entities.User, error)
	IsHeadExistsInDepartment(ctx context.Context, departmentID uint64, excludeUserID uint64) (bool, error)

	UpdateTelegramChatID(ctx context.Context, userID uint64, chatID int64) error
	FindUserByTelegramChatID(ctx context.Context, chatID int64) (*entities.User, error)
	FindActiveUsersByBranch(ctx context.Context, tx pgx.Tx, posType string, branchID uint64, officeID *uint64) ([]entities.User, error)
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

func (r *UserRepository) buildBaseSelect() sq.SelectBuilder {
	return sq.Select("u.*", "s.code as status_code", "COALESCE(p.type, '') as position_type").
		From("users u").
		LeftJoin("statuses s ON u.status_id = s.id").
		LeftJoin("positions p ON u.position_id = p.id")
}

func (r *UserRepository) GetUsers(ctx context.Context, filter types.Filter) ([]entities.User, uint64, error) {
	// --- ЗАПРОС №1: COUNT (только WHERE) ---
	countBuilder := sq.Select("count(*)").From("users u").
		LeftJoin("statuses s ON u.status_id = s.id").
		LeftJoin("positions p ON u.position_id = p.id").
		Where(sq.Eq{"u.deleted_at": nil}).
		PlaceholderFormat(sq.Dollar)

	if filter.Search != "" {
		pat := "%" + filter.Search + "%"
		countBuilder = countBuilder.Where(sq.Or{
			sq.ILike{"u.fio": pat}, sq.ILike{"u.email": pat},
		})
	}

	for k, v := range filter.Filter {
		if col, ok := allowedUserFilters[k]; ok {
			countBuilder = countBuilder.Where(sq.Eq{col: v})
		}
	}

	countSql, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}

	var totalCount uint64
	if err := r.storage.QueryRow(ctx, countSql, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return []entities.User{}, 0, nil
	}

	// --- ЗАПРОС №2: SELECT (WHERE + Sort + Pagination) ---
	selectBuilder := r.buildBaseSelect().
		Where(sq.Eq{"u.deleted_at": nil}).
		PlaceholderFormat(sq.Dollar)

	if filter.Search != "" {
		pat := "%" + filter.Search + "%"
		selectBuilder = selectBuilder.Where(sq.Or{
			sq.ILike{"u.fio": pat}, sq.ILike{"u.email": pat},
		})
	}
	for k, v := range filter.Filter {
		if col, ok := allowedUserFilters[k]; ok {
			selectBuilder = selectBuilder.Where(sq.Eq{col: v})
		}
	}

	if len(filter.Sort) > 0 {
		for field, dir := range filter.Sort {
			if dbCol, ok := allowedUserFilters[field]; ok {
				d := "ASC"
				if strings.ToLower(dir) == "desc" {
					d = "DESC"
				}
				selectBuilder = selectBuilder.OrderBy(fmt.Sprintf("%s %s", dbCol, d))
			}
		}
	} else {
		selectBuilder = selectBuilder.OrderBy("u.created_at DESC")
	}

	if filter.WithPagination {
		selectBuilder = selectBuilder.Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	}

	selectSql, selectArgs, err := selectBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.storage.Query(ctx, selectSql, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.User])
	return users, totalCount, err
}

func (r *UserRepository) CreateUser(ctx context.Context, tx pgx.Tx, u *entities.User) (uint64, error) {
	q := `INSERT INTO users (fio, email, phone_number, password, position_id, status_id, branch_id, 
		department_id, office_id, otdel_id, photo_url, must_change_password, is_head, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW()) RETURNING id`

	var id uint64
	err := tx.QueryRow(ctx, q,
		u.Fio, u.Email, u.PhoneNumber, u.Password, u.PositionID,
		u.StatusID, u.BranchID, u.DepartmentID, u.OfficeID,
		u.OtdelID, u.PhotoURL, u.MustChangePassword, u.IsHead,
	).Scan(&id)
	if err != nil {
		return 0, r.handlePgError(err)
	}
	return id, nil
}

// UpdateUser теперь принимает ПОЛНУЮ сущность, а логику ApplyUpdates делает сервис.
func (r *UserRepository) UpdateUser(ctx context.Context, tx pgx.Tx, u *entities.User) error {
	b := sq.Update(userTable).
		PlaceholderFormat(sq.Dollar).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": u.ID, "deleted_at": nil}).
		// Fields
		Set("fio", u.Fio).
		Set("email", u.Email).
		Set("phone_number", u.PhoneNumber).
		Set("position_id", u.PositionID).
		Set("status_id", u.StatusID).
		Set("branch_id", u.BranchID).
		Set("department_id", u.DepartmentID).
		Set("office_id", u.OfficeID).
		Set("otdel_id", u.OtdelID).
		Set("photo_url", u.PhotoURL).
		Set("is_head", u.IsHead)

	// Password обновляем только если он задан (не пустая строка)
	if u.Password != "" {
		b = b.Set("password", u.Password)
	}

	sqlStr, args, err := b.ToSql()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, sqlStr, args...); err != nil {
		return r.handlePgError(err)
	}
	return nil
}

func (r *UserRepository) handlePgError(err error) error {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		switch pgErr.Code {
		case "23505":
			if strings.Contains(pgErr.ConstraintName, "email") {
				return apperrors.NewHttpError(http.StatusConflict, "Email уже используется", err, nil)
			}
			if strings.Contains(pgErr.ConstraintName, "phone") {
				return apperrors.NewHttpError(http.StatusConflict, "Номер телефона уже используется", err, nil)
			}
		}
	}
	return err
}

func (r *UserRepository) FindUserByID(ctx context.Context, id uint64) (*entities.User, error) {
	q := r.buildBaseSelect().Where(sq.Eq{"u.id": id, "u.deleted_at": nil}).PlaceholderFormat(sq.Dollar)
	sqlStr, args, _ := q.ToSql()
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.User])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	return &u, err
}

func (r *UserRepository) FindUserByIDInTx(ctx context.Context, tx pgx.Tx, id uint64) (*entities.User, error) {
	q := r.buildBaseSelect().Where(sq.Eq{"u.id": id, "u.deleted_at": nil}).PlaceholderFormat(sq.Dollar)
	sqlStr, args, _ := q.ToSql()
	rows, err := tx.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.User])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	return &u, err
}

func (r *UserRepository) DeleteUser(ctx context.Context, id uint64) error {
	_, err := r.storage.Exec(ctx, "UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL", id)
	return err
}

// --- Sync (для 1C и прочего, если используется) ---
func (r *UserRepository) CreateFromSync(ctx context.Context, tx pgx.Tx, u entities.User) (uint64, error) {
	q := `INSERT INTO users (fio, email, phone_number, password, status_id, position_id, department_id, 
		  otdel_id, branch_id, office_id, external_id, source_system, created_at, updated_at, must_change_password, is_head)
		  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW(), false, false) RETURNING id`
	var id uint64
	err := tx.QueryRow(ctx, q, u.Fio, u.Email, u.PhoneNumber, u.Password, u.StatusID, u.PositionID,
		u.DepartmentID, u.OtdelID, u.BranchID, u.OfficeID, u.ExternalID, u.SourceSystem).Scan(&id)
	return id, err
}

func (r *UserRepository) UpdateFromSync(ctx context.Context, tx pgx.Tx, id uint64, u entities.User) error {
	q := `UPDATE users SET fio=$1, email=$2, phone_number=$3, status_id=$4, position_id=$5,
		department_id=$6, otdel_id=$7, branch_id=$8, office_id=$9, updated_at=NOW() WHERE id=$10`
	_, err := tx.Exec(ctx, q, u.Fio, u.Email, u.PhoneNumber, u.StatusID, u.PositionID,
		u.DepartmentID, u.OtdelID, u.BranchID, u.OfficeID, id)
	return err
}

func (r *UserRepository) FindByExternalID(ctx context.Context, tx pgx.Tx, externalID, source string) (*entities.User, error) {
	q := r.buildBaseSelect().Where(sq.Eq{"u.external_id": externalID, "u.source_system": source}).PlaceholderFormat(sq.Dollar)

	var qer Querier = r.storage
	if tx != nil {
		qer = tx
	}

	sqlStr, args, _ := q.ToSql()
	rows, err := qer.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.User])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

// --- Specific Finders ---
func (r *UserRepository) FindUserByEmailOrLogin(ctx context.Context, login string) (*entities.User, error) {
	q := r.buildBaseSelect().Where(sq.Eq{"LOWER(u.email)": strings.ToLower(login), "u.deleted_at": nil}).PlaceholderFormat(sq.Dollar)

	sqlStr, args, _ := q.ToSql()
	r.logger.Warn("DEBUGGING SQL", zap.String("sql", sqlStr), zap.Any("args", args))

	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		r.logger.Error("DEBUGGING SQL: Query Error", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	// Пытаемся прочитать
	user, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.User])

	// Логируем результат
	if errors.Is(err, pgx.ErrNoRows) {
		r.logger.Warn("DEBUGGING SQL: pgx.ErrNoRows - SQL did not find a user.")
		return nil, apperrors.ErrNotFound
	} else if err != nil {
		r.logger.Error("DEBUGGING SQL: CollectOneRow FAILED (likely a mapping error)", zap.Error(err))
		return nil, err // Важно вернуть ошибку, если она не 'no rows'
	}

	r.logger.Warn("DEBUGGING SQL: User found and mapped successfully.", zap.Any("user", user))

	return &user, nil
}

func (r *UserRepository) FindUserByPhone(ctx context.Context, phone string) (*entities.User, error) {
	q := sq.Select("u.*", "s.code as status_code").From("users u").
		Join("statuses s ON u.status_id = s.id").
		LeftJoin("positions p ON u.position_id = p.id").
		Where(sq.Eq{"u.phone_number": phone, "u.deleted_at": nil}).PlaceholderFormat(sq.Dollar)
	sqlStr, args, _ := q.ToSql()
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.User])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	return &u, err
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
	// (Старый метод для full update) - редиректим на основной update или оставляем если используется только в специфике
	// Реализуем по новой
	_, err := r.storage.Exec(ctx, `UPDATE users SET fio=$1, email=$2, department_id=$3, position_id=$4, updated_at=NOW() WHERE id=$5`,
		u.Fio, u.Email, u.DepartmentID, u.PositionID, u.ID)
	return r.handlePgError(err)
}

// --- Roles & Permissions (Bulk helpers) ---
func (r *UserRepository) SyncUserRoles(ctx context.Context, tx pgx.Tx, userID uint64, roleIDs []uint64) error {
	if _, err := tx.Exec(ctx, "DELETE FROM user_roles WHERE user_id=$1", userID); err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}
	rows := [][]interface{}{}
	for _, rid := range roleIDs {
		rows = append(rows, []interface{}{userID, rid})
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"user_roles"}, []string{"user_id", "role_id"}, pgx.CopyFromRows(rows))
	return err
}

func (r *UserRepository) SyncUserDirectPermissions(ctx context.Context, tx pgx.Tx, userID uint64, pIDs []uint64) error {
	if _, err := tx.Exec(ctx, "DELETE FROM user_permissions WHERE user_id=$1", userID); err != nil {
		return err
	}
	if len(pIDs) == 0 {
		return nil
	}
	rows := [][]interface{}{}
	for _, pid := range pIDs {
		rows = append(rows, []interface{}{userID, pid})
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"user_permissions"}, []string{"user_id", "permission_id"}, pgx.CopyFromRows(rows))
	return err
}

func (r *UserRepository) SyncUserDeniedPermissions(ctx context.Context, tx pgx.Tx, userID uint64, pIDs []uint64) error {
	if _, err := tx.Exec(ctx, "DELETE FROM user_permission_denials WHERE user_id=$1", userID); err != nil {
		return err
	}
	if len(pIDs) == 0 {
		return nil
	}
	rows := [][]interface{}{}
	for _, pid := range pIDs {
		rows = append(rows, []interface{}{userID, pid})
	}
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"user_permission_denials"}, []string{"user_id", "permission_id"}, pgx.CopyFromRows(rows))
	return err
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
	q := sq.Select("u.*", "s.code as status_code").From("users u").
		Join("statuses s ON u.status_id = s.id").Where(sq.Eq{"u.id": userIDs, "u.deleted_at": nil}).PlaceholderFormat(sq.Dollar)
	sqlStr, args, _ := q.ToSql()
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[uint64]entities.User)
	users, _ := pgx.CollectRows(rows, pgx.RowToStructByName[entities.User])
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

func (r *UserRepository) FindActiveUsersByPositionTypeAndOrg(ctx context.Context, tx pgx.Tx, posType string, depID, otdelID *uint64) ([]entities.User, error) {
	q := sq.Select("u.*", "s.code as status_code").From("users u").
		Join("statuses s ON u.status_id = s.id").
		Join("positions p ON u.position_id = p.id").
		Where("UPPER(s.code) = 'ACTIVE'").
		Where(sq.Eq{"u.deleted_at": nil, "p.type": posType}).
		PlaceholderFormat(sq.Dollar)
	if depID != nil {
		q = q.Where(sq.Eq{"u.department_id": *depID})
	}
	if otdelID != nil {
		q = q.Where(sq.Eq{"u.otdel_id": *otdelID})
	}

	sqlStr, args, _ := q.ToSql()
	rows, err := tx.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[entities.User])
}

// Telegram
func (r *UserRepository) UpdateTelegramChatID(ctx context.Context, userID uint64, chatID int64) error {
	tag, err := r.storage.Exec(ctx, "UPDATE users SET telegram_chat_id=$1, updated_at=NOW() WHERE id=$2", chatID, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return err
}

func (r *UserRepository) FindUserByTelegramChatID(ctx context.Context, chatID int64) (*entities.User, error) {
	q := sq.Select("u.*", "s.code as status_code").From("users u").
		Join("statuses s ON u.status_id = s.id").
		Where(sq.Eq{"u.telegram_chat_id": chatID, "u.deleted_at": nil}).PlaceholderFormat(sq.Dollar)
	sqlStr, args, _ := q.ToSql()
	rows, err := r.storage.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[entities.User])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	return &u, err
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

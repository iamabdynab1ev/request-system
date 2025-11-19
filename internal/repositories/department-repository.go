package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
)

const departmentTable = "departments"

var (
	departmentAllowedFilterFields = map[string]string{"d.status_id": "d.status_id"}
	departmentAllowedSortFields   = map[string]bool{"id": true, "name": true, "created_at": true, "updated_at": true}
	departmentSelectFields        = []string{"id", "name", "status_id", "created_at", "updated_at", "external_id", "source_system"}
)

// DepartmentRepositoryInterface - ЧИСТАЯ ВЕРСИЯ БЕЗ ДУБЛИКАТОВ
type DepartmentRepositoryInterface interface {
	// Основные "рабочие" методы для изменения данных
	Create(ctx context.Context, tx pgx.Tx, department entities.Department) (uint64, error)
	Update(ctx context.Context, tx pgx.Tx, id uint64, department entities.Department) error

	// Методы для чтения данных
	FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Department, error)
	FindDepartment(ctx context.Context, id uint64) (*entities.Department, error)
	GetDepartments(ctx context.Context, filter types.Filter) ([]entities.Department, uint64, error)
	GetDepartmentsWithStats(ctx context.Context, filter types.Filter) ([]dto.DepartmentStatsDTO, uint64, error)
	DeleteDepartment(ctx context.Context, id uint64) error
	FindIDByName(ctx context.Context, name string) (uint64, error)
}

type DepartmentRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewDepartmentRepository(storage *pgxpool.Pool, logger *zap.Logger) DepartmentRepositoryInterface {
	return &DepartmentRepository{storage: storage, logger: logger}
}

func scanDepartment(row pgx.Row) (*entities.Department, error) {
	var d entities.Department
	var externalID, sourceSystem sql.NullString
	err := row.Scan(&d.ID, &d.Name, &d.StatusID, &d.CreatedAt, &d.UpdatedAt, &externalID, &sourceSystem)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка сканирования department: %w", err)
	}
	if externalID.Valid {
		d.ExternalID = &externalID.String
	}
	if sourceSystem.Valid {
		d.SourceSystem = &sourceSystem.String
	}
	return &d, nil
}

func (r *DepartmentRepository) Create(ctx context.Context, tx pgx.Tx, department entities.Department) (uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Insert(departmentTable).
		Columns("name", "status_id", "external_id", "source_system", "created_at", "updated_at").
		Values(department.Name, department.StatusID, department.ExternalID, department.SourceSystem, sq.Expr("NOW()"), sq.Expr("NOW()")).
		Suffix("RETURNING id").
		ToSql()
	if err != nil {
		return 0, err
	}
	var newID uint64
	err = tx.QueryRow(ctx, query, args...).Scan(&newID)
	return newID, err
}

func (r *DepartmentRepository) Update(ctx context.Context, tx pgx.Tx, id uint64, department entities.Department) error {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Update(departmentTable).
		Set("name", department.Name).
		Set("status_id", department.StatusID).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}
	result, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *DepartmentRepository) findOne(ctx context.Context, querier Querier, where sq.Eq) (*entities.Department, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select(departmentSelectFields...).From(departmentTable).Where(where).ToSql()
	if err != nil {
		return nil, err
	}
	return scanDepartment(querier.QueryRow(ctx, query, args...))
}

func (r *DepartmentRepository) FindByExternalID(ctx context.Context, tx pgx.Tx, externalID string, sourceSystem string) (*entities.Department, error) {
	var querier Querier = r.storage
	if tx != nil {
		querier = tx
	}
	return r.findOne(ctx, querier, sq.Eq{"external_id": externalID, "source_system": sourceSystem})
}

func (r *DepartmentRepository) FindDepartment(ctx context.Context, id uint64) (*entities.Department, error) {
	return r.findOne(ctx, r.storage, sq.Eq{"id": id})
}

func (r *DepartmentRepository) buildFilterQuery(filter types.Filter, tableAlias string) (string, []interface{}) {
	conditions := []string{}
	args := []interface{}{}
	argCounter := 1
	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("%s.name ILIKE $%d", tableAlias, argCounter))
		args = append(args, "%"+filter.Search+"%")
		argCounter++
	}
	for key, value := range filter.Filter {
		if dbColumn, ok := departmentAllowedFilterFields[key]; ok {
			items := strings.Split(fmt.Sprintf("%v", value), ",")
			if len(items) > 1 {
				placeholders := []string{}
				for _, item := range items {
					placeholders = append(placeholders, fmt.Sprintf("$%d", argCounter))
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
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func (r *DepartmentRepository) CountDepartments(ctx context.Context, filter types.Filter, tableAlias string) (uint64, error) {
	whereClause, args := r.buildFilterQuery(filter, tableAlias)
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s AS %s %s", departmentTable, tableAlias, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *DepartmentRepository) GetDepartments(ctx context.Context, filter types.Filter) ([]entities.Department, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	baseBuilder := psql.Select().From(departmentTable + " AS d")

	if filter.Search != "" {
		baseBuilder = baseBuilder.Where(sq.ILike{"d.name": "%" + filter.Search + "%"})
	}
	for key, value := range filter.Filter {
		if dbColumn, ok := departmentAllowedFilterFields[key]; ok {
			if items, ok := value.(string); ok && strings.Contains(items, ",") {
				baseBuilder = baseBuilder.Where(sq.Eq{dbColumn: strings.Split(items, ",")})
			} else {
				baseBuilder = baseBuilder.Where(sq.Eq{dbColumn: value})
			}
		}
	}

	countBuilder := baseBuilder.Columns("COUNT(d.id)")
	countQuery, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Department{}, 0, nil
	}

	selectBuilder := baseBuilder.Columns("d.id", "d.name", "d.status_id", "d.created_at", "d.updated_at", "d.external_id", "d.source_system")

	if len(filter.Sort) > 0 {
		for field, direction := range filter.Sort {
			if _, ok := departmentAllowedSortFields[field]; ok {
				order := "ASC"
				if strings.ToUpper(direction) == "DESC" {
					order = "DESC"
				}
				selectBuilder = selectBuilder.OrderBy(fmt.Sprintf("d.%s %s", field, order))
			}
		}
	} else {
		selectBuilder = selectBuilder.OrderBy("d.id DESC")
	}

	if filter.WithPagination {
		selectBuilder = selectBuilder.Limit(uint64(filter.Limit)).Offset(uint64(filter.Offset))
	}

	query, args, err := selectBuilder.ToSql()
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	departments := make([]entities.Department, 0)
	for rows.Next() {
		dept, err := scanDepartment(rows)
		if err != nil {
			return nil, 0, err
		}
		departments = append(departments, *dept)
	}
	return departments, total, rows.Err()
}

func (r *DepartmentRepository) CreateDepartment(ctx context.Context, department entities.Department) (*entities.Department, error) {
	query := `
		INSERT INTO departments (name, status_id, external_id, source_system) 
		VALUES($1, $2, $3, $4) 
		RETURNING id, name, status_id, created_at, updated_at, external_id, source_system`
	return scanDepartment(r.storage.QueryRow(ctx, query, department.Name, department.StatusID, department.ExternalID, department.SourceSystem))
}

func (r *DepartmentRepository) UpdateDepartment(ctx context.Context, id uint64, dto dto.UpdateDepartmentDTO) (*entities.Department, error) {
	r.logger.Warn("!!!!!!!!!! ВЫПОЛНЯЕТСЯ НОВЫЙ МЕТОД UPDATE В РЕПОЗИТОРИИ !!!!!!!!!!", zap.Any("dto", dto))

	updateBuilder := sq.Update(departmentTable).
		PlaceholderFormat(sq.Dollar).
		Where(sq.Eq{"id": id}).
		Set("updated_at", sq.Expr("NOW()"))
	hasChanges := false
	if dto.Name != nil {
		updateBuilder = updateBuilder.Set("name", *dto.Name)
		hasChanges = true
	}
	if dto.StatusID != nil {
		updateBuilder = updateBuilder.Set("status_id", *dto.StatusID)
		hasChanges = true
	}
	if !hasChanges {
		return r.FindDepartment(ctx, id)
	}
	query, args, err := updateBuilder.Suffix("RETURNING id, name, status_id, created_at, updated_at").ToSql()
	if err != nil {
		return nil, err
	}
	return scanDepartment(r.storage.QueryRow(ctx, query, args...))
}

func (r *DepartmentRepository) DeleteDepartment(ctx context.Context, id uint64) error {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Delete(departmentTable).Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return err
	}
	result, err := r.storage.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *DepartmentRepository) FindIDByName(ctx context.Context, name string) (uint64, error) {
	var id uint64
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Select("id").From(departmentTable).Where(sq.ILike{"name": name}).Limit(1).ToSql()
	if err != nil {
		return 0, err
	}
	err = r.storage.QueryRow(ctx, query, args...).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperrors.ErrNotFound
	}
	return id, err
}

func (r *DepartmentRepository) GetDepartmentsWithStats(ctx context.Context, filter types.Filter) ([]dto.DepartmentStatsDTO, uint64, error) {
	// Здесь тоже надо добавить deleted_at IS NULL в WHERE, если ты его используешь для orders
	whereClause, args := r.buildFilterQuery(filter, "d")
	total, err := r.CountDepartments(ctx, filter, "d")
	if err != nil || total == 0 {
		return []dto.DepartmentStatsDTO{}, total, err
	}
	mainQuery := `SELECT d.id, d.name, COUNT(o.id) FILTER (WHERE s.code = 'OPEN') AS open_orders, COUNT(o.id) FILTER (WHERE s.code = 'CLOSED') AS closed_orders FROM departments AS d LEFT JOIN orders AS o ON d.id = o.department_id AND o.deleted_at IS NULL LEFT JOIN statuses AS s ON o.status_id = s.id`
	orderByClause := " GROUP BY d.id, d.name ORDER BY d.id ASC "
	var finalQuery strings.Builder
	finalQuery.WriteString(mainQuery)
	// Важно! Where clause должен идти ПЕРЕД Group By
	finalQuery.WriteString(whereClause)
	finalQuery.WriteString(orderByClause)
	argCounter := len(args) + 1
	if filter.WithPagination {
		finalQuery.WriteString(fmt.Sprintf("LIMIT $%d OFFSET $%d", argCounter, argCounter+1))
		args = append(args, filter.Limit, filter.Offset)
	}
	rows, err := r.storage.Query(ctx, finalQuery.String(), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	stats := make([]dto.DepartmentStatsDTO, 0)
	for rows.Next() {
		var s dto.DepartmentStatsDTO
		if err := rows.Scan(&s.ID, &s.Name, &s.OpenOrdersCount, &s.ClosedOrdersCount); err != nil {
			r.logger.Error("ошибка сканирования статистики", zap.Error(err))
			return nil, 0, err
		}
		stats = append(stats, s)
	}
	return stats, total, rows.Err()
}

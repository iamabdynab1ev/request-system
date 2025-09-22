package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const equipmentTable = "equipments"

var equipmentAllowedFilterFields = map[string]string{
	"status_id":         "e.status_id",
	"branch_id":         "e.branch_id",
	"office_id":         "e.office_id",
	"equipment_type_id": "e.equipment_type_id",
}

// Сортировка остается как была, но тоже лучше использовать полное имя
var equipmentAllowedSortFields = map[string]string{
	"id":         "e.id",
	"name":       "e.name",
	"created_at": "e.created_at",
}

type EquipmentRepositoryInterface interface {
	GetEquipments(ctx context.Context, filter types.Filter) ([]entities.Equipment, uint64, error)
	FindEquipment(ctx context.Context, id uint64) (*entities.Equipment, error)
	CreateEquipment(ctx context.Context, eq entities.Equipment) (*entities.Equipment, error)
	UpdateEquipment(ctx context.Context, id uint64, eq entities.Equipment) (*entities.Equipment, error)
	DeleteEquipment(ctx context.Context, id uint64) error
	CountOrdersByEquipmentID(ctx context.Context, id uint64) (int, error)
}

type EquipmentRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewEquipmentRepository(storage *pgxpool.Pool, logger *zap.Logger) EquipmentRepositoryInterface {
	return &EquipmentRepository{storage: storage, logger: logger}
}

func scanEquipment(row pgx.Row) (*entities.Equipment, error) {
	var e entities.Equipment
	var b entities.Branch
	var o entities.Office
	var et entities.EquipmentType
	var s entities.Status
	var createdAt, updatedAt time.Time

	err := row.Scan(
		&e.ID, &e.Name, &e.Address, &e.BranchID, &e.OfficeID, &e.StatusID, &e.EquipmentTypeID, &createdAt, &updatedAt,
		&b.ID, &b.Name, &b.ShortName,
		&o.ID, &o.Name,
		&et.ID, &et.Name,
		&s.ID, &s.Name,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка сканирования equipment: %w", err)
	}

	e.CreatedAt, e.UpdatedAt, e.Branch, e.Office, e.EquipmentType, e.Status = &createdAt, &updatedAt, &b, &o, &et, &s
	return &e, nil
}

func (r *EquipmentRepository) GetEquipments(ctx context.Context, filter types.Filter) ([]entities.Equipment, uint64, error) {
	args := make([]interface{}, 0)
	conditions := []string{}
	argCounter := 1

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		conditions = append(conditions, fmt.Sprintf("(e.name ILIKE $%d OR e.address ILIKE $%d)", argCounter, argCounter))
		args = append(args, searchPattern)
		argCounter++
	}

	for key, value := range filter.Filter {
		if dbColumn, ok := equipmentAllowedFilterFields[key]; ok {
			if strVal, ok := value.(string); ok && strings.Contains(strVal, ",") {
				items := strings.Split(strVal, ",")
				placeholders := make([]string, len(items))
				for i, item := range items {
					placeholders[i] = fmt.Sprintf("$%d", argCounter)
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

	joinClause := fmt.Sprintf(`%s e
		LEFT JOIN branches b ON e.branch_id = b.id
		LEFT JOIN offices o ON e.office_id = o.id
		LEFT JOIN equipment_types et ON e.equipment_type_id = et.id
		LEFT JOIN statuses s ON e.status_id = s.id`, equipmentTable)

	var whereClause string
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(e.id) FROM %s e %s", equipmentTable, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		r.logger.Error("ошибка подсчета оборудования", zap.Error(err), zap.String("query", countQuery), zap.Any("args", args))
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Equipment{}, 0, nil
	}

	orderByClause := "ORDER BY e.id DESC" // Default order
	// Add sorting logic here if needed

	// Append args for pagination
	paginationArgs := make([]interface{}, 0)
	limitClause := ""
	if filter.WithPagination {
		limitClause = fmt.Sprintf("LIMIT $%d OFFSET $%d", argCounter, argCounter+1)
		paginationArgs = append(paginationArgs, filter.Limit, filter.Offset)
	}

	selectFields := `e.id, e.name, e.address, e.branch_id, e.office_id, e.status_id, e.equipment_type_id, e.created_at, e.updated_at,
		COALESCE(b.id, 0), COALESCE(b.name, ''), COALESCE(b.short_name, ''),
		COALESCE(o.id, 0), COALESCE(o.name, ''),
		COALESCE(et.id, 0), COALESCE(et.name, ''),
		COALESCE(s.id, 0), COALESCE(s.name, '')`

	queryBuilder := &strings.Builder{}
	fmt.Fprintf(queryBuilder, "SELECT %s FROM %s %s %s %s", selectFields, joinClause, whereClause, orderByClause, limitClause)

	finalQuery := queryBuilder.String()
	finalArgs := append(args, paginationArgs...)

	rows, err := r.storage.Query(ctx, finalQuery, finalArgs...)
	if err != nil {
		r.logger.Error("ошибка основного запроса", zap.Error(err), zap.String("query", finalQuery), zap.Any("args", finalArgs))
		return nil, 0, err
	}
	defer rows.Close()

	equipments := make([]entities.Equipment, 0)
	for rows.Next() {
		eq, err := scanEquipment(rows)
		if err != nil {
			return nil, 0, err
		}
		equipments = append(equipments, *eq)
	}
	return equipments, total, rows.Err()
}

func (r *EquipmentRepository) FindEquipment(ctx context.Context, id uint64) (*entities.Equipment, error) {
	joinClause := fmt.Sprintf(`%s e
		LEFT JOIN branches b ON e.branch_id = b.id
		LEFT JOIN offices o ON e.office_id = o.id
		LEFT JOIN equipment_types et ON e.equipment_type_id = et.id
		LEFT JOIN statuses s ON e.status_id = s.id`, equipmentTable)
	selectFields := `e.id, e.name, e.address, e.branch_id, e.office_id, e.status_id, e.equipment_type_id, e.created_at, e.updated_at,
		COALESCE(b.id, 0), COALESCE(b.name, ''), COALESCE(b.short_name, ''),
		COALESCE(o.id, 0), COALESCE(o.name, ''),
		COALESCE(et.id, 0), COALESCE(et.name, ''),
		COALESCE(s.id, 0), COALESCE(s.name, '')`
	query := fmt.Sprintf("SELECT %s FROM %s WHERE e.id = $1", selectFields, joinClause)
	return scanEquipment(r.storage.QueryRow(ctx, query, id))
}

func (r *EquipmentRepository) CreateEquipment(ctx context.Context, eq entities.Equipment) (*entities.Equipment, error) {
	query := fmt.Sprintf("INSERT INTO %s (name, address, branch_id, office_id, status_id, equipment_type_id) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id", equipmentTable)
	var createdID uint64
	err := r.storage.QueryRow(ctx, query, eq.Name, eq.Address, eq.BranchID, eq.OfficeID, eq.StatusID, eq.EquipmentTypeID).Scan(&createdID)
	if err != nil {
		return nil, err
	}
	return r.FindEquipment(ctx, createdID)
}

func (r *EquipmentRepository) UpdateEquipment(ctx context.Context, id uint64, eq entities.Equipment) (*entities.Equipment, error) {
	query := fmt.Sprintf(`UPDATE %s
        SET name = $1, address = $2, branch_id = $3, office_id = $4, status_id = $5, equipment_type_id = $6, updated_at = NOW()
		WHERE id = $7`, equipmentTable)
	result, err := r.storage.Exec(ctx, query, eq.Name, eq.Address, eq.BranchID, eq.OfficeID, eq.StatusID, eq.EquipmentTypeID, id)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, apperrors.ErrNotFound
	}
	return r.FindEquipment(ctx, id)
}

func (r *EquipmentRepository) DeleteEquipment(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", equipmentTable)
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *EquipmentRepository) CountOrdersByEquipmentID(ctx context.Context, id uint64) (int, error) {
	var count int
	query := "SELECT COUNT(id) FROM orders WHERE equipment_id = $1 AND deleted_at IS NULL"
	err := r.storage.QueryRow(ctx, query, id).Scan(&count)
	if err != nil {
		r.logger.Error("ошибка подсчета заявок по оборудованию", zap.Uint64("equipmentID", id), zap.Error(err))
		return 0, err
	}
	return count, nil
}

package repositories

import (
	"context"
	"errors"
	"fmt"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const equipmentTypeTable = "equipment_types"

var etAllowedFilterFields = map[string]bool{} // Пока нет полей для фильтрации
var etAllowedSortFields = map[string]bool{
	"id":         true,
	"name":       true,
	"created_at": true,
}

type EquipmentTypeRepositoryInterface interface {
	GetEquipmentTypes(ctx context.Context, filter types.Filter) ([]entities.EquipmentType, uint64, error)
	FindEquipmentType(ctx context.Context, id uint64) (*entities.EquipmentType, error)
	CreateEquipmentType(ctx context.Context, et entities.EquipmentType) (*entities.EquipmentType, error)
	UpdateEquipmentType(ctx context.Context, id uint64, et entities.EquipmentType) (*entities.EquipmentType, error)
	DeleteEquipmentType(ctx context.Context, id uint64) error
}

type EquipmentTypeRepository struct {
	storage *pgxpool.Pool
	logger  *zap.Logger
}

func NewEquipmentTypeRepository(storage *pgxpool.Pool, logger *zap.Logger) EquipmentTypeRepositoryInterface {
	return &EquipmentTypeRepository{
		storage: storage,
		logger:  logger,
	}
}

func scanEquipmentType(row pgx.Row) (*entities.EquipmentType, error) {
	var et entities.EquipmentType

	// Временные переменные для сканирования. Они нам все еще нужны.
	var createdAt, updatedAt time.Time

	err := row.Scan(&et.ID, &et.Name, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка сканирования equipment_type: %w", err)
	}

	et.CreatedAt = &createdAt
	et.UpdatedAt = &updatedAt

	return &et, nil
}

func (r *EquipmentTypeRepository) GetEquipmentTypes(ctx context.Context, filter types.Filter) ([]entities.EquipmentType, uint64, error) {
	allArgs := make([]interface{}, 0)
	conditions := []string{} // У справочников нет deleted_at
	placeholderNum := 1

	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", placeholderNum))
		allArgs = append(allArgs, searchPattern)
		placeholderNum++
	}

	var whereClause string
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(id) FROM %s %s", equipmentTypeTable, whereClause)
	var total uint64
	if err := r.storage.QueryRow(ctx, countQuery, allArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.EquipmentType{}, 0, nil
	}

	orderByClause := "ORDER BY id DESC"
	if len(filter.Sort) > 0 {
		var sortParts []string
		for field, direction := range filter.Sort {
			if etAllowedSortFields[field] {
				safeDirection := "ASC"
				if strings.ToLower(direction) == "desc" {
					safeDirection = "DESC"
				}
				sortParts = append(sortParts, fmt.Sprintf("%s %s", field, safeDirection))
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

	selectFields := "id, name, created_at, updated_at"
	mainQuery := fmt.Sprintf("SELECT %s FROM %s %s %s %s", selectFields, equipmentTypeTable, whereClause, orderByClause, limitClause)

	rows, err := r.storage.Query(ctx, mainQuery, allArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	equipmentTypes := make([]entities.EquipmentType, 0)
	for rows.Next() {
		et, err := scanEquipmentType(rows)
		if err != nil {
			return nil, 0, err
		}
		equipmentTypes = append(equipmentTypes, *et)
	}
	return equipmentTypes, total, rows.Err()
}

func (r *EquipmentTypeRepository) FindEquipmentType(ctx context.Context, id uint64) (*entities.EquipmentType, error) {
	query := `SELECT id, name, created_at, updated_at FROM equipment_types WHERE id = $1`
	return scanEquipmentType(r.storage.QueryRow(ctx, query, id))
}

func (r *EquipmentTypeRepository) CreateEquipmentType(ctx context.Context, et entities.EquipmentType) (*entities.EquipmentType, error) {
	query := `INSERT INTO equipment_types (name) VALUES($1) RETURNING id, name, created_at, updated_at`
	return scanEquipmentType(r.storage.QueryRow(ctx, query, et.Name))
}

func (r *EquipmentTypeRepository) UpdateEquipmentType(ctx context.Context, id uint64, et entities.EquipmentType) (*entities.EquipmentType, error) {
	query := `UPDATE equipment_types SET name = $1, updated_at = NOW() WHERE id = $2 RETURNING id, name, created_at, updated_at`
	return scanEquipmentType(r.storage.QueryRow(ctx, query, et.Name, id))
}

func (r *EquipmentTypeRepository) DeleteEquipmentType(ctx context.Context, id uint64) error {
	query := `DELETE FROM equipment_types WHERE id = $1`
	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

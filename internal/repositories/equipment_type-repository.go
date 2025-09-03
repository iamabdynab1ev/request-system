package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const equipmentTypeTable = "equipment_types"

var (
	etAllowedFilterFields = map[string]bool{}
	etAllowedSortFields   = map[string]bool{
		"id":         true,
		"name":       true,
		"created_at": true,
	}
)

type EquipmentTypeRepositoryInterface interface {
	GetEquipmentTypes(ctx context.Context, filter types.Filter) ([]entities.EquipmentType, uint64, error)
	FindEquipmentType(ctx context.Context, id uint64) (*entities.EquipmentType, error)
	CreateEquipmentType(ctx context.Context, et entities.EquipmentType) (*entities.EquipmentType, error)
	UpdateEquipmentType(ctx context.Context, id uint64, reqDTO dto.UpdateEquipmentTypeDTO) (*dto.EquipmentTypeDTO, error)
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
	conditions := []string{}
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
	query := `INSERT INTO equipment_types (name, created_at, updated_at) VALUES($1, $2, $3) RETURNING id, name, created_at, updated_at`
	return scanEquipmentType(r.storage.QueryRow(ctx, query, et.Name, et.CreatedAt, et.UpdatedAt))
}

func (r *EquipmentTypeRepository) UpdateEquipmentType(ctx context.Context, id uint64, reqDTO dto.UpdateEquipmentTypeDTO) (*dto.EquipmentTypeDTO, error) {
	updates := make([]string, 0)
	args := make([]interface{}, 0)
	argID := 1

	if reqDTO.Name != "" {
		updates = append(updates, fmt.Sprintf("name = $%d", argID))
		args = append(args, reqDTO.Name)
		argID++
	}

	if len(updates) == 0 {
		existingEntity, err := r.FindEquipmentType(ctx, id)
		if err != nil {
			return nil, err
		}
		createdAtStr := existingEntity.CreatedAt.Format("2006-01-02 15:04:05")
		updatedAtStr := existingEntity.UpdatedAt.Format("2006-01-02 15:04:05")
		return &dto.EquipmentTypeDTO{
			ID:        existingEntity.ID,
			Name:      existingEntity.Name,
			CreatedAt: &createdAtStr, // Передаем как *string
			UpdatedAt: &updatedAtStr, // Передаем как *string
		}, nil
	}

	updates = append(updates, fmt.Sprintf("updated_at = $%d", argID))
	args = append(args, time.Now())
	argID++

	args = append(args, id)
	selectFieldsForReturn := "id, name, created_at, updated_at"
	query := fmt.Sprintf(`UPDATE %s SET %s WHERE id = $%d RETURNING %s`, equipmentTypeTable, strings.Join(updates, ", "), argID, selectFieldsForReturn)

	var dtoResult dto.EquipmentTypeDTO
	var createdAtTime, updatedAtTime time.Time // Временные переменные для time.Time
	err := r.storage.QueryRow(ctx, query, args...).Scan(&dtoResult.ID, &dtoResult.Name, &createdAtTime, &updatedAtTime)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	createdAtStr := createdAtTime.Format("2006-01-02 15:04:05")
	updatedAtStr := updatedAtTime.Format("2006-01-02 15:04:05")
	dtoResult.CreatedAt = &createdAtStr
	dtoResult.UpdatedAt = &updatedAtStr

	return &dtoResult, nil
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

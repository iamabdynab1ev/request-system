package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"request-system/internal/entities"
	// Подключаем наш БД-хелпер
	"request-system/internal/infrastructure/bd"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
)

const equipmentTable = "equipments"

// ЕДИНАЯ КАРТА ПОЛЕЙ (Фильтрация + Сортировка)
// Префикс "e." обязателен, так как есть JOIN-ы
var equipmentMap = map[string]string{
	"id":                "e.id",
	"name":              "e.name",
	"address":           "e.address",
	"status_id":         "e.status_id",
	"branch_id":         "e.branch_id",
	"office_id":         "e.office_id",
	"equipment_type_id": "e.equipment_type_id",
	"created_at":        "e.created_at",
	"updated_at":        "e.updated_at",
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

// scanEquipment: Сканирование строки в структуру со связями
func (r *EquipmentRepository) scanEquipment(row pgx.Row) (*entities.Equipment, error) {
	var e entities.Equipment
	var b entities.Branch
	var o entities.Office
	var et entities.EquipmentType
	var s entities.Status
	var createdAt, updatedAt time.Time
	
	// ВАЖНО: Используем переменные-указатели для сканирования NULL значений
	// Entity.BranchID и Entity.OfficeID у вас уже *uint64, поэтому &e.BranchID — это **uint64. 
    // Драйверу проще, когда мы передаем адрес указателя напрямую.
	err := row.Scan(
		&e.ID, &e.Name, &e.Address, 
		&e.BranchID, &e.OfficeID, // Сканируем прямо в указатели структуры
		&e.StatusID, &e.EquipmentTypeID, &createdAt, &updatedAt,
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

	e.CreatedAt = &createdAt
	e.UpdatedAt = &updatedAt

	if e.BranchID != nil && *e.BranchID > 0 { e.Branch = &b }
	if e.OfficeID != nil && *e.OfficeID > 0 { e.Office = &o }
	if e.EquipmentTypeID > 0 { e.EquipmentType = &et }
	e.Status = &s

	return &e, nil
}

// --------------------------------------------------------
// GetEquipments
// --------------------------------------------------------
func (r *EquipmentRepository) GetEquipments(ctx context.Context, filter types.Filter) ([]entities.Equipment, uint64, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	
	// Функция для текстового поиска
	applySearch := func(b sq.SelectBuilder) sq.SelectBuilder {
		if filter.Search != "" {
			pat := "%" + filter.Search + "%"
			return b.Where(sq.Or{
				sq.ILike{"e.name": pat},
				sq.ILike{"e.address": pat},
			})
		}
		return b
	}

	// 1. COUNT
	countBuilder := psql.Select("COUNT(e.id)").From(equipmentTable + " e")
	countBuilder = applySearch(countBuilder)

	countFilter := filter
	countFilter.WithPagination = false
	countFilter.Sort = nil

	// Helper
	countBuilder = bd.ApplyListParams(countBuilder, countFilter, equipmentMap)

	countSql, countArgs, err := countBuilder.ToSql()
	if err != nil { return nil, 0, err }

	var total uint64
	if err := r.storage.QueryRow(ctx, countSql, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []entities.Equipment{}, 0, nil
	}

	// 2. SELECT
	// Список полей строго как в scanEquipment
	baseBuilder := psql.Select(
		"e.id", "e.name", "e.address", "e.branch_id", "e.office_id", "e.status_id", "e.equipment_type_id", "e.created_at", "e.updated_at",
		"COALESCE(b.id, 0)", "COALESCE(b.name, '')", "COALESCE(b.short_name, '')",
		"COALESCE(o.id, 0)", "COALESCE(o.name, '')",
		"COALESCE(et.id, 0)", "COALESCE(et.name, '')",
		"COALESCE(s.id, 0)", "COALESCE(s.name, '')",
	).
		From(equipmentTable + " e").
		LeftJoin("branches b ON e.branch_id = b.id").
		LeftJoin("offices o ON e.office_id = o.id").
		LeftJoin("equipment_types et ON e.equipment_type_id = et.id").
		LeftJoin("statuses s ON e.status_id = s.id")

	baseBuilder = applySearch(baseBuilder)

	if len(filter.Sort) == 0 {
		baseBuilder = baseBuilder.OrderBy("e.id DESC")
	}

	// Helper
	baseBuilder = bd.ApplyListParams(baseBuilder, filter, equipmentMap)

	query, args, err := baseBuilder.ToSql()
	if err != nil { return nil, 0, err }

	rows, err := r.storage.Query(ctx, query, args...)
	if err != nil { return nil, 0, err }
	defer rows.Close()

	equipments := make([]entities.Equipment, 0)
	for rows.Next() {
		eq, err := r.scanEquipment(rows)
		if err != nil { return nil, 0, err }
		equipments = append(equipments, *eq)
	}

	return equipments, total, rows.Err()
}

func (r *EquipmentRepository) FindEquipment(ctx context.Context, id uint64) (*entities.Equipment, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	queryBuilder := psql.Select(
		"e.id", "e.name", "e.address", "e.branch_id", "e.office_id", "e.status_id", "e.equipment_type_id", "e.created_at", "e.updated_at",
		"COALESCE(b.id, 0)", "COALESCE(b.name, '')", "COALESCE(b.short_name, '')",
		"COALESCE(o.id, 0)", "COALESCE(o.name, '')",
		"COALESCE(et.id, 0)", "COALESCE(et.name, '')",
		"COALESCE(s.id, 0)", "COALESCE(s.name, '')",
	).
		From(equipmentTable + " e").
		LeftJoin("branches b ON e.branch_id = b.id").
		LeftJoin("offices o ON e.office_id = o.id").
		LeftJoin("equipment_types et ON e.equipment_type_id = et.id").
		LeftJoin("statuses s ON e.status_id = s.id").
		Where(sq.Eq{"e.id": id})

	sqlStr, args, err := queryBuilder.ToSql()
	if err != nil { return nil, err }

	return r.scanEquipment(r.storage.QueryRow(ctx, sqlStr, args...))
}



func (r *EquipmentRepository) CreateEquipment(ctx context.Context, eq entities.Equipment) (*entities.Equipment, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	
	query, args, err := psql.Insert(equipmentTable).
		Columns("name", "address", "branch_id", "office_id", "status_id", "equipment_type_id").
		Values(eq.Name, eq.Address, eq.BranchID, eq.OfficeID, eq.StatusID, eq.EquipmentTypeID).
		Suffix("RETURNING id").
		ToSql()

	if err != nil { return nil, err }

	var createdID uint64
	if err := r.storage.QueryRow(ctx, query, args...).Scan(&createdID); err != nil {
		return nil, err
	}
	return r.FindEquipment(ctx, createdID)
}

func (r *EquipmentRepository) UpdateEquipment(ctx context.Context, id uint64, eq entities.Equipment) (*entities.Equipment, error) {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	query, args, err := psql.Update(equipmentTable).
		Set("name", eq.Name).
		Set("address", eq.Address).
		Set("branch_id", eq.BranchID).
		Set("office_id", eq.OfficeID).   
		Set("status_id", eq.StatusID).
		Set("equipment_type_id", eq.EquipmentTypeID).
		Set("updated_at", sq.Expr("NOW()")).
		Where(sq.Eq{"id": id}).
		ToSql()
	
	if err != nil { return nil, err }

	result, err := r.storage.Exec(ctx, query, args...)
	if err != nil { return nil, err }

	if result.RowsAffected() == 0 { return nil, apperrors.ErrNotFound }

	return r.FindEquipment(ctx, id)
}

func (r *EquipmentRepository) DeleteEquipment(ctx context.Context, id uint64) error {
	psql := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	query, args, err := psql.Delete(equipmentTable).Where(sq.Eq{"id": id}).ToSql()
	if err != nil { return err }

	result, err := r.storage.Exec(ctx, query, args...)
	if err != nil { return err }
	if result.RowsAffected() == 0 { return apperrors.ErrNotFound }
	return nil
}

func (r *EquipmentRepository) CountOrdersByEquipmentID(ctx context.Context, id uint64) (int, error) {
	var count int
	err := r.storage.QueryRow(ctx, 
		"SELECT COUNT(id) FROM orders WHERE equipment_id = $1 AND deleted_at IS NULL", id).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

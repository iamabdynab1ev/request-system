package repositories

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const equipmentTableForJoinFinal = "equipments"
const equipmentFieldsForJoinFinal = "e.id, e.name, e.address, e.branch_id, e.office_id, e.status_id, e.equipment_type_id, e.created_at, e.updated_at"
const branchTableForEquipmentJoinFinal = "branches"
const branchFieldsForEquipmentJoinFinal = "b.id, b.name"
const officeTableForEquipmentJoinFinal = "offices"
const officeFieldsForEquipmentJoinFinal = "o.id, o.name"
const equipmentTypeTableForEquipmentJoinFinal = "equipment_types"
const equipmentTypeFieldsForEquipmentJoinFinal = "et.id, et.name"

type EquipmentRepositoryInterface interface {
	GetEquipments(ctx context.Context, limit uint64, offset uint64) (interface{}, uint64, error)
	FindEquipment(ctx context.Context, id uint64) (*dto.EquipmentDTO, error)
	CreateEquipment(ctx context.Context, dto dto.CreateEquipmentDTO) error
	UpdateEquipment(ctx context.Context, id uint64, dto dto.UpdateEquipmentDTO) error
	DeleteEquipment(ctx context.Context, id uint64) error
}

type EquipmentRepository struct {
	storage *pgxpool.Pool
}

func NewEquipmentRepository(storage *pgxpool.Pool) EquipmentRepositoryInterface {

	return &EquipmentRepository{
		storage: storage,
	}
}

func (r *EquipmentRepository) GetEquipments(ctx context.Context, limit uint64, offset uint64) (interface{}, uint64, error) {
	data, total, err := FetchDataAndCount(ctx, r.storage, Params{
		Table:   "equipments",
		Columns: "equipments.*, branch.id AS branch_id, branch.name AS branch_name, offices.id AS office_id, offices.name AS office_name, equipment_types.id AS equipment_type_id, equipment_types.name AS equipment_type_name",
		Relations: []Join{
			{Table: "branches", Alias: "branch", OnLeft: "branch.id", OnRight: "equipments.branch_id", JoinType: "LEFT"},
			{Table: "offices", Alias: "offices", OnLeft: "offices.id", OnRight: "equipments.office_id", JoinType: "LEFT"},
			{Table: "equipment_types", Alias: "equipment_types", OnLeft: "equipment_types.id", OnRight: "equipments.equipment_type_id", JoinType: "LEFT"},
		},
		WithPg: true,
		Limit:  limit,
		Offset: offset,
		Filter: map[string]interface{}{},
	})

	return data, total, err
}

func (r *EquipmentRepository) FindEquipment(ctx context.Context, id uint64) (*dto.EquipmentDTO, error) {
	query := fmt.Sprintf(`
		SELECT 
			%s,
			%s,
			%s,
			%s
		FROM %s e
			LEFT JOIN %s b ON e.branch_id = b.id
			LEFT JOIN %s o ON e.office_id = o.id
			LEFT JOIN %s et ON e.equipment_type_id = et.id
		WHERE e.id = $1
	`,
		equipmentFieldsForJoinFinal,
		branchFieldsForEquipmentJoinFinal,
		officeFieldsForEquipmentJoinFinal,
		equipmentTypeFieldsForEquipmentJoinFinal,
		equipmentTableForJoinFinal,
		branchTableForEquipmentJoinFinal,
		officeTableForEquipmentJoinFinal,
		equipmentTypeTableForEquipmentJoinFinal,
	)

	var equipment dto.EquipmentDTO
	var branch dto.ShortBranchDTO
	var office dto.ShortOfficeDTO
	var equipmentType dto.ShortEquipmentTypeDTO

	var createdAt time.Time
	var updatedAt time.Time
	var branchesIDFromEquipmentTable int
	var officeIDFromEquipmentTable int
	var statusIDFromEquipmentTable int
	var equipmentTypeIDFromEquipmentTable int

	err := r.storage.QueryRow(ctx, query, id).Scan(
		&equipment.ID,
		&equipment.Name,
		&equipment.Address,

		&branchesIDFromEquipmentTable,
		&officeIDFromEquipmentTable,
		&statusIDFromEquipmentTable,
		&equipmentTypeIDFromEquipmentTable,

		&createdAt,
		&updatedAt,

		&branch.ID,
		&branch.Name,

		&office.ID,
		&office.Name,

		&equipmentType.ID,
		&equipmentType.Name,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	equipment.CreatedAt = createdAt.Format("2006-01-02, 15:04:05")
	equipment.UpdatedAt = updatedAt.Format("2006-01-02, 15:04:05")
	equipment.Branch = branch
	equipment.Office = office
	equipment.EquipmentType = equipmentType
	equipment.StatusID = statusIDFromEquipmentTable

	return &equipment, nil
}

func (r *EquipmentRepository) CreateEquipment(ctx context.Context, dto dto.CreateEquipmentDTO) error {

	query := fmt.Sprintf(`
        INSERT INTO %s (name, address, branch_id, office_id, status_id, equipment_type_id)
        VALUES ($1, $2, $3, $4, $5, $6)
    `,
		equipmentTableForJoinFinal)

	_, err := r.storage.Exec(ctx, query,
		dto.Name,
		dto.Address,
		dto.BranchID,
		dto.OfficeID,
		dto.StatusID,
		dto.EquipmentTypeID,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *EquipmentRepository) UpdateEquipment(ctx context.Context, id uint64, dto dto.UpdateEquipmentDTO) error {

	query := fmt.Sprintf(`
        UPDATE %s
        SET name = $1, address = $2, branch_id = $3, office_id = $4, status_id = $5, equipment_type_id = $6, updated_at = CURRENT_TIMESTAMP
		WHERE id = $7
    `, equipmentTableForJoinFinal)

	result, err := r.storage.Exec(ctx, query,
		dto.Name,
		dto.Address,
		dto.BranchID,
		dto.OfficeID,
		dto.StatusID,
		dto.EquipmentTypeID,
		id,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *EquipmentRepository) DeleteEquipment(ctx context.Context, id uint64) error {

	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", equipmentTableForJoinFinal)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return nil
}

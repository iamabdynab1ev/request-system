package repositories

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	EQUIPMENT_TYPE_TABLE  = "equipment_types"
	EQUIPMENT_TYPE_FIELDS = "id, name, created_at, updated_at"
)

type EquipmentTypeRepositoryInterface interface {
	GetEquipmentTypes(ctx context.Context, limit uint64, offset uint64) ([]dto.EquipmentTypeDTO, error)
	FindEquipmentType(ctx context.Context, id uint64) (*dto.EquipmentTypeDTO, error)
	CreateEquipmentType(ctx context.Context, dto dto.CreateEquipmentTypeDTO) error
	UpdateEquipmentType(ctx context.Context, id uint64, dto dto.UpdateEquipmentTypeDTO) error
	DeleteEquipmentType(ctx context.Context, id uint64) error
}

type EquipmentTypeRepository struct{
	storage *pgxpool.Pool
}

func NewEquipmentTypeRepository(storage *pgxpool.Pool) EquipmentTypeRepositoryInterface {

	return &EquipmentTypeRepository{
		storage: storage,
	}
}

func (r *EquipmentTypeRepository) GetEquipmentTypes(ctx context.Context, limit uint64, offset uint64) ([]dto.EquipmentTypeDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		`, EQUIPMENT_TYPE_FIELDS, EQUIPMENT_TYPE_TABLE)

	rows, err := r.storage.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	equipmentTypes := make([]dto.EquipmentTypeDTO, 0)

	for rows.Next() {
		var equipmentType dto.EquipmentTypeDTO
		var createdAt time.Time
        var updatedAt time.Time

		err := rows.Scan(
			&equipmentType.ID,
			&equipmentType.Name,
			&createdAt,
            &updatedAt,
		)

		if err != nil {
			return nil, err
		}

        createdAtLocal := createdAt.Local()
        updatedAtLocal := updatedAt.Local()

		equipmentType.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
        equipmentType.UpdatedAt = updatedAtLocal.Format("2006-01-02 15:04:05")


		equipmentTypes = append(equipmentTypes, equipmentType)
	}

	if err:= rows.Err(); err != nil {
		return nil, err
	}
	return equipmentTypes, nil
}

func (r *EquipmentTypeRepository) FindEquipmentType(ctx context.Context, id uint64) (*dto.EquipmentTypeDTO, error) {
	query := fmt.Sprintf(`
		SELECT
			%s
		FROM %s r
		WHERE r.id = $1
	`, EQUIPMENT_TYPE_FIELDS, EQUIPMENT_TYPE_TABLE)

	var equipmentType dto.EquipmentTypeDTO
	var createdAt time.Time
    var updatedAt time.Time


	err := r.storage.QueryRow(ctx, query, id).Scan(
		&equipmentType.ID,
		&equipmentType.Name,
		&createdAt,
        &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, utils.ErrorNotFound
		}
		return nil, err
	}

    createdAtLocal := createdAt.Local()
    updatedAtLocal := updatedAt.Local()

	equipmentType.CreatedAt = createdAtLocal.Format("2006-01-02 15:04:05")
    equipmentType.UpdatedAt = updatedAtLocal.Format("2006-01-02 15:04:05")


	return &equipmentType, nil
}

func (r *EquipmentTypeRepository) CreateEquipmentType(ctx context.Context, dto dto.CreateEquipmentTypeDTO) error {
	query := fmt.Sprintf(`
        INSERT INTO %s (name)
        VALUES ($1)
    `, EQUIPMENT_TYPE_TABLE)

	_, err := r.storage.Exec(ctx, query,
		dto.Name,
	)

	if err != nil {
		return err
	}
	return nil
}

func (r *EquipmentTypeRepository) UpdateEquipmentType(ctx context.Context, id uint64, dto dto.UpdateEquipmentTypeDTO) error {
	query := fmt.Sprintf(`
        UPDATE %s
        SET name = $1, updated_at = CURRENT_TIMESTAMP
        WHERE id = $2
    `, EQUIPMENT_TYPE_TABLE)

	result, err := r.storage.Exec(ctx, query,
		dto.Name,
        id,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}
	return nil
}

func (r *EquipmentTypeRepository) DeleteEquipmentType(ctx context.Context, id uint64) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", EQUIPMENT_TYPE_TABLE)

	result, err := r.storage.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return utils.ErrorNotFound
	}

	return nil
}
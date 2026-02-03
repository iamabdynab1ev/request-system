package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

// Интерфейс для сервиса
type EquipmentServiceInterface interface {
	GetEquipments(ctx context.Context, filter types.Filter) ([]dto.EquipmentListResponseDTO, uint64, error)
	FindEquipment(ctx context.Context, id uint64) (*dto.EquipmentDTO, error)
	CreateEquipment(ctx context.Context, dto dto.CreateEquipmentDTO) (*dto.EquipmentDTO, error)
	UpdateEquipment(ctx context.Context, id uint64, dto dto.UpdateEquipmentDTO) (*dto.EquipmentDTO, error)
	DeleteEquipment(ctx context.Context, id uint64) error
}

type EquipmentService struct {
	eqRepository   repositories.EquipmentRepositoryInterface
	userRepository repositories.UserRepositoryInterface // Нужен для проверки прав
	logger         *zap.Logger
}

func NewEquipmentService(
	eqRepo repositories.EquipmentRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) EquipmentServiceInterface {
	return &EquipmentService{
		eqRepository:   eqRepo,
		userRepository: userRepo,
		logger:         logger,
	}
}

// Хелпер для проверки прав
func (s *EquipmentService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := s.userRepository.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	return &authz.Context{Actor: actor, Permissions: permissionsMap}, nil
}

// "Переводчик" из Entity в DTO. Справляется со всеми связанными сущностями.
func eqEntityToDTO(entity *entities.Equipment) *dto.EquipmentDTO {
	if entity == nil {
		return nil
	}

	var createdAt, updatedAt string
	if entity.CreatedAt != nil {
		createdAt = entity.CreatedAt.Format("2006-01-02 15:04:05")
	}
	if entity.UpdatedAt != nil {
		updatedAt = entity.UpdatedAt.Format("2006-01-02 15:04:05")
	}

	dtoResponse := &dto.EquipmentDTO{
		ID:        entity.ID,
		Name:      entity.Name,
		Address:   entity.Address,
		StatusID:  entity.StatusID,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	// >>> ВОТ ИСПРАВЛЕНИЕ: Мы не пишем 'dto.' перед типами из того же пакета <<<
	if entity.Branch != nil {
		dtoResponse.Branch = dto.ShortBranchDTO{
			ID:        entity.Branch.ID,
			Name:      entity.Branch.Name,
			ShortName: entity.Branch.ShortName,
		}
	}
	if entity.Office != nil {
		dtoResponse.Office = dto.ShortOfficeDTO{
			ID:   uint64(entity.Office.ID),
			Name: entity.Office.Name,
		}
	}
	if entity.EquipmentType != nil {
		dtoResponse.EquipmentType = dto.ShortEquipmentTypeDTO{
			ID:   uint64(entity.EquipmentType.ID),
			Name: entity.EquipmentType.Name,
		}
	}

	return dtoResponse
}

// ----- РАБОЧИЕ МЕТОДЫ СЕРВИСА -----

func (s *EquipmentService) GetEquipments(ctx context.Context, filter types.Filter) ([]dto.EquipmentListResponseDTO, uint64, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.EquipmentsView, *authCtx) {
		return nil, 0, apperrors.ErrForbidden
	}

	entities, total, err := s.eqRepository.GetEquipments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// >>> ИЗМЕНЕНИЕ ЗДЕСЬ: Мы конвертируем в новый DTO для списка <<<
	dtos := make([]dto.EquipmentListResponseDTO, 0, len(entities))
	for _, eq := range entities {

		var createdAt, updatedAt string
		if eq.CreatedAt != nil {
			createdAt = eq.CreatedAt.Format("2006-01-02 15:04:05")
		}
		if eq.UpdatedAt != nil {
			updatedAt = eq.UpdatedAt.Format("2006-01-02 15:04:05")
		}

		dtos = append(dtos, dto.EquipmentListResponseDTO{
			ID:              eq.ID,
			Name:            eq.Name,
			Address:         eq.Address,
			BranchID:        eq.BranchID,
			OfficeID:        eq.OfficeID,
			EquipmentTypeID: eq.EquipmentTypeID,
			StatusID:        eq.StatusID,
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
		})
	}
	return dtos, total, nil
}

func (s *EquipmentService) FindEquipment(ctx context.Context, id uint64) (*dto.EquipmentDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.EquipmentsView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	entity, err := s.eqRepository.FindEquipment(ctx, id)
	if err != nil {
		return nil, err
	}
	return eqEntityToDTO(entity), nil
}

func (s *EquipmentService) CreateEquipment(ctx context.Context, dto dto.CreateEquipmentDTO) (*dto.EquipmentDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.EquipmentsCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	now := time.Now()
	entity := entities.Equipment{
		Name:            dto.Name,
		Address:         dto.Address,
		BranchID:        dto.BranchID,
		OfficeID:        dto.OfficeID,
		StatusID:        dto.StatusID,
		EquipmentTypeID: dto.EquipmentTypeID,
		BaseEntity: types.BaseEntity{
			CreatedAt: &now,
			UpdatedAt: &now,
		},
	}

	createdEntity, err := s.eqRepository.CreateEquipment(ctx, entity)
	if err != nil {
		s.logger.Error("Ошибка создания оборудования в репозитории", zap.Error(err))
		return nil, err
	}
	return eqEntityToDTO(createdEntity), nil
}

func (s *EquipmentService) UpdateEquipment(ctx context.Context, id uint64, dto dto.UpdateEquipmentDTO) (*dto.EquipmentDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.EquipmentsUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	// Сначала получаем сущность, чтобы не потерять CreatedAt
	existingEntity, err := s.eqRepository.FindEquipment(ctx, id)
	if err != nil {
		return nil, err
	}

	if dto.Name != nil {
		existingEntity.Name = *dto.Name
	}
	if dto.Address != nil {
		existingEntity.Address = *dto.Address
	}
	if dto.BranchID != nil {
		existingEntity.BranchID = dto.BranchID
	}
	if dto.OfficeID != nil {
		existingEntity.OfficeID = dto.OfficeID
	}
	if dto.StatusID != nil {
		existingEntity.StatusID = *dto.StatusID
	}
	if dto.EquipmentTypeID != nil {
		existingEntity.EquipmentTypeID = *dto.EquipmentTypeID
	}

	now := time.Now()
	existingEntity.UpdatedAt = &now

	updatedEntity, err := s.eqRepository.UpdateEquipment(ctx, id, *existingEntity)
	if err != nil {
		s.logger.Error("Ошибка обновления оборудования в репозитории", zap.Error(err))
		return nil, err
	}

	return eqEntityToDTO(updatedEntity), nil
}

func (s *EquipmentService) DeleteEquipment(ctx context.Context, id uint64) error {
	// 1. Авторизация
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.EquipmentsDelete, *authCtx) {
		return apperrors.ErrForbidden
	}

	// 2. Проверяем, существует ли такое оборудование вообще
	_, err = s.eqRepository.FindEquipment(ctx, id)
	if err != nil {
		return err // Если не найдено, вернется правильная 404 ошибка
	}

	// 3. НОВАЯ ЛОГИКА: Проверяем, используется ли оборудование в заявках
	orderCount, err := s.eqRepository.CountOrdersByEquipmentID(ctx, id)
	if err != nil {
		return apperrors.ErrInternalServer
	}

	// 4. Если используется - возвращаем понятную ошибку
	if orderCount > 0 {
		errorMessage := fmt.Sprintf(
			"Невозможно удалить оборудование, так как оно используется в %d заявках.",
			orderCount,
		)
		s.logger.Warn("Попытка удаления используемого оборудования",
			zap.Uint64("equipmentID", id),
			zap.Int("orderCount", orderCount),
		)
		return apperrors.NewHttpError(http.StatusConflict, errorMessage, nil, nil)
	}

	// 5. Если все проверки пройдены - удаляем
	return s.eqRepository.DeleteEquipment(ctx, id)
}

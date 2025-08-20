package services

import (
	"context"
	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
	"time"

	"go.uber.org/zap"
)

// Интерфейс для сервиса (хорошая практика)
type EquipmentTypeServiceInterface interface {
	GetEquipmentTypes(ctx context.Context, filter types.Filter) ([]dto.EquipmentTypeDTO, uint64, error)
	FindEquipmentType(ctx context.Context, id uint64) (*dto.EquipmentTypeDTO, error)
	CreateEquipmentType(ctx context.Context, dto dto.CreateEquipmentTypeDTO) (*dto.EquipmentTypeDTO, error)
	UpdateEquipmentType(ctx context.Context, id uint64, dto dto.UpdateEquipmentTypeDTO) (*dto.EquipmentTypeDTO, error)
	DeleteEquipmentType(ctx context.Context, id uint64) error
}

type EquipmentTypeService struct {
	etRepository   repositories.EquipmentTypeRepositoryInterface
	userRepository repositories.UserRepositoryInterface // Нужен для проверки прав
	logger         *zap.Logger
}

// Конструктор теперь тоже возвращает интерфейс
func NewEquipmentTypeService(
	etRepo repositories.EquipmentTypeRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) EquipmentTypeServiceInterface {
	return &EquipmentTypeService{
		etRepository:   etRepo,
		userRepository: userRepo,
		logger:         logger,
	}
}

// Хелпер для проверки прав (чтобы не дублировать код)
func (s *EquipmentTypeService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
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

// Хелпер "переводчик" из Entity в DTO
func etEntityToDTO(entity *entities.EquipmentType) *dto.EquipmentTypeDTO {
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

	return &dto.EquipmentTypeDTO{
		ID:        entity.ID,
		Name:      entity.Name,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

// ----- ИСПРАВЛЕННЫЕ МЕТОДЫ СЕРВИСА -----

func (s *EquipmentTypeService) GetEquipmentTypes(ctx context.Context, filter types.Filter) ([]dto.EquipmentTypeDTO, uint64, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.EquipmentTypesView, *authCtx) {
		return nil, 0, apperrors.ErrForbidden
	}

	entities, total, err := s.etRepository.GetEquipmentTypes(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]dto.EquipmentTypeDTO, 0, len(entities))
	for _, et := range entities {
		dtos = append(dtos, *etEntityToDTO(&et))
	}
	return dtos, total, nil
}

func (s *EquipmentTypeService) FindEquipmentType(ctx context.Context, id uint64) (*dto.EquipmentTypeDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.EquipmentTypesView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	entity, err := s.etRepository.FindEquipmentType(ctx, id)
	if err != nil {
		return nil, err
	}
	return etEntityToDTO(entity), nil
}

func (s *EquipmentTypeService) CreateEquipmentType(ctx context.Context, dto dto.CreateEquipmentTypeDTO) (*dto.EquipmentTypeDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.EquipmentTypesCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	now := time.Now()
	entity := entities.EquipmentType{
		Name: dto.Name,
		BaseEntity: types.BaseEntity{ // Инициализируем BaseEntity
			CreatedAt: &now,
			UpdatedAt: &now,
		},
	}
	createdEntity, err := s.etRepository.CreateEquipmentType(ctx, entity)
	if err != nil {
		s.logger.Error("Ошибка при создании типа оборудования в репозитории", zap.Error(err))
		return nil, err
	}
	return etEntityToDTO(createdEntity), nil
}

func (s *EquipmentTypeService) UpdateEquipmentType(ctx context.Context, id uint64, dto dto.UpdateEquipmentTypeDTO) (*dto.EquipmentTypeDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.EquipmentTypesUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	// Сначала получаем сущность, чтобы не потерять CreatedAt
	existingEntity, err := s.etRepository.FindEquipmentType(ctx, id)
	if err != nil {
		return nil, err
	}

	// Обновляем только нужные поля
	existingEntity.Name = dto.Name
	now := time.Now()
	existingEntity.UpdatedAt = &now

	updatedEntity, err := s.etRepository.UpdateEquipmentType(ctx, id, *existingEntity)
	if err != nil {
		s.logger.Error("Ошибка при обновлении типа оборудования в репозитории", zap.Error(err))
		return nil, err
	}
	return etEntityToDTO(updatedEntity), nil
}

func (s *EquipmentTypeService) DeleteEquipmentType(ctx context.Context, id uint64) error {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.EquipmentTypesDelete, *authCtx) {
		return apperrors.ErrForbidden
	}
	return s.etRepository.DeleteEquipmentType(ctx, id)
}

// Файл: internal/services/order_type_service.go

package services

import (
	"context"
	"errors"
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

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type OrderTypeServiceInterface interface {
	Create(ctx context.Context, createDTO dto.CreateOrderTypeDTO) (*dto.OrderTypeResponseDTO, error)
	Update(ctx context.Context, id int, updateDTO dto.UpdateOrderTypeDTO) (*dto.OrderTypeResponseDTO, error)
	Delete(ctx context.Context, id int) error
	GetByID(ctx context.Context, id int) (*dto.OrderTypeResponseDTO, error)
	GetAll(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.OrderTypeResponseDTO], error)
	// --- ИНТЕРФЕЙС ИЗМЕНЕН ---
	GetConfig(ctx context.Context, orderTypeID uint64) (map[string]interface{}, error)
}

type OrderTypeService struct {
	repo       repositories.OrderTypeRepositoryInterface
	userRepo   repositories.UserRepositoryInterface
	txManager  repositories.TxManagerInterface
	ruleEngine RuleEngineServiceInterface // <--- ДОБАВЛЕНО
	logger     *zap.Logger
}

func NewOrderTypeService(
	repo repositories.OrderTypeRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	txManager repositories.TxManagerInterface,
	ruleEngine RuleEngineServiceInterface, // <--- ДОБАВЛЕНО
	logger *zap.Logger,
) OrderTypeServiceInterface {
	return &OrderTypeService{
		repo:       repo,
		userRepo:   userRepo,
		txManager:  txManager,
		ruleEngine: ruleEngine, // <--- ДОБАВЛЕНО
		logger:     logger,
	}
}

// buildAuthzContext - вспомогательная функция для создания контекста авторизации.
func (s *OrderTypeService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: nil}, nil
}

// toResponseDTO конвертирует сущность OrderType в DTO для ответа.
func toResponseDTO(entity *entities.OrderType) *dto.OrderTypeResponseDTO {
	if entity == nil {
		return nil
	}

	resp := &dto.OrderTypeResponseDTO{
		ID:        uint64(entity.ID),
		Name:      entity.Name,
		StatusID:  entity.StatusID,
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}

	if entity.Code != nil {
		resp.Code = *entity.Code
	}

	return resp
}

// Create создает новый тип заявки.
func (s *OrderTypeService) Create(ctx context.Context, createDTO dto.CreateOrderTypeDTO) (*dto.OrderTypeResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	// ПРЕДПОЛОЖЕНИЕ: Мы добавим эти привилегии в сидеры.
	// Они аналогичны вашим `StatusesCreate`, `PrioritiesCreate` и т.д.
	if !authz.CanDo("order_type:create", *authContext) {
		return nil, apperrors.ErrForbidden
	}

	var newID int
	now := time.Now()

	orderTypeEntity := &entities.OrderType{
		Name:       createDTO.Name,
		Code:       createDTO.Code,
		StatusID:   createDTO.StatusID,
		BaseEntity: types.BaseEntity{CreatedAt: &now, UpdatedAt: &now},
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		nameExists, err := s.repo.ExistsByName(ctx, tx, createDTO.Name, 0)
		if err != nil {
			return err
		}
		if nameExists {
			return apperrors.NewHttpError(http.StatusBadRequest, "Тип заявки с таким названием уже существует", nil, nil)
		}

		codeExists, err := s.repo.ExistsByCode(ctx, tx, createDTO.Code, 0)
		if err != nil {
			return err
		}
		if codeExists {
			return apperrors.NewHttpError(http.StatusBadRequest, "Тип заявки с таким кодом уже существует", nil, nil)
		}

		createdID, errTx := s.repo.Create(ctx, tx, orderTypeEntity)
		if errTx != nil {
			return errTx
		}
		newID = createdID
		return nil
	})
	if err != nil {
		s.logger.Error("Ошибка при создании типа заявки", zap.Error(err))
		return nil, err
	}

	createdEntity, err := s.repo.FindByID(ctx, newID)
	if err != nil {
		s.logger.Error("Не удалось получить созданный тип заявки по ID", zap.Int("id", newID), zap.Error(err))
		return nil, err
	}

	return toResponseDTO(createdEntity), nil
}

func (s *OrderTypeService) Update(ctx context.Context, id int, updateDTO dto.UpdateOrderTypeDTO) (*dto.OrderTypeResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo("order_type:update", *authContext) {
		return nil, apperrors.ErrForbidden
	}

	existingEntity, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	nameChanged := false
	codeChanged := false
	// Обновляем поля, только если они были переданы в DTO
	if updateDTO.Name != nil {
		existingEntity.Name = *updateDTO.Name
	}
	if updateDTO.Code != nil {
		existingEntity.Code = updateDTO.Code
	}
	if updateDTO.StatusID != nil {
		existingEntity.StatusID = *updateDTO.StatusID
	}
	now := time.Now()
	existingEntity.UpdatedAt = &now

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		if nameChanged {
			nameExists, err := s.repo.ExistsByName(ctx, tx, existingEntity.Name, id) // <-- Передаем `id` для исключения
			if err != nil {
				return err
			}
			if nameExists {
				return apperrors.NewHttpError(http.StatusConflict, fmt.Sprintf("Тип заявки с именем '%s' уже существует.", existingEntity.Name), nil, nil)
			}
		}

		// Проверяем код, ТОЛЬКО ЕСЛИ он изменился
		if codeChanged {
			codeExists, err := s.repo.ExistsByCode(ctx, tx, existingEntity.Code, id) // <-- Передаем `id` для исключения
			if err != nil {
				return err
			}
			if codeExists {
				return apperrors.NewHttpError(http.StatusConflict, fmt.Sprintf("Тип заявки с кодом '%s' уже существует.", *existingEntity.Code), nil, nil)
			}
		}

		return s.repo.Update(ctx, tx, existingEntity)
	})
	if err != nil {
		s.logger.Error("Ошибка при обновлении типа заявки", zap.Int("id", id), zap.Error(err))
		return nil, err
	}

	return toResponseDTO(existingEntity), nil
}

// Delete удаляет тип заявки.
func (s *OrderTypeService) Delete(ctx context.Context, id int) error {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo("order_type:delete", *authContext) {
		return apperrors.ErrForbidden
	}

	// Дополнительная проверка, если нужно: убедиться, что тип заявки не используется в заявках

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, id)
	})
	if err != nil {
		s.logger.Error("Ошибка при удалении типа заявки", zap.Int("id", id), zap.Error(err))
	}

	return err
}

// GetByID находит тип заявки по ID.
func (s *OrderTypeService) GetByID(ctx context.Context, id int) (*dto.OrderTypeResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo("order_type:view", *authContext) {
		return nil, apperrors.ErrForbidden
	}

	entity, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return toResponseDTO(entity), nil
}

// GetAll получает список типов заявок.
func (s *OrderTypeService) GetAll(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.OrderTypeResponseDTO], error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo("order_type:view", *authContext) {
		return nil, apperrors.ErrForbidden
	}

	entities, total, err := s.repo.GetAll(ctx, limit, offset, search)
	if err != nil {
		s.logger.Error("Ошибка при получении списка типов заявок", zap.Error(err))
		return nil, err
	}

	dtos := make([]dto.OrderTypeResponseDTO, 0, len(entities))
	for _, entity := range entities {
		dtos = append(dtos, *toResponseDTO(entity))
	}

	var currentPage uint64 = 1
	if limit > 0 {
		currentPage = (offset / limit) + 1
	}

	return &dto.PaginatedResponse[dto.OrderTypeResponseDTO]{
		List:       dtos,
		Pagination: dto.PaginationObject{TotalCount: total, Page: currentPage, Limit: limit},
	}, nil
}

func (s *OrderTypeService) GetConfig(ctx context.Context, orderTypeID uint64) (map[string]interface{}, error) {
	// Для этой операции не нужна полная транзакция, мы будем использовать read-only
	// но для простоты передадим `nil`, репозиторий сам разберется

	// Обернем вызов в транзакцию для консистентности
	var result *RuleEngineResult
	err := s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		res, errTx := s.ruleEngine.GetPredefinedRoute(ctx, tx, orderTypeID)
		if errTx != nil {
			return errTx
		}
		result = res
		return nil
	})
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			// Если жесткого правила нет, это не ошибка. Просто нет преднастроек.
			return map[string]interface{}{"is_locked": false}, nil
		}
		// Другая, реальная ошибка
		return nil, err
	}

	// Если нашли жесткий маршрут
	return map[string]interface{}{
		"is_locked": true,
		"prefilled_data": map[string]interface{}{
			"department_id": result.DepartmentID,
			"otdel_id":      result.OtdelID,
		},
	}, nil
}

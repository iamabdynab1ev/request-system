// Файл: internal/services/order_routing_rule_service.go
package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type OrderRoutingRuleServiceInterface interface {
	Create(ctx context.Context, d dto.CreateOrderRoutingRuleDTO) (*dto.OrderRoutingRuleResponseDTO, error)
	Update(ctx context.Context, id int, d dto.UpdateOrderRoutingRuleDTO) (*dto.OrderRoutingRuleResponseDTO, error)
	Delete(ctx context.Context, id int) error
	GetByID(ctx context.Context, id int) (*dto.OrderRoutingRuleResponseDTO, error)
	GetAll(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.OrderRoutingRuleResponseDTO], error)
}

type OrderRoutingRuleService struct {
	repo          repositories.OrderRoutingRuleRepositoryInterface
	userRepo      repositories.UserRepositoryInterface
	orderTypeRepo repositories.OrderTypeRepositoryInterface
	txManager     repositories.TxManagerInterface
	logger        *zap.Logger
}

func NewOrderRoutingRuleService(
	r repositories.OrderRoutingRuleRepositoryInterface,
	u repositories.UserRepositoryInterface,
	tm repositories.TxManagerInterface,
	l *zap.Logger,
	otr repositories.OrderTypeRepositoryInterface, // <<< ДОБАВЛЕНО
) OrderRoutingRuleServiceInterface {
	return &OrderRoutingRuleService{
		repo:          r,
		userRepo:      u,
		txManager:     tm,
		logger:        l,
		orderTypeRepo: otr, // <<< ДОБАВЛЕНО
	}
}

func toRuleResponseDTO(e *entities.OrderRoutingRule) *dto.OrderRoutingRuleResponseDTO {
	if e == nil {
		return nil
	}
	return &dto.OrderRoutingRuleResponseDTO{
		ID:           uint64(e.ID),
		RuleName:     e.RuleName,
		OrderTypeID:  e.OrderTypeID,
		DepartmentID: e.DepartmentID,
		OtdelID:      e.OtdelID,
		PositionID:   e.PositionID,
		StatusID:     e.StatusID,
		CreatedAt:    e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    e.UpdatedAt.Format(time.RFC3339),
	}
}

// services/order_routing_rule_service.go

func (s *OrderRoutingRuleService) Create(ctx context.Context, d dto.CreateOrderRoutingRuleDTO) (*dto.OrderRoutingRuleResponseDTO, error) {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo("order_rule:create", *authContext) {
		return nil, apperrors.ErrForbidden
	}

	// ВАЖНО: убедись, что твой OrderTypeID в DTO - это *int.
	// Если это int, то проверка `d.OrderTypeID == nil` не сработает.
	// Для простоты я буду считать, что `OrderTypeID *int` (указатель).
	if d.OrderTypeID == nil {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Поле 'order_type_id' обязательно", nil, nil)
	}

	assignToPosID := &d.PositionID

	rule := &entities.OrderRoutingRule{
		RuleName:     d.RuleName,
		OrderTypeID:  d.OrderTypeID,
		DepartmentID: d.DepartmentID,
		OtdelID:      d.OtdelID,
		PositionID:   assignToPosID,
		StatusID:     d.StatusID,
	}

	var newID int
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		// <<<----- НАЧАЛО НОВОГО БЛОКА ПРОВЕРКИ ----->>>

		// 1. Проверяем, не существует ли уже правило для данного типа заявки.
		exists, err := s.repo.ExistsByOrderTypeID(ctx, tx, *d.OrderTypeID)
		if err != nil {
			s.logger.Error("Ошибка при проверке существования правила", zap.Error(err))
			return err // Возвращаем внутреннюю ошибку, если проверка не удалась
		}
		if exists {
			// 2. Если правило уже есть, возвращаем красивую и понятную ошибку клиенту.
			return apperrors.NewHttpError(
				http.StatusConflict, // 409 Conflict - подходящий статус
				"Для данного типа заявки правило маршрутизации уже существует. Вы можете его отредактировать, но не создавать новое.",
				nil, nil,
			)
		}

		// <<<----- КОНЕЦ НОВОГО БЛОКА ПРОВЕРКИ ----->>>

		// 3. Если всё в порядке, создаем новое правило.
		id, errTx := s.repo.Create(ctx, tx, rule)
		if errTx != nil {
			return errTx
		}
		newID = id
		return nil
	})
	if err != nil {
		// Ошибка уже может быть нашего типа HttpError, просто возвращаем ее.
		var httpErr *apperrors.HttpError
		if errors.As(err, &httpErr) {
			return nil, httpErr
		}
		// Иначе, это внутренняя ошибка.
		s.logger.Error("Не удалось создать правило маршрутизации", zap.Error(err))
		return nil, err
	}

	created, err := s.repo.FindByID(ctx, newID)
	if err != nil {
		return nil, err
	}
	return toRuleResponseDTO(created), nil
}

func (s *OrderRoutingRuleService) Update(ctx context.Context, id int, d dto.UpdateOrderRoutingRuleDTO) (*dto.OrderRoutingRuleResponseDTO, error) {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo("rule:update", *authContext) {
		return nil, apperrors.ErrForbidden
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if d.RuleName != nil {
		existing.RuleName = *d.RuleName
	}
	if d.OrderTypeID != nil {
		existing.OrderTypeID = d.OrderTypeID
	}
	if d.DepartmentID != nil {
		existing.DepartmentID = d.DepartmentID
	}
	if d.OtdelID != nil {
		existing.OtdelID = d.OtdelID
	}
	if d.PositionID != nil {
		existing.PositionID = d.PositionID
	}
	if d.StatusID != nil {
		existing.StatusID = *d.StatusID
	}

	now := time.Now()
	existing.UpdatedAt = &now

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.repo.Update(ctx, tx, existing)
	})
	if err != nil {
		return nil, err
	}

	return toRuleResponseDTO(existing), nil
}

func (s *OrderRoutingRuleService) Delete(ctx context.Context, id int) error {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.OrderRuleDelete, *authContext) {
		return apperrors.ErrForbidden
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, id)
	})
	if err != nil {
		s.logger.Error("Ошибка удаления правила маршрутизации", zap.Int("id", id), zap.Error(err))
	}
	return err
}

func (s *OrderRoutingRuleService) GetByID(ctx context.Context, id int) (*dto.OrderRoutingRuleResponseDTO, error) {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrderRuleView, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	entity, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return toRuleResponseDTO(entity), nil
}

func (s *OrderRoutingRuleService) GetAll(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.OrderRoutingRuleResponseDTO], error) {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrderRuleView, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	entities, total, err := s.repo.GetAll(ctx, limit, offset, search)
	if err != nil {
		return nil, err
	}

	if total == 0 {
		return &dto.PaginatedResponse[dto.OrderRoutingRuleResponseDTO]{
			List:       []dto.OrderRoutingRuleResponseDTO{},
			Pagination: dto.PaginationObject{TotalCount: total},
		}, nil
	}

	dtos := make([]dto.OrderRoutingRuleResponseDTO, 0, len(entities))

	// Собираем все order_type_id (типа int) в map для уникальности
	orderTypeIDsMap := make(map[int]struct{})
	for _, e := range entities {
		if e.OrderTypeID != nil {
			orderTypeIDsMap[*e.OrderTypeID] = struct{}{}
		}
	}

	// `map[order_type_id] -> "CODE"` для быстрого доступа
	typeCodesMap := make(map[uint64]string)
	if len(orderTypeIDsMap) > 0 {
		// Конвертируем map ключей в срез uint64 для запроса в БД
		idsToFetch := make([]uint64, 0, len(orderTypeIDsMap))
		for id := range orderTypeIDsMap {
			idsToFetch = append(idsToFetch, uint64(id))
		}

		// Вызываем новый метод `FindCodesByIDs`
		codesMap, _ := s.orderTypeRepo.FindCodesByIDs(ctx, idsToFetch)
		if codesMap != nil {
			typeCodesMap = codesMap
		}
	}

	// Итерируем по сущностям и обогащаем DTO
	for _, e := range entities {
		dto := toRuleResponseDTO(e)

		var orderTypeCode string
		if e.OrderTypeID != nil {
			// Ищем код в нашей карте, приводя тип `*int` к `uint64`
			orderTypeCode = typeCodesMap[uint64(*e.OrderTypeID)]
		}

		// Ищем код в реестре `ValidationRegistry`, который находится в том же пакете
		validationRules, ok := ValidationRegistry[orderTypeCode]
		if ok {
			requiredFields := make([]string, 0, len(validationRules))
			for _, vr := range validationRules {
				requiredFields = append(requiredFields, vr.FieldName)
			}
			dto.RequiredFields = requiredFields
		} else {
			// Всегда возвращаем пустой срез, а не null
			dto.RequiredFields = []string{}
		}

		dtos = append(dtos, *dto)
	}

	var currentPage uint64 = 1
	if limit > 0 {
		currentPage = (offset / limit) + 1
	}

	return &dto.PaginatedResponse[dto.OrderRoutingRuleResponseDTO]{
		List:       dtos,
		Pagination: dto.PaginationObject{TotalCount: total, Page: currentPage, Limit: limit},
	}, nil
}

package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
)

type OrderRoutingRuleServiceInterface interface {
	Create(ctx context.Context, d dto.CreateOrderRoutingRuleDTO) (*dto.OrderRoutingRuleResponseDTO, error)
	Update(ctx context.Context, id int, d dto.UpdateOrderRoutingRuleDTO, rawBody []byte) (*dto.OrderRoutingRuleResponseDTO, error)
	Delete(ctx context.Context, id int) error
	GetByID(ctx context.Context, id int) (*dto.OrderRoutingRuleResponseDTO, error)
	GetAll(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.OrderRoutingRuleResponseDTO], error)
}

type OrderRoutingRuleService struct {
	repo          repositories.OrderRoutingRuleRepositoryInterface
	userRepo      repositories.UserRepositoryInterface
	positionRepo  repositories.PositionRepositoryInterface // <-- ДОБАВЛЕНО
	orderTypeRepo repositories.OrderTypeRepositoryInterface
	txManager     repositories.TxManagerInterface
	logger        *zap.Logger
}

func NewOrderRoutingRuleService(
	r repositories.OrderRoutingRuleRepositoryInterface,
	u repositories.UserRepositoryInterface,
	p repositories.PositionRepositoryInterface, // <-- ДОБАВЛЕНО
	tm repositories.TxManagerInterface,
	l *zap.Logger,
	otr repositories.OrderTypeRepositoryInterface,
) OrderRoutingRuleServiceInterface {
	return &OrderRoutingRuleService{
		repo:          r,
		userRepo:      u,
		positionRepo:  p, // <-- ДОБАВЛЕНО
		txManager:     tm,
		logger:        l,
		orderTypeRepo: otr,
	}
}

func (s *OrderRoutingRuleService) toResponseDTO(ctx context.Context, entity *entities.OrderRoutingRule) (*dto.OrderRoutingRuleResponseDTO, error) {
	if entity == nil {
		return nil, nil
	}

	response := &dto.OrderRoutingRuleResponseDTO{
		ID:           uint64(entity.ID),
		RuleName:     entity.RuleName,
		OrderTypeID:  entity.OrderTypeID,
		DepartmentID: entity.DepartmentID,
		OtdelID:      entity.OtdelID,
		PositionID:   entity.PositionID,
		StatusID:     entity.StatusID,
		CreatedAt:    entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    entity.UpdatedAt.Format(time.RFC3339),
	}

	if entity.PositionID != nil {
		pos, err := s.positionRepo.FindByID(ctx, nil, uint64(*entity.PositionID))
		if err == nil && pos.Type != nil {
			response.PositionType = *pos.Type
			if name, ok := constants.PositionTypeNames[constants.PositionType(*pos.Type)]; ok {
				response.PositionTypeName = name
			}
		}
	}

	return response, nil
}

func (s *OrderRoutingRuleService) Create(ctx context.Context, d dto.CreateOrderRoutingRuleDTO) (*dto.OrderRoutingRuleResponseDTO, error) {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrderRuleCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	var depID, otdelID *uint64
	if d.DepartmentID != nil {
		v := uint64(*d.DepartmentID)
		depID = &v
	}
	if d.OtdelID != nil {
		v := uint64(*d.OtdelID)
		otdelID = &v
	}

	positions, err := s.positionRepo.FindByTypeAndOrg(ctx, nil, d.PositionType, depID, otdelID)
	if err != nil {
		return nil, err
	}
	if len(positions) == 0 {
		errMsg := fmt.Sprintf("Не найдено ни одной должности с типом '%s'", d.PositionType)
		if depID != nil {
			errMsg += fmt.Sprintf(" в департаменте ID %d", *depID)
		}
		if otdelID != nil {
			errMsg += fmt.Sprintf(" и отделе ID %d", *otdelID)
		}
		return nil, apperrors.NewHttpError(http.StatusBadRequest, errMsg, nil, nil)
	}

	foundPositionID := positions[0].ID
	positionID := int(foundPositionID)

	rule := &entities.OrderRoutingRule{
		RuleName:     d.RuleName,
		OrderTypeID:  d.OrderTypeID,
		DepartmentID: d.DepartmentID,
		OtdelID:      d.OtdelID,
		PositionID:   &positionID,
		StatusID:     d.StatusID,
	}
	var newID int
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		id, errTx := s.repo.Create(ctx, tx, rule)
		if errTx != nil {
			return errTx
		}
		newID = id
		return nil
	})
	if err != nil {
		return nil, err
	}

	created, err := s.repo.FindByID(ctx, newID)
	if err != nil {
		return nil, err
	}

	return s.toResponseDTO(ctx, created)
}

func (s *OrderRoutingRuleService) Update(ctx context.Context, id int, d dto.UpdateOrderRoutingRuleDTO, rawBody []byte) (*dto.OrderRoutingRuleResponseDTO, error) {
	authContext, err := buildAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrderRuleUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Сначала применяем все простые поля через патчер
	if err := utils.ApplyPatchFinal(existing, d, rawBody); err != nil {
		return nil, err
	}

	// Отдельно обрабатываем сложную логику для position_type
	if utils.WasFieldSent("position_type", rawBody) {
		if d.PositionType.Valid && d.PositionType.String != "" {
			var depID, otdelID *uint64
			if existing.DepartmentID != nil {
				v := uint64(*existing.DepartmentID)
				depID = &v
			}
			if existing.OtdelID != nil {
				v := uint64(*existing.OtdelID)
				otdelID = &v
			}

			positions, err := s.positionRepo.FindByTypeAndOrg(ctx, nil, d.PositionType.String, depID, otdelID)
			if err != nil {
				return nil, err
			}
			if len(positions) == 0 {
				return nil, apperrors.NewHttpError(http.StatusBadRequest, "Должность с указанным типом не найдена в рамках оргструктуры правила", nil, nil)
			}

			positionID := int(positions[0].ID)
			existing.PositionID = &positionID
		} else {
			// Если пришел {"position_type": null}
			existing.PositionID = nil
		}
	}

	now := time.Now()
	existing.UpdatedAt = &now

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.repo.Update(ctx, tx, existing)
	})
	if err != nil {
		return nil, err
	}

	updated, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.toResponseDTO(ctx, updated)
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

	return s.toResponseDTO(ctx, entity)
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

	dtos := make([]dto.OrderRoutingRuleResponseDTO, 0, len(entities))
	for _, e := range entities {
		responseDTO, err := s.toResponseDTO(ctx, e)
		if err != nil {
			return nil, err
		}
		dtos = append(dtos, *responseDTO)
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

	return err
}

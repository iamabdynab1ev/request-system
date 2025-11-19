package services

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
)

type OrderContext struct {
	OrderTypeID  uint64
	DepartmentID uint64
	OtdelID      *uint64
}

type RuleEngineResult struct {
	Executor     *entities.User
	DepartmentID uint64
	OtdelID      *uint64
}

type RuleEngineServiceInterface interface {
	ResolveExecutor(ctx context.Context, tx pgx.Tx, orderCtx OrderContext, userSelectedExecutorID *uint64) (*RuleEngineResult, error)
	GetPredefinedRoute(ctx context.Context, tx pgx.Tx, orderTypeID uint64) (*RuleEngineResult, error)
}

type RuleEngineService struct {
	ruleRepo     repositories.OrderRoutingRuleRepositoryInterface
	userRepo     repositories.UserRepositoryInterface
	positionRepo repositories.PositionRepositoryInterface
	logger       *zap.Logger
}

func NewRuleEngineService(
	ruleRepo repositories.OrderRoutingRuleRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	positionRepo repositories.PositionRepositoryInterface,
	logger *zap.Logger,
) RuleEngineServiceInterface {
	return &RuleEngineService{
		ruleRepo:     ruleRepo,
		userRepo:     userRepo,
		positionRepo: positionRepo,
		logger:       logger,
	}
}

func (s *RuleEngineService) ResolveExecutor(ctx context.Context, tx pgx.Tx, orderCtx OrderContext, userSelectedExecutorID *uint64) (*RuleEngineResult, error) {
	s.logger.Debug("Запуск движка правил...", zap.Any("orderCtx", orderCtx), zap.Any("userSelectedExecutorID", userSelectedExecutorID))

	if userSelectedExecutorID != nil {
		s.logger.Info("Исполнитель выбран вручную.")
		executor, err := s.userRepo.FindUserByIDInTx(ctx, tx, *userSelectedExecutorID)
		if err != nil || executor == nil {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Указанный исполнитель не найден", err, nil)
		}
		return &RuleEngineResult{Executor: executor, DepartmentID: orderCtx.DepartmentID, OtdelID: orderCtx.OtdelID}, nil
	}

	if orderCtx.OrderTypeID != 0 {
		rule, err := s.ruleRepo.FindByTypeID(ctx, tx, int(orderCtx.OrderTypeID))
		if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
			s.logger.Error("Ошибка при поиске правила маршрутизации", zap.Error(err))
			return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка при поиске правила маршрутизации", err, nil)
		}
		if rule != nil {
			s.logger.Info("Найдено правило маршрутизации, эскалация 'снизу-вверх'.", zap.String("ruleName", rule.RuleName))
			if rule.PositionID == nil {
				return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации: у правила не указана должность", nil, nil)
			}

			startPosition, err := s.positionRepo.FindByID(ctx, tx, uint64(*rule.PositionID))
			if err != nil || startPosition == nil || startPosition.Type == nil {
				return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Не найдена должность из правила или у нее не указан тип", err, nil)
			}

			startPosType := constants.PositionType(*startPosition.Type)
			deptID := orderCtx.DepartmentID
			if rule.DepartmentID != nil {
				deptID = uint64(*rule.DepartmentID)
			}
			otdelID := orderCtx.OtdelID
			if rule.OtdelID != nil {
				v := uint64(*rule.OtdelID)
				otdelID = &v
			}

			executor, err := s.findExecutorAscending(ctx, tx, deptID, otdelID, startPosType)
			if err != nil {
				return nil, err
			}

			return &RuleEngineResult{Executor: executor, DepartmentID: deptID, OtdelID: otdelID}, nil
		}
	}

	s.logger.Info("Включается ручной режим назначения 'сверху-вниз'.")
	if orderCtx.DepartmentID == 0 {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Необходимо указать департамент", nil, nil)
	}

	executor, err := s.findExecutorDescending(ctx, tx, orderCtx.DepartmentID, orderCtx.OtdelID)
	if err != nil {
		return nil, err
	}

	return &RuleEngineResult{Executor: executor, DepartmentID: orderCtx.DepartmentID, OtdelID: orderCtx.OtdelID}, nil
}

func (s *RuleEngineService) GetPredefinedRoute(ctx context.Context, tx pgx.Tx, orderTypeID uint64) (*RuleEngineResult, error) {
	rule, err := s.ruleRepo.FindByTypeID(ctx, tx, int(orderTypeID))
	if err != nil {
		return nil, err
	}

	result := &RuleEngineResult{}
	if rule.DepartmentID != nil {
		result.DepartmentID = uint64(*rule.DepartmentID)
	}
	if rule.OtdelID != nil {
		v := uint64(*rule.OtdelID)
		result.OtdelID = &v
	}

	return result, nil
}

// Эскалация "СНИЗУ-ВВЕРХ" для правил
func (s *RuleEngineService) findExecutorAscending(ctx context.Context, tx pgx.Tx, departmentID uint64, otdelID *uint64, startPositionType constants.PositionType) (*entities.User, error) {
	hierarchy := constants.GetAscendingHierarchy()
	startIndex := -1
	for i, pt := range hierarchy {
		if pt == startPositionType {
			startIndex = i
			break
		}
	}
	if startIndex == -1 {
		startIndex = 0
	}

	for i := startIndex; i < len(hierarchy); i++ {
		currentPosType := hierarchy[i]
		s.logger.Debug("Эскалация вверх: ищем", zap.String("positionType", string(currentPosType)))
		users, err := s.userRepo.FindActiveUsersByPositionTypeAndOrg(ctx, tx, string(currentPosType), &departmentID, otdelID)
		if err != nil {
			return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка при поиске пользователей", err, nil)
		}
		if len(users) > 0 {
			s.logger.Info("Исполнитель найден при эскалации вверх", zap.Uint64("userID", users[0].ID))
			return &users[0], nil
		}
	}
	return nil, apperrors.NewHttpError(http.StatusNotFound, "Не удалось найти исполнителя по правилу после полной эскалации 'вверх'", nil, nil)
}

// Эскалация "СВЕРХУ-ВНИЗ" для ручного режима
func (s *RuleEngineService) findExecutorDescending(ctx context.Context, tx pgx.Tx, departmentID uint64, otdelID *uint64) (*entities.User, error) {
	hierarchy := constants.GetDescendingHierarchy()

	startIndex := 0
	if otdelID != nil {
		for i, pt := range hierarchy {
			if pt == constants.PositionTypeHeadOfOtdel {
				startIndex = i
				break
			}
		}
	}

	for i := startIndex; i < len(hierarchy); i++ {
		currentPosType := hierarchy[i]
		s.logger.Debug("Эскалация вниз: ищем", zap.String("positionType", string(currentPosType)))
		users, err := s.userRepo.FindActiveUsersByPositionTypeAndOrg(ctx, tx, string(currentPosType), &departmentID, otdelID)
		if err != nil {
			return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка при поиске пользователей", err, nil)
		}
		if len(users) > 0 {
			s.logger.Info("Исполнитель найден при эскалации вниз", zap.Uint64("userID", users[0].ID))
			return &users[0], nil
		}
	}
	return nil, apperrors.NewHttpError(http.StatusNotFound, "Не удалось найти исполнителя после полной эскалации 'вниз'", nil, nil)
}

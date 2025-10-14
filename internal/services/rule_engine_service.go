// Файл: internal/services/rule_engine_service.go
package services

import (
	"context"
	"errors"
	"net/http"

	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
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
	return &RuleEngineService{ruleRepo: ruleRepo, userRepo: userRepo, positionRepo: positionRepo, logger: logger}
}

func (s *RuleEngineService) ResolveExecutor(ctx context.Context, tx pgx.Tx, orderCtx OrderContext, userSelectedExecutorID *uint64) (*RuleEngineResult, error) {
	s.logger.Debug("Запуск движка...", zap.Uint64("orderTypeID", orderCtx.OrderTypeID))

	rule, err := s.ruleRepo.FindByTypeID(ctx, tx, int(orderCtx.OrderTypeID))

	if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
		return nil, err
	}

	if rule != nil {
		s.logger.Debug("Найдено жесткое правило.", zap.String("ruleName", rule.RuleName))
		if rule.PositionID == nil {
			return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации: у правила нет должности", nil, nil)
		}

		resultCtx := &RuleEngineResult{}

		if rule.DepartmentID != nil {
			resultCtx.DepartmentID = uint64(*rule.DepartmentID)
		}
		if rule.OtdelID != nil {
			v := uint64(*rule.OtdelID)
			resultCtx.OtdelID = &v
		}

		executor, err := s.findAvailableExecutor(ctx, tx, resultCtx.DepartmentID, resultCtx.OtdelID, uint64(*rule.PositionID))
		if err != nil {
			return nil, err
		}
		resultCtx.Executor = executor

		return resultCtx, nil
	}

	s.logger.Debug("Правило не найдено. Ручной режим.")

	resultCtx := &RuleEngineResult{DepartmentID: orderCtx.DepartmentID, OtdelID: orderCtx.OtdelID}

	s.logger.Debug("Ищем руководителя.", zap.Uint64("departmentID", orderCtx.DepartmentID))

	headPos, err := s.userRepo.FindHighestPositionInDepartment(ctx, tx, orderCtx.DepartmentID)
	if err != nil {
		return nil, err
	}

	startPositionID := uint64(headPos.Id)

	executor, err := s.findAvailableExecutor(ctx, tx, orderCtx.DepartmentID, orderCtx.OtdelID, startPositionID)
	if err != nil {
		return nil, err
	}

	resultCtx.Executor = executor

	return resultCtx, nil
}

func (s *RuleEngineService) findAvailableExecutor(ctx context.Context, tx pgx.Tx, departmentID uint64, otdelID *uint64, startPositionID uint64) (*entities.User, error) {
	var searchChain []string

	if otdelID != nil {
		searchChain = []string{"DEPARTMENT_HEAD", "DEPARTMENT_VICE_HEAD", "OTDEL_HEAD"}
	} else {
		searchChain = []string{"DEPARTMENT_HEAD", "DEPARTMENT_VICE_HEAD"}
	}

	for _, positionCode := range searchChain {

		users, err := s.userRepo.FindActiveUsersByPositionCode(ctx, tx, positionCode, departmentID, otdelID)
		if err != nil {
			return nil, err
		}

		if len(users) > 0 {
			foundUser := users[0]
			s.logger.Info("Найден доступный исполнитель!", zap.String("fio", foundUser.Fio), zap.String("positionCode", positionCode))
			return &foundUser, nil
		}

		s.logger.Debug("На должностях с кодом "+positionCode+" не найдено активных исполнителей", zap.String("code", positionCode))
	}

	s.logger.Error("Ни один руководитель/менеджер в цепочке эскалации не доступен", zap.Uint64("departmentID", departmentID))
	return nil, apperrors.NewHttpError(http.StatusConflict, "В данном подразделении нет доступного руководителя для назначения заявки.", nil, nil)
}

func (s *RuleEngineService) GetPredefinedRoute(ctx context.Context, tx pgx.Tx, orderTypeID uint64) (*RuleEngineResult, error) {
	s.logger.Debug("Поиск предопределенного маршрута", zap.Uint64("orderTypeID", orderTypeID))

	rule, err := s.ruleRepo.FindByTypeID(ctx, tx, int(orderTypeID))
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	if rule.DepartmentID != nil && rule.PositionID != nil {
		result := &RuleEngineResult{DepartmentID: uint64(*rule.DepartmentID)}
		if rule.OtdelID != nil {
			v := uint64(*rule.OtdelID)
			result.OtdelID = &v
		}
		return result, nil
	}

	return nil, apperrors.ErrNotFound
}

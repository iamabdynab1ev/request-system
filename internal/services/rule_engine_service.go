package services

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
)

// OrderContext определяет контекст заявки для движка правил
type OrderContext struct {
	OrderTypeID  uint64
	DepartmentID uint64
	OtdelID      *uint64
}

// RuleEngineResult возвращает результат работы движка правил
type RuleEngineResult struct {
	Executor     *entities.User
	DepartmentID uint64
	OtdelID      *uint64
}

// RuleEngineServiceInterface определяет интерфейс движка правил
type RuleEngineServiceInterface interface {
	ResolveExecutor(ctx context.Context, tx pgx.Tx, orderCtx OrderContext, userSelectedExecutorID *uint64) (*RuleEngineResult, error)
	GetPredefinedRoute(ctx context.Context, tx pgx.Tx, orderTypeID uint64) (*RuleEngineResult, error)
}

// RuleEngineService реализует логику назначения исполнителей
type RuleEngineService struct {
	ruleRepo     repositories.OrderRoutingRuleRepositoryInterface
	userRepo     repositories.UserRepositoryInterface
	positionRepo repositories.PositionRepositoryInterface
	logger       *zap.Logger
}

// NewRuleEngineService создает новый экземпляр RuleEngineService
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

func (s *RuleEngineService) MapPositionToType(position *entities.Position) constants.PositionType {
	if position == nil || position.Name == "" {
		return constants.PositionTypeSpecialist
	}
	for posType, name := range constants.PositionTypeNames {
		if position.Name == name {
			return posType
		}
	}
	return constants.PositionTypeSpecialist
}

func (s *RuleEngineService) ResolveExecutor(ctx context.Context, tx pgx.Tx, orderCtx OrderContext, userSelectedExecutorID *uint64) (*RuleEngineResult, error) {
	s.logger.Debug("Запуск движка правил для определения исполнителя",
		zap.Any("orderCtx", orderCtx),
		zap.Any("userSelectedExecutorID", userSelectedExecutorID))

	// Если указан userSelectedExecutorID, проверяем его в первую очередь
	if userSelectedExecutorID != nil {
		executor, err := s.userRepo.FindUserByIDInTx(ctx, tx, *userSelectedExecutorID)
		if err != nil {
			s.logger.Error("Пользователь с указанным ID не найден",
				zap.Uint64("userSelectedExecutorID", *userSelectedExecutorID),
				zap.Error(err))
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Указанный исполнитель не найден", err, nil)
		}
		if executor == nil {
			s.logger.Error("Пользователь с указанным ID равен nil",
				zap.Uint64("userSelectedExecutorID", *userSelectedExecutorID))
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Указанный исполнитель не найден", nil, nil)
		}
		s.logger.Info("Исполнитель выбран вручную",
			zap.Uint64("executorID", executor.ID),
			zap.String("fio", executor.Fio))
		return &RuleEngineResult{
			Executor:     executor,
			DepartmentID: orderCtx.DepartmentID,
			OtdelID:      orderCtx.OtdelID,
		}, nil
	}

	// Поиск правила маршрутизации
	rule, err := s.ruleRepo.FindByTypeID(ctx, tx, int(orderCtx.OrderTypeID))
	if err != nil {
		s.logger.Error("Ошибка при поиске правила маршрутизации",
			zap.Int("orderTypeID", int(orderCtx.OrderTypeID)),
			zap.Error(err))
		return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка при поиске правила маршрутизации", err, nil)
	}
	if rule == nil {
		s.logger.Warn("Правило маршрутизации не найдено, и не указан userSelectedExecutorID",
			zap.Int("orderTypeID", int(orderCtx.OrderTypeID)))
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Не указан исполнитель, и правило маршрутизации не найдено", nil, nil)
	}

	s.logger.Info("Найдено правило маршрутизации",
		zap.String("ruleName", rule.RuleName),
		zap.Any("rule", rule))

	// Проверка, указана ли должность в правиле
	if rule.PositionID == nil {
		s.logger.Error("Ошибка конфигурации: у правила не указана должность",
			zap.Int("orderTypeID", int(orderCtx.OrderTypeID)))
		return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации: у правила не указана должность", nil, nil)
	}

	// Формируем контекст для поиска исполнителя
	ruleCtx := orderCtx
	if rule.DepartmentID != nil {
		ruleCtx.DepartmentID = uint64(*rule.DepartmentID)
	}
	if rule.OtdelID != nil {
		otdelID := uint64(*rule.OtdelID)
		ruleCtx.OtdelID = &otdelID
	} else {
		ruleCtx.OtdelID = nil
	}

	// Поиск должности из правила
	startPosition, err := s.positionRepo.FindByID(ctx, tx, uint64(*rule.PositionID))
	if err != nil {
		s.logger.Error("Должность из правила не найдена",
			zap.Uint64("positionID", uint64(*rule.PositionID)),
			zap.Error(err))
		return nil, apperrors.NewHttpError(http.StatusNotFound, "Должность из правила не найдена", err, nil)
	}

	// Map position to PositionType
	if startPosition.Type == nil || *startPosition.Type == "" {
		s.logger.Error("Ошибка конфигурации: у должности из правила не указан системный тип",
			zap.Uint64("positionID", startPosition.ID))
		return nil, apperrors.NewHttpError(http.StatusInternalServerError, "У должности в правиле не указан системный тип", nil, nil)
	}

	// КОММЕНТАРИЙ: ПРАВИЛЬНО ОПРЕДЕЛЯЕМ СТАРТОВУЮ ТОЧКУ.
	// Берем системный тип прямо из должности, указанной в правиле.
	startPositionType := constants.PositionType(*startPosition.Type)
	s.logger.Info("Определен начальный тип должности для поиска", zap.String("positionType", string(startPositionType)))

	// КОММЕНТАРИЙ: ПЕРЕДАЕМ НАЙДЕННЫЙ ТИП В ФУНКЦИЮ ЭСКАЛАЦИИ.
	executor, err := s.findExecutorWithTypeEscalation(ctx, tx, &ruleCtx.DepartmentID, ruleCtx.OtdelID, startPositionType)
	if err != nil {
		s.logger.Error("Не удалось найти исполнителя по правилу",
			zap.Error(err),
			zap.Uint64("departmentID", ruleCtx.DepartmentID),
			zap.Any("otdelID", ruleCtx.OtdelID),
			zap.String("startPositionType", string(startPositionType)))
		return nil, err
	}

	if executor == nil {
		s.logger.Error("Не удалось найти исполнителя по правилу",
			zap.Uint64("departmentID", ruleCtx.DepartmentID),
			zap.Any("otdelID", ruleCtx.OtdelID),
			zap.String("startPositionType", string(startPositionType)))
		return nil, apperrors.NewHttpError(http.StatusNotFound, "Не удалось найти исполнителя по правилу", nil, nil)
	}

	s.logger.Info("Исполнитель найден по правилу",
		zap.Uint64("executorID", executor.ID),
		zap.String("fio", executor.Fio),
		zap.Uint64("departmentID", ruleCtx.DepartmentID),
		zap.Any("otdelID", ruleCtx.OtdelID))

	return &RuleEngineResult{
		Executor:     executor,
		DepartmentID: ruleCtx.DepartmentID,
		OtdelID:      ruleCtx.OtdelID,
	}, nil
}

// GetPredefinedRoute возвращает предопределенный маршрут для типа заявки
func (s *RuleEngineService) GetPredefinedRoute(ctx context.Context, tx pgx.Tx, orderTypeID uint64) (*RuleEngineResult, error) {
	s.logger.Debug("Поиск предопределенного маршрута",
		zap.Uint64("orderTypeID", orderTypeID))

	// Поиск правила маршрутизации
	rule, err := s.ruleRepo.FindByTypeID(ctx, tx, int(orderTypeID))
	if err != nil {
		s.logger.Error("Ошибка при поиске правила маршрутизации",
			zap.Uint64("orderTypeID", orderTypeID),
			zap.Error(err))
		return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка при поиске правила маршрутизации", err, nil)
	}
	if rule == nil {
		s.logger.Warn("Правило маршрутизации не найдено",
			zap.Uint64("orderTypeID", orderTypeID))
		return nil, apperrors.NewHttpError(http.StatusNotFound, "Правило маршрутизации не найдено", nil, nil)
	}

	s.logger.Info("Найдено правило маршрутизации",
		zap.String("ruleName", rule.RuleName),
		zap.Any("rule", rule))

	// Проверка, указана ли должность
	if rule.PositionID == nil {
		s.logger.Error("Ошибка конфигурации: у правила не указана должность",
			zap.Uint64("orderTypeID", orderTypeID))
		return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации: у правила не указана должность", nil, nil)
	}

	// Формируем контекст
	routeCtx := OrderContext{
		OrderTypeID:  orderTypeID,
		DepartmentID: 0,
		OtdelID:      nil,
	}
	if rule.DepartmentID != nil {
		routeCtx.DepartmentID = uint64(*rule.DepartmentID)
	}
	if rule.OtdelID != nil {
		otdelID := uint64(*rule.OtdelID)
		routeCtx.OtdelID = &otdelID
	}

	// Поиск должности
	startPosition, err := s.positionRepo.FindByID(ctx, tx, uint64(*rule.PositionID))
	if err != nil {
		s.logger.Error("Должность из правила не найдена",
			zap.Uint64("positionID", uint64(*rule.PositionID)),
			zap.Error(err))
		return nil, apperrors.NewHttpError(http.StatusNotFound, "Должность из правила не найдена", err, nil)
	}

	// Map position to PositionType
	positionType := s.MapPositionToType(startPosition)

	// Поиск исполнителя
	departmentIDPtr := &routeCtx.DepartmentID
	executor, err := s.findExecutorWithTypeEscalation(ctx, tx, departmentIDPtr, routeCtx.OtdelID, positionType)
	if err != nil {
		s.logger.Error("Не удалось найти исполнителя для предопределенного маршрута",
			zap.Error(err),
			zap.Uint64("departmentID", routeCtx.DepartmentID),
			zap.Any("otdelID", routeCtx.OtdelID),
			zap.String("positionType", string(positionType)))
		return nil, err
	}
	if executor == nil {
		s.logger.Error("Исполнитель не найден для предопределенного маршрута",
			zap.Uint64("departmentID", routeCtx.DepartmentID),
			zap.Any("otdelID", routeCtx.OtdelID),
			zap.String("positionType", string(positionType)))
		return nil, apperrors.NewHttpError(http.StatusNotFound, "Исполнитель не найден для предопределенного маршрута", nil, nil)
	}

	s.logger.Info("Предопределенный маршрут найден",
		zap.Uint64("executorID", executor.ID),
		zap.String("fio", executor.Fio),
		zap.Uint64("departmentID", routeCtx.DepartmentID),
		zap.Any("otdelID", routeCtx.OtdelID))

	return &RuleEngineResult{
		Executor:     executor,
		DepartmentID: routeCtx.DepartmentID,
		OtdelID:      routeCtx.OtdelID,
	}, nil
}

// findExecutorWithTypeEscalation ищет исполнителя с учетом эскалации по типу должности
func (s *RuleEngineService) findExecutorWithTypeEscalation(ctx context.Context, tx pgx.Tx, departmentID *uint64, otdelID *uint64, startPositionType constants.PositionType) (*entities.User, error) {
	s.logger.Debug("Запуск поиска исполнителя с эскалацией",
		zap.String("startPositionType", string(startPositionType)),
		zap.Any("departmentID", departmentID),
		zap.Any("otdelID", otdelID))

	// Находим индекс стартовой должности в иерархии "снизу-вверх"
	startIndex := -1
	for i, pt := range constants.EscalationHierarchy {
		if pt == startPositionType {
			startIndex = i
			break
		}
	}

	// Если стартовый тип не найден в иерархии, это ошибка конфигурации,
	// но мы можем подстраховаться, начав с самого низа.
	if startIndex == -1 {
		s.logger.Warn("Стартовый тип должности не найден в иерархии эскалации. Начинаем поиск с самого низа.",
			zap.String("positionType", string(startPositionType)))
		startIndex = 0
	}

	// Идем ВВЕРХ по иерархии, начиная со стартовой должности.
	for i := startIndex; i < len(constants.EscalationHierarchy); i++ {
		currentPositionType := constants.EscalationHierarchy[i]
		s.logger.Info("Эскалация: ищем пользователя с типом должности",
			zap.String("positionType", string(currentPositionType)))

		users, err := s.userRepo.FindActiveUsersByPositionTypeAndOrg(ctx, tx, string(currentPositionType), departmentID, otdelID)
		if err != nil {
			s.logger.Error("Ошибка при поиске пользователей на шаге эскалации", zap.Error(err))
			// Важно не прерывать эскалацию, если произошла ошибка в БД, а вернуть ее
			return nil, err
		}

		// Если на этом уровне НАШЛИ хотя бы одного пользователя - СРАЗУ ВОЗВРАЩАЕМ ЕГО.
		if len(users) > 0 {
			s.logger.Info("Исполнитель найден!",
				zap.Uint64("userID", users[0].ID),
				zap.String("fio", users[0].Fio),
				zap.String("positionType", string(currentPositionType)))
			return &users[0], nil
		}
		// Если не нашли, цикл просто перейдет к следующей, более высокой должности.
	}

	// Если мы прошли весь цикл и никого не нашли.
	s.logger.Error("Исполнитель не найден после полной эскалации",
		zap.String("startPositionType", string(startPositionType)))
	return nil, apperrors.NewHttpError(http.StatusNotFound, "Не удалось найти исполнителя после эскалации", nil, nil)
}

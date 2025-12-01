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

// OrderContext описывает входные данные заявки
type OrderContext struct {
	OrderTypeID  uint64
	DepartmentID uint64
	OtdelID      *uint64
	BranchID     *uint64
	OfficeID     *uint64
}

// RuleEngineResult результат (кого назначили и куда привязали)
type RuleEngineResult struct {
	Executor     *entities.User
	DepartmentID uint64
	OtdelID      *uint64
	BranchID     *uint64
	OfficeID     *uint64
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
	s.logger.Debug("Запуск движка правил...",
		zap.Uint64("dept_id", orderCtx.DepartmentID),
		zap.Any("branch_id", orderCtx.BranchID),
	)

	// 1. РУЧНОЙ ВЫБОР (Если юзер выбрал исполнителя явно)
	if userSelectedExecutorID != nil {
		s.logger.Info("Исполнитель выбран вручную.")
		executor, err := s.userRepo.FindUserByIDInTx(ctx, tx, *userSelectedExecutorID)
		if err != nil || executor == nil {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Указанный исполнитель не найден", err, nil)
		}
		// Возвращаем контекст, привязывая к данным исполнителя, если в заявке было пусто
		deptID, otdelID := orderCtx.DepartmentID, orderCtx.OtdelID
		if deptID == 0 && executor.DepartmentID != nil {
			deptID = *executor.DepartmentID
			otdelID = executor.OtdelID
		}
		branchID, officeID := orderCtx.BranchID, orderCtx.OfficeID
		if branchID == nil && executor.BranchID != nil {
			branchID = executor.BranchID
			officeID = executor.OfficeID
		}

		return &RuleEngineResult{Executor: executor, DepartmentID: deptID, OtdelID: otdelID, BranchID: branchID, OfficeID: officeID}, nil
	}

	// 2. ПРАВИЛА ТИПОВ ЗАЯВОК (Глобальная настройка)
	if orderCtx.OrderTypeID != 0 {
		rule, err := s.ruleRepo.FindByTypeID(ctx, tx, int(orderCtx.OrderTypeID))
		if err != nil && !errors.Is(err, apperrors.ErrNotFound) {
			return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка проверки правил", err, nil)
		}
		if rule != nil {
			s.logger.Info("Найдено спец. правило маршрутизации.")
			// ... (логика правил осталась прежней, она специфична и настроена админом) ...
			if rule.PositionID == nil {
				return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Правило без должности", nil, nil)
			}
			startPos, err := s.positionRepo.FindByID(ctx, tx, uint64(*rule.PositionID))
			if err != nil || startPos == nil {
				return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Должность правила не найдена", err, nil)
			}
			targetDept := uint64(0)
			if rule.DepartmentID != nil {
				targetDept = uint64(*rule.DepartmentID)
			}
			var targetOtdel *uint64
			if rule.OtdelID != nil {
				v := uint64(*rule.OtdelID)
				targetOtdel = &v
			}

			executor, err := s.findExecutorAscending(ctx, tx, targetDept, targetOtdel, constants.PositionType(*startPos.Type))
			if err != nil {
				return nil, err
			}

			return &RuleEngineResult{
				Executor: executor, DepartmentID: targetDept, OtdelID: targetOtdel,
				BranchID: orderCtx.BranchID, OfficeID: orderCtx.OfficeID,
			}, nil
		}
	}

	// 3. АВТО-НАЗНАЧЕНИЕ: СТРОГАЯ ЛОГИКА ПРИОРИТЕТОВ
	s.logger.Info("Правило не найдено. Запуск поиска руководителя по иерархии.")

	// ================================================================
	// ПРИОРИТЕТ 1: ДЕПАРТАМЕНТ
	// Если Департамент указан - работаем ТОЛЬКО по нему.
	// ================================================================
	if orderCtx.DepartmentID > 0 {
		s.logger.Info(">>> Приоритет: Департамент. Ищем руководителя вертикали.")

		executor, err := s.findExecutorDescending(ctx, tx, orderCtx.DepartmentID, orderCtx.OtdelID)
		if err != nil {
			s.logger.Error("Ошибка поиска в Департаменте", zap.Error(err))
			// Если не нашли здесь - это ОШИБКА (никаких переходов на филиал)
			return nil, apperrors.NewHttpError(http.StatusNotFound, "Руководитель Департамента/Отдела не найден", nil, nil)
		}

		return &RuleEngineResult{
			Executor:     executor,
			DepartmentID: orderCtx.DepartmentID,
			OtdelID:      orderCtx.OtdelID,
			BranchID:     orderCtx.BranchID,
			OfficeID:     orderCtx.OfficeID,
		}, nil
	}

	// ================================================================
	// ПРИОРИТЕТ 2: ФИЛИАЛ
	// Если Департамента нет (ID=0), но есть Филиал - работаем по нему.
	// ================================================================
	if orderCtx.BranchID != nil {
		s.logger.Info(">>> Приоритет: Филиал. Ищем руководство.")

		executor, err := s.findBranchDirector(ctx, tx, *orderCtx.BranchID, orderCtx.OfficeID)
		if err != nil {
			s.logger.Error("Ошибка поиска в Филиале", zap.Error(err))
			return nil, apperrors.NewHttpError(http.StatusNotFound, "Директор Филиала или Начальник Офиса не найдены", nil, nil)
		}

		return &RuleEngineResult{
			Executor:     executor,
			DepartmentID: 0,
			OtdelID:      nil,
			BranchID:     executor.BranchID,
			OfficeID:     executor.OfficeID,
		}, nil
	}

	// ================================================================
	// ОШИБКА: Не указан ни Департамент, ни Филиал
	// ================================================================
	return nil, apperrors.NewHttpError(http.StatusBadRequest, "Не определен маршрут (нет Департамента и нет Филиала)", nil, nil)
}

func (s *RuleEngineService) GetPredefinedRoute(ctx context.Context, tx pgx.Tx, orderTypeID uint64) (*RuleEngineResult, error) {
	rule, err := s.ruleRepo.FindByTypeID(ctx, tx, int(orderTypeID))
	if err != nil {
		return nil, err
	}
	res := &RuleEngineResult{}
	if rule.DepartmentID != nil {
		res.DepartmentID = uint64(*rule.DepartmentID)
	}
	if rule.OtdelID != nil {
		v := uint64(*rule.OtdelID)
		res.OtdelID = &v
	}
	return res, nil
}

// ------------------- ЛОГИКА ПОИСКА -------------------

// 1. Поиск в ДЕПАРТАМЕНТЕ (Рук. Деп -> Рук. Отдел -> ...)
func (s *RuleEngineService) findExecutorDescending(ctx context.Context, tx pgx.Tx, departmentID uint64, otdelID *uint64) (*entities.User, error) {
	// [HEAD_OF_DEPARTMENT, HEAD_OF_OTDEL, ...]
	hierarchy := constants.GetDescendingHierarchy()

	// Если указан отдел, начинаем с HEAD_OF_OTDEL (индекс смещаем)
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
		posType := hierarchy[i]
		s.logger.Debug("Поиск в Департаменте:", zap.String("role", string(posType)))

		// ВАЖНО: Твой обновленный метод в user_repository должен возвращать (User, Status),
		// тут мы вызываем старый метод `FindActiveUsersByPositionTypeAndOrg` или подобный.
		// Убедись, что он работает так же надежно, как мы сделали для филиала (с UPPER кодом статуса).
		users, err := s.userRepo.FindActiveUsersByPositionTypeAndOrg(ctx, tx, string(posType), &departmentID, otdelID)
		if err != nil {
			return nil, err
		}
		if len(users) > 0 {
			return &users[0], nil
		}
	}
	return nil, errors.New("hierarchy exhausted")
}

// 2. Поиск в ФИЛИАЛЕ (Директор -> Зам -> Рук.Офис -> ...)
func (s *RuleEngineService) findBranchDirector(ctx context.Context, tx pgx.Tx, branchID uint64, officeID *uint64) (*entities.User, error) {
	// [BRANCH_DIRECTOR, DEPUTY_BRANCH, HEAD_OF_OFFICE, DEPUTY_OFFICE, ...]
	hierarchy := constants.GetDescendingBranchHierarchy()

	// Начинаем с САМОГО ВЕРХА всегда (Директор).
	for _, posType := range hierarchy {

		var searchOfficeID *uint64

		// Логика: "Кто есть кто?"
		isBigBoss := (posType == constants.PositionTypeBranchDirector || posType == constants.PositionTypeDeputyBranchDirector)

		if isBigBoss {
			// Директоров ищем по ВСЕМУ филиалу (игнорируем office_id = 136 и т.д., ищем с nil)
			searchOfficeID = nil
		} else {
			// Менеджеров и Нач.Офиса ищем ТОЛЬКО в конкретном офисе
			if officeID == nil {
				// Офис не задан, а должность офисная -> пропускаем уровень
				continue
			}
			searchOfficeID = officeID
		}

		s.logger.Debug("Поиск в Филиале:",
			zap.String("role", string(posType)),
			zap.Any("in_office_id", searchOfficeID))

		// Вызываем наш (уже исправленный) репозиторий
		users, err := s.userRepo.FindActiveUsersByBranch(ctx, tx, string(posType), branchID, searchOfficeID)
		if err != nil {
			// Логируем, но продолжаем (или возвращаем ошибку)
			return nil, err
		}

		if len(users) > 0 {
			s.logger.Info("Исполнитель найден!", zap.String("role", string(posType)), zap.String("fio", users[0].Fio))
			return &users[0], nil
		}
	}

	return nil, errors.New("branch hierarchy exhausted")
}

// 3. Для правил (не меняется)
func (s *RuleEngineService) findExecutorAscending(ctx context.Context, tx pgx.Tx, departmentID uint64, otdelID *uint64, startPosType constants.PositionType) (*entities.User, error) {
	hierarchy := constants.GetAscendingHierarchy()
	startIndex := -1
	for i, pt := range hierarchy {
		if pt == startPosType {
			startIndex = i
			break
		}
	}
	if startIndex == -1 {
		startIndex = 0
	}

	for i := startIndex; i < len(hierarchy); i++ {
		posType := hierarchy[i]
		users, err := s.userRepo.FindActiveUsersByPositionTypeAndOrg(ctx, tx, string(posType), &departmentID, otdelID)
		if err != nil {
			return nil, err
		}
		if len(users) > 0 {
			return &users[0], nil
		}
	}
	return nil, errors.New("rule hierarchy exhausted")
}

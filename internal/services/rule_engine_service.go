package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// OrderContext
type OrderContext struct {
	OrderTypeID  uint64
	DepartmentID uint64
	OtdelID      *uint64
	BranchID     *uint64
	OfficeID     *uint64
}

// RoutingResult
type RoutingResult struct {
	Executor  entities.User
	StatusID  int
	RuleFound bool

	// Для конфига
	DepartmentID *int
	OtdelID      *int
}

type RuleEngineServiceInterface interface {
	ResolveExecutor(ctx context.Context, tx pgx.Tx, orderCtx OrderContext, explicitExecutorID *uint64) (*RoutingResult, error)
	GetPredefinedRoute(ctx context.Context, tx pgx.Tx, orderTypeID uint64) (*RoutingResult, error)
}

type RuleEngineService struct {
	repo     repositories.OrderRoutingRuleRepositoryInterface
	userRepo repositories.UserRepositoryInterface
	logger   *zap.Logger
}

func NewRuleEngineService(
	repo repositories.OrderRoutingRuleRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) RuleEngineServiceInterface {
	return &RuleEngineService{
		repo:     repo,
		userRepo: userRepo,
		logger:   logger,
	}
}

// ResolveExecutor - Точка входа для поиска исполнителя
func (s *RuleEngineService) ResolveExecutor(ctx context.Context, tx pgx.Tx, orderCtx OrderContext, explicitExecutorID *uint64) (*RoutingResult, error) {
	// 1. Явный исполнитель (если выбрали вручную при создании)
	if explicitExecutorID != nil {
		user, err := s.userRepo.FindUserByIDInTx(ctx, tx, *explicitExecutorID)
		if err != nil {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Указанный исполнитель не найден", err, nil)
		}
		return &RoutingResult{
			Executor:  *user,
			StatusID:  0,
			RuleFound: false,
		}, nil
	}

	// 2. Попытка найти ПРАВИЛО в таблице order_routing_rules (Специальные маршруты)
	query := `
		SELECT assign_to_position_id, status_id
		FROM order_routing_rules
		WHERE 
			(order_type_id IS NULL OR order_type_id = $1)
			AND (department_id IS NULL OR department_id = $2)
			AND (otdel_id IS NULL OR otdel_id = $3)
			AND (branch_id IS NULL OR branch_id = $4)
			AND (office_id IS NULL OR office_id = $5)
		ORDER BY 
			order_type_id NULLS LAST, 
			otdel_id NULLS LAST,
			office_id NULLS LAST,
			department_id NULLS LAST, 
			branch_id NULLS LAST
		LIMIT 1
	`

	var targetPositionID *int
	var targetStatusID int

	err := tx.QueryRow(ctx, query,
		orderCtx.OrderTypeID,
		orderCtx.DepartmentID,
		orderCtx.OtdelID,
		orderCtx.BranchID,
		orderCtx.OfficeID,
	).Scan(&targetPositionID, &targetStatusID)

	// ================= [ЛОГИКА ФОЛБЕКА / ВОДОПАДА] =================
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.logger.Info("Правило не найдено. Запуск поиска по иерархии (Priority Waterfall)", zap.Uint64("type", orderCtx.OrderTypeID))

			// Если правило не найдено — ищем начальника автоматически по жесткой иерархии
			return s.resolveByHierarchy(ctx, tx, orderCtx)
		}
		return nil, fmt.Errorf("ошибка SQL: %w", err)
	}
	// ================================================================

	if targetPositionID == nil {
		return nil, apperrors.NewHttpError(http.StatusInternalServerError, "В правиле маршрутизации не указана должность", nil, nil)
	}

	// 3. Если правило найдено — ищем человека с этой должностью
	foundUser, err := s.findUserByPositionAndStructure(ctx, tx, *targetPositionID, orderCtx)
	if err != nil {
		return nil, err
	}

	return &RoutingResult{
		Executor:  *foundUser,
		StatusID:  targetStatusID,
		RuleFound: true,
	}, nil
}

// resolveByHierarchy - находит руководителя автоматически
// ПРИОРИТЕТ: Department -> Otdel -> Branch -> Office
func (s *RuleEngineService) resolveByHierarchy(ctx context.Context, tx pgx.Tx, d OrderContext) (*RoutingResult, error) {
	var targetPosType string
	
	// Базовый SQL: ищем юзера, который активен (через join со статусом) и имеет нужный тип должности
	query := `
		SELECT u.id, u.fio, u.email, u.position_id, u.department_id, u.branch_id
		FROM users u
		JOIN positions p ON u.position_id = p.id
		JOIN statuses s ON u.status_id = s.id
		WHERE u.deleted_at IS NULL 
		  AND UPPER(s.code) = 'ACTIVE' 
		  AND p.type = $1 
	`
	args := []interface{}{}

	// === ВОДОПАД (WATERFALL) ===
	
	// 1. ВЫСОКИЙ ПРИОРИТЕТ: Если есть Департамент -> ищем Директора Департамента
	// Мы специально ИГНОРИРУЕМ otdel_id, даже если он передан, чтобы найти "Главного" в этом департаменте.
	if d.DepartmentID != 0 {
		targetPosType = "HEAD_OF_DEPARTMENT" 
		query += " AND u.department_id = $2 "
		args = append(args, targetPosType, d.DepartmentID)

	} else if d.OtdelID != nil {
		// 2. СРЕДНИЙ ПРИОРИТЕТ: Если нет Депа, но есть Отдел -> ищем Начальника Отдела
		targetPosType = "MANAGER_OF_OTDEL"
		query += " AND u.otdel_id = $2 "
		args = append(args, targetPosType, *d.OtdelID)
		
		// Часто отделы дублируются в разных филиалах (например "Бухгалтерия" в Душанбе и Худжанде).
		// Поэтому если есть Филиал, мы добавляем его как фильтр для точности.
		if d.BranchID != nil {
			query += " AND u.branch_id = $3 "
			args = append(args, *d.BranchID)
		}

	} else if d.BranchID != nil {
		// 3. НИЗКИЙ ПРИОРИТЕТ: Только Филиал -> Директор Филиала
		targetPosType = "BRANCH_DIRECTOR"
		query += " AND u.branch_id = $2 "
		// Ищем директора этого филиала
		args = append(args, targetPosType, *d.BranchID)

	} else if d.OfficeID != nil {
		// 4. МИНИМАЛЬНЫЙ: Офис обслуживания
		targetPosType = "HEAD_OF_OFFICE"
		query += " AND u.office_id = $2 "
		args = append(args, targetPosType, *d.OfficeID)

	} else {
		return nil, apperrors.NewHttpError(http.StatusBadRequest,
			"Не выбрано ни одно подразделение, невозможно определить руководителя автоматически.", nil, nil)
	}

	query += " LIMIT 1"

	s.logger.Debug("Auto-Resolve executing", zap.String("query", query), zap.Any("args", args))

	var u entities.User
	err := tx.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Fio, &u.Email, &u.PositionID, &u.DepartmentID, &u.BranchID,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.logger.Warn("Автоматический поиск не дал результатов",
				zap.String("role", targetPosType),
				zap.Any("context", d))

			return nil, apperrors.NewHttpError(http.StatusBadRequest,
				fmt.Sprintf("Не найден руководитель с ролью '%s' для выбранного подразделения. Проверьте штатную структуру.", targetPosType), nil, nil)
		}
		return nil, err
	}

	return &RoutingResult{
		Executor:  u,
		StatusID:  0,
		RuleFound: false, // Это автопоиск
	}, nil
}

func (s *RuleEngineService) findUserByPositionAndStructure(ctx context.Context, tx pgx.Tx, posID int, ctxData OrderContext) (*entities.User, error) {
	positionID := uint64(posID)
	// Вспомогательная функция, когда мы нашли ПРАВИЛО (пункт 2), но нужно найти человека
	query := `
		SELECT u.id, u.fio, u.email, u.position_id, u.department_id, u.branch_id 
		FROM users u
		JOIN statuses s ON u.status_id = s.id 
		WHERE u.position_id = $1 
		  AND u.deleted_at IS NULL
		  AND UPPER(s.code) = 'ACTIVE'
	`
	args := []interface{}{positionID}
	argIdx := 2

	// Здесь мы пытаемся быть мягче: ИЛИ совпадает, ИЛИ у юзера это поле NULL (глобальный начальник)
	if ctxData.DepartmentID != 0 {
		query += fmt.Sprintf(" AND (u.department_id = $%d OR u.department_id IS NULL)", argIdx)
		args = append(args, ctxData.DepartmentID)
		argIdx++
	}
	// Если департамент выбран, то обычно Branch игнорируется для поиска головного офиса,
	// но для региональных правил Branch важен.
	if ctxData.BranchID != nil {
		query += fmt.Sprintf(" AND (u.branch_id = $%d OR u.branch_id IS NULL)", argIdx)
		args = append(args, *ctxData.BranchID)
		argIdx++
	}
	if ctxData.OtdelID != nil {
		query += fmt.Sprintf(" AND (u.otdel_id = $%d OR u.otdel_id IS NULL)", argIdx)
		args = append(args, *ctxData.OtdelID)
		argIdx++
	}
	
	query += " ORDER BY u.otdel_id NULLS LAST, u.department_id NULLS LAST, u.branch_id NULLS LAST LIMIT 1"

	var u entities.User
	err := tx.QueryRow(ctx, query, args...).Scan(&u.ID, &u.Fio, &u.Email, &u.PositionID, &u.DepartmentID, &u.BranchID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewHttpError(http.StatusBadRequest,
				"Правило найдено, но нет активного сотрудника с нужной должностью в данном подразделении.", nil, nil)
		}
		return nil, err
	}
	return &u, nil
}

func (s *RuleEngineService) GetPredefinedRoute(ctx context.Context, tx pgx.Tx, orderTypeID uint64) (*RoutingResult, error) {
	query := `SELECT department_id, otdel_id FROM order_routing_rules WHERE order_type_id = $1 LIMIT 1`
	var res RoutingResult
	err := tx.QueryRow(ctx, query, orderTypeID).Scan(&res.DepartmentID, &res.OtdelID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &res, nil
}

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

// ResolveExecutor
func (s *RuleEngineService) ResolveExecutor(ctx context.Context, tx pgx.Tx, orderCtx OrderContext, explicitExecutorID *uint64) (*RoutingResult, error) {
	// 1. Явный исполнитель
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

	// 2. Попытка найти ПРАВИЛО (Dynamic SQL)
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
	// ================= [ЛОГИКА ФОЛБЕКА (FALLBACK)] =================
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.logger.Info("Правило не найдено. Запуск поиска по иерархии", zap.Uint64("type", orderCtx.OrderTypeID))

			// Запускаем автоматический поиск начальника по иерархии
			return s.resolveByHierarchy(ctx, tx, orderCtx)
		}
		return nil, fmt.Errorf("ошибка SQL: %w", err)
	}
	// ================================================================

	if targetPositionID == nil {
		return nil, apperrors.NewHttpError(http.StatusInternalServerError, "В правиле не указана должность", nil, nil)
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

// resolveByHierarchy - находит руководителя автоматически на основе структуры заявки
func (s *RuleEngineService) resolveByHierarchy(ctx context.Context, tx pgx.Tx, d OrderContext) (*RoutingResult, error) {
	var targetPosType string

	// ОПРЕДЕЛЯЕМ ИЕРАРХИЮ
	// 1. Департамент выбран? -> Директор департамента
	if d.DepartmentID != 0 {
		targetPosType = "HEAD_OF_DEPARTMENT" // Поправь на свой точный константный string, если отличается
	} else if d.OtdelID != nil {
		// 2. Нет Депа, но есть Отдел -> Начальник Отдела
		targetPosType = "MANAGER_OF_OTDEL"
	} else if d.BranchID != nil {
		// 3. Нет Депа/Отдела, есть Филиал -> Директор Филиала
		targetPosType = "BRANCH_DIRECTOR"
	} else if d.OfficeID != nil {
		// 4. Только офис -> Начальник Офиса
		targetPosType = "HEAD_OF_OFFICE"
	} else {
		return nil, apperrors.NewHttpError(http.StatusBadRequest,
			"Не выбрано ни одно подразделение, невозможно определить руководителя.", nil, nil)
	}

	// Теперь ищем пользователя, у которого ДОЛЖНОСТЬ имеет этот ТИП в ЭТОМ подразделении
	query := `
		SELECT u.id, u.fio, u.email, u.position_id, u.department_id, u.branch_id
		FROM users u
		JOIN positions p ON u.position_id = p.id
		WHERE u.deleted_at IS NULL
		  AND p.type = $1
	`
	args := []interface{}{targetPosType}
	argIdx := 2

	if d.DepartmentID != 0 {
		query += fmt.Sprintf(" AND u.department_id = $%d", argIdx)
		args = append(args, d.DepartmentID)
		argIdx++
	}
	if d.OtdelID != nil {
		query += fmt.Sprintf(" AND u.otdel_id = $%d", argIdx)
		args = append(args, *d.OtdelID)
		argIdx++
	}
	// Важно: Для директоров департамента branch не всегда совпадает,
	// но если мы ищем Директора Филиала - branch важен.
	if d.BranchID != nil {
		// Если мы ищем Директора Департамента, часто игнорируют branch,
		// но здесь автоматика, оставим строгое соответствие или сделаем мягкое
		if targetPosType != "HEAD_OF_DEPARTMENT" {
			query += fmt.Sprintf(" AND u.branch_id = $%d", argIdx)
			args = append(args, *d.BranchID)
			argIdx++
		}
	}
	if d.OfficeID != nil {
		query += fmt.Sprintf(" AND u.office_id = $%d", argIdx)
		args = append(args, *d.OfficeID)
		argIdx++
	}

	query += " LIMIT 1"

	var u entities.User
	err := tx.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Fio, &u.Email, &u.PositionID, &u.DepartmentID, &u.BranchID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.logger.Warn("Автоматический поиск не дал результатов",
				zap.String("type", targetPosType))

			return nil, apperrors.NewHttpError(http.StatusBadRequest,
				fmt.Sprintf("Правило не найдено. Попытка назначить на '%s' не удалась: сотрудник не найден.", targetPosType), nil, nil)
		}
		return nil, err
	}

	return &RoutingResult{
		Executor:  u,
		StatusID:  0,
		RuleFound: false,
	}, nil
}

func (s *RuleEngineService) findUserByPositionAndStructure(ctx context.Context, tx pgx.Tx, posID int, ctxData OrderContext) (*entities.User, error) {
	positionID := uint64(posID)
	query := `SELECT id, fio, email, position_id, department_id, branch_id FROM users WHERE position_id = $1 AND deleted_at IS NULL`
	args := []interface{}{positionID}
	argIdx := 2

	if ctxData.DepartmentID != 0 {
		query += fmt.Sprintf(" AND (department_id = $%d OR department_id IS NULL)", argIdx)
		args = append(args, ctxData.DepartmentID)
		argIdx++
	}
	if ctxData.BranchID != nil && ctxData.DepartmentID == 0 {
		query += fmt.Sprintf(" AND (branch_id = $%d OR branch_id IS NULL)", argIdx)
		args = append(args, *ctxData.BranchID)
		argIdx++
	}
	if ctxData.OtdelID != nil {
		query += fmt.Sprintf(" AND (otdel_id = $%d OR otdel_id IS NULL)", argIdx)
		args = append(args, *ctxData.OtdelID)
		argIdx++
	}
	query += " ORDER BY otdel_id NULLS LAST, department_id NULLS LAST, branch_id NULLS LAST LIMIT 1"

	var u entities.User
	err := tx.QueryRow(ctx, query, args...).Scan(&u.ID, &u.Fio, &u.Email, &u.PositionID, &u.DepartmentID, &u.BranchID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewHttpError(http.StatusBadRequest,
				"Правило найдено, но в данном подразделении нет активного сотрудника с нужной должностью.", nil, nil)
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

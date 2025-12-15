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

type OrderContext struct {
	OrderTypeID  uint64
	DepartmentID uint64
	OtdelID      *uint64
	BranchID     *uint64
	OfficeID     *uint64
}

type RoutingResult struct {
	Executor  entities.User
	StatusID  int
	RuleFound bool

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

	// 2. Поиск правила
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
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.logger.Warn("Правило маршрутизации не найдено", zap.Uint64("type_id", orderCtx.OrderTypeID))
			return nil, apperrors.NewHttpError(http.StatusBadRequest,
				"Не найден исполнитель для данного типа заявки (правило отсутствует). Выберите исполнителя вручную.", nil, nil)
		}
		return nil, fmt.Errorf("ошибка SQL (rule lookup): %w", err)
	}

	if targetPositionID == nil {
		return nil, apperrors.NewHttpError(http.StatusInternalServerError, "В правиле не указана должность", nil, nil)
	}

	// 3. Поиск человека (Вызываем вспомогательную функцию ниже)
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

func (s *RuleEngineService) findUserByPositionAndStructure(ctx context.Context, tx pgx.Tx, posID int, ctxData OrderContext) (*entities.User, error) {
	positionID := uint64(posID)

	query := `
		SELECT id, fio, email, position_id, department_id, branch_id 
		FROM users 
		WHERE position_id = $1 
		  AND deleted_at IS NULL
	`

	args := []interface{}{positionID}
	argIdx := 2

	// 1. Проверяем Департамент
	if ctxData.DepartmentID != 0 {
		query += fmt.Sprintf(" AND (department_id = $%d OR department_id IS NULL)", argIdx)
		args = append(args, ctxData.DepartmentID)
		argIdx++
	}

	// 2. [ИСПРАВЛЕНИЕ]
	// Добавляем условие по Филиалу ТОЛЬКО если мы НЕ ищем по Департаменту.
	// Это решает твою проблему: Директор департамента найдется, даже если branch_id заявки отличается.
	if ctxData.BranchID != nil && ctxData.DepartmentID == 0 {
		query += fmt.Sprintf(" AND (branch_id = $%d OR branch_id IS NULL)", argIdx)
		args = append(args, *ctxData.BranchID)
		argIdx++
	}

	// 3. Проверяем Отдел
	if ctxData.OtdelID != nil {
		query += fmt.Sprintf(" AND (otdel_id = $%d OR otdel_id IS NULL)", argIdx)
		args = append(args, *ctxData.OtdelID)
		argIdx++
	}

	query += " ORDER BY otdel_id NULLS LAST, department_id NULLS LAST, branch_id NULLS LAST LIMIT 1"

	var u entities.User
	err := tx.QueryRow(ctx, query, args...).Scan(
		&u.ID, &u.Fio, &u.Email, &u.PositionID, &u.DepartmentID, &u.BranchID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.logger.Warn("Исполнитель не найден (структурный поиск)",
				zap.Uint64("pos", positionID),
				zap.Uint64("dept_ctx", ctxData.DepartmentID),
			)
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

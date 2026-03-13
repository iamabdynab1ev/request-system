package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"net/http"
	"request-system/pkg/constants"   
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
	// 1. Если исполнитель выбран вручную — берем его (тут без изменений)
	if explicitExecutorID != nil {
		user, err := s.userRepo.FindUserByIDInTx(ctx, tx, *explicitExecutorID)
		if err != nil { return nil, apperrors.NewHttpError(http.StatusBadRequest, "Исполнитель не найден", err, nil) }
		return &RoutingResult{Executor: *user, StatusID: 0, RuleFound: false}, nil
	}

	// 2. Ищем ПРАВИЛО в БД
	query := `
		SELECT assign_to_position_id, status_id, department_id, otdel_id, branch_id, office_id
		FROM order_routing_rules
		WHERE (order_type_id IS NULL OR order_type_id = $1)
			AND (department_id IS NULL OR department_id = $2)
			AND (otdel_id IS NULL OR otdel_id = $3)
			AND (branch_id IS NULL OR branch_id = $4)
			AND (office_id IS NULL OR office_id = $5)
		ORDER BY order_type_id NULLS LAST, otdel_id NULLS LAST, office_id NULLS LAST, department_id NULLS LAST, branch_id NULLS LAST
		LIMIT 1
	`
	var targetPositionID *int
	var targetStatusID int
	var ruleDept, ruleOtdel, ruleBranch, ruleOffice *uint64

	err := tx.QueryRow(ctx, query, orderCtx.OrderTypeID, orderCtx.DepartmentID, orderCtx.OtdelID, orderCtx.BranchID, orderCtx.OfficeID).
		Scan(&targetPositionID, &targetStatusID, &ruleDept, &ruleOtdel, &ruleBranch, &ruleOffice)

	// 3. Если правила НЕТ вообще — идем в стандартный Waterfall
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.resolveByHierarchy(ctx, tx, orderCtx)
		}
		return nil, fmt.Errorf("ошибка SQL правил: %w", err)
	}

	// 4. ПРАВИЛО ЕСТЬ — пробуем найти по нему конкретного человека
	foundUser, err := s.findUserByPositionAndStructure(ctx, tx, *targetPositionID, orderCtx)
	
	if err == nil {
		return &RoutingResult{Executor: *foundUser, StatusID: targetStatusID, RuleFound: true}, nil
	}

	// 5. 🔥 САМОЕ ВАЖНОЕ: Если по правилу человека НЕ НАШЛИ (позиция пуста),
	// мы НЕ выдаем ошибку, а отдаем заявку в Waterfall, но с учетом структуры из правила!
	s.logger.Info("Человек по должности из правила не найден, использую запасной поиск (Hierarchy Fallback)")
	
	// Подменяем контекст данными из правила, если они там указаны
	if ruleDept != nil { orderCtx.DepartmentID = *ruleDept }
	if ruleOtdel != nil { orderCtx.OtdelID = ruleOtdel }
	if ruleBranch != nil { orderCtx.BranchID = ruleBranch }
	if ruleOffice != nil { orderCtx.OfficeID = ruleOffice }

	return s.resolveByHierarchy(ctx, tx, orderCtx)
}

func (s *RuleEngineService) resolveByHierarchy(ctx context.Context, tx pgx.Tx, d OrderContext) (*RoutingResult, error) {
	var targetRoles []string
	var searchScopeName string
	var deptID, otdelID, branchID, officeID *uint64

	// 1. ОПРЕДЕЛЕНИЕ ГОЛОВНОГО ФИЛИАЛА ПО ИМЕНИ
	isHeadBranch := false
	if d.BranchID != nil {
		currentBranchName := ""
if err := tx.QueryRow(ctx, "SELECT name FROM branches WHERE id = $1", *d.BranchID).Scan(&currentBranchName); err != nil {
    s.logger.Warn("Не удалось получить название филиала", zap.Uint64("branchID", *d.BranchID), zap.Error(err))
}

		// Читаем эталон из настроек
		headBranchNames := os.Getenv("HEAD_BRANCH_NAMES")
		if headBranchNames == "" {
			headBranchNames = "Саридора" 
		}

		// Сравниваем
		for _, name := range strings.Split(headBranchNames, ",") {
			if strings.TrimSpace(currentBranchName) == strings.TrimSpace(name) {
				isHeadBranch = true
				break
			}
		}
	}

	// 2. ВОДОПАД (WATERFALL): Определяем кого искать
	if d.DepartmentID != 0 {
		// Департамент (Высший уровень)
		searchScopeName = "Департамент"
		targetRoles = []string{"HEAD_OF_DEPARTMENT", "DEPUTY_HEAD_OF_DEPARTMENT"}
		id := d.DepartmentID; deptID = &id

	} else if isHeadBranch && d.OfficeID != nil {
		// Служба в Саридоре (Трактуем как Департамент)
		searchScopeName = "Служба (Головной филиал)"
		targetRoles = []string{"HEAD_OF_DEPARTMENT", "DEPUTY_HEAD_OF_DEPARTMENT"}
		officeID = d.OfficeID

	} else if d.OtdelID != nil {
		// Отдел
		searchScopeName = "Отдел"
		targetRoles = []string{"HEAD_OF_OTDEL", "DEPUTY_HEAD_OF_OTDEL", "MANAGER"}
		otdelID = d.OtdelID
		if d.BranchID != nil { branchID = d.BranchID }

	} else if d.BranchID != nil {
		// Филиал (Региональный)
		searchScopeName = "Филиал"
		targetRoles = []string{"BRANCH_DIRECTOR", "DEPUTY_BRANCH_DIRECTOR"}
		branchID = d.BranchID

	} else if d.OfficeID != nil {
		// Офис (ЦБО Региональный)
		searchScopeName = "Офис"
		targetRoles = []string{"HEAD_OF_OFFICE", "DEPUTY_HEAD_OF_OFFICE"}
		officeID = d.OfficeID

	} else {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Не выбрано подразделение.", nil, nil)
	}

	// 3. ПОИСК В БАЗЕ (С ПОДДЕРЖКОЙ ЗАМЕСТИТЕЛЯ)
	for _, role := range targetRoles {
		query := `
			SELECT DISTINCT u.id, u.fio, u.email, u.position_id, u.department_id, u.branch_id
			FROM users u
			JOIN user_positions up ON u.id = up.user_id
			JOIN positions p ON up.position_id = p.id
			JOIN statuses s ON u.status_id = s.id
			WHERE u.deleted_at IS NULL 
			  AND UPPER(s.code) = 'ACTIVE' 
			  AND p.type = $1 
		`
		args := []interface{}{role}
		argIdx := 2

		// Подставляем только непустые фильтры
		if deptID != nil { query += fmt.Sprintf(" AND u.department_id = $%d", argIdx); args = append(args, *deptID); argIdx++ }
		if otdelID != nil { query += fmt.Sprintf(" AND u.otdel_id = $%d", argIdx); args = append(args, *otdelID); argIdx++ }
		if branchID != nil { query += fmt.Sprintf(" AND u.branch_id = $%d", argIdx); args = append(args, *branchID); argIdx++ }
		if officeID != nil { query += fmt.Sprintf(" AND u.office_id = $%d", argIdx); args = append(args, *officeID); argIdx++ }

		query += " LIMIT 1"

		var u entities.User
		err := tx.QueryRow(ctx, query, args...).Scan(
			&u.ID, &u.Fio, &u.Email, &u.PositionID, &u.DepartmentID, &u.BranchID,
		)

		if err == nil {
	
			s.logger.Info("Ответственный найден автоматически", zap.String("role", role), zap.String("fio", u.Fio))
			return &RoutingResult{Executor: u, RuleFound: false}, nil
		}
	
	}

	
roleName1 := ""
roleName2 := ""
if len(targetRoles) > 0 {
    roleName1 = constants.PositionTypeNames[constants.PositionType(targetRoles[0])]
    if roleName1 == "" { roleName1 = targetRoles[0] }
}
if len(targetRoles) > 1 {
    roleName2 = constants.PositionTypeNames[constants.PositionType(targetRoles[1])]
    if roleName2 == "" { roleName2 = targetRoles[1] }
}

return nil, apperrors.NewHttpError(http.StatusBadRequest,
    fmt.Sprintf("В подразделении '%s' не найден ни '%s', ни '%s'.", searchScopeName, roleName1, roleName2), nil, nil)
}
func (s *RuleEngineService) findUserByPositionAndStructure(ctx context.Context, tx pgx.Tx, posID int, ctxData OrderContext) (*entities.User, error) {
	positionID := uint64(posID)
	shouldIgnoreBranch := (ctxData.DepartmentID != 0 || ctxData.OtdelID != nil)

	query := `
		SELECT DISTINCT u.id, u.fio, u.email, u.position_id, u.department_id, u.branch_id 
		FROM users u
		JOIN user_positions up ON u.id = up.user_id
		JOIN statuses s ON u.status_id = s.id 
		WHERE up.position_id = $1 
		  AND u.deleted_at IS NULL
		  AND UPPER(s.code) = 'ACTIVE'
	`
	args := []interface{}{positionID}
	argIdx := 2
	
	if ctxData.DepartmentID != 0 {
		query += fmt.Sprintf(" AND (u.department_id = $%d OR u.department_id IS NULL)", argIdx)
		args = append(args, ctxData.DepartmentID); argIdx++
	}
	if ctxData.OtdelID != nil {
		query += fmt.Sprintf(" AND (u.otdel_id = $%d OR u.otdel_id IS NULL)", argIdx)
		args = append(args, *ctxData.OtdelID); argIdx++
	}
	if !shouldIgnoreBranch && ctxData.BranchID != nil {
		query += fmt.Sprintf(" AND (u.branch_id = $%d OR u.branch_id IS NULL)", argIdx)
		args = append(args, *ctxData.BranchID); argIdx++
	}

	query += " ORDER BY u.id ASC LIMIT 1"

	var u entities.User
	err := tx.QueryRow(ctx, query, args...).Scan(&u.ID, &u.Fio, &u.Email, &u.PositionID, &u.DepartmentID, &u.BranchID)
	if err != nil {
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

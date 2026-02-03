package services

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
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
	positionRepo  repositories.PositionRepositoryInterface
	orderTypeRepo repositories.OrderTypeRepositoryInterface
	txManager     repositories.TxManagerInterface
	logger        *zap.Logger
}

func NewOrderRoutingRuleService(
	r repositories.OrderRoutingRuleRepositoryInterface,
	u repositories.UserRepositoryInterface,
	p repositories.PositionRepositoryInterface,
	tm repositories.TxManagerInterface,
	l *zap.Logger,
	otr repositories.OrderTypeRepositoryInterface,
) OrderRoutingRuleServiceInterface {
	return &OrderRoutingRuleService{
		repo:          r,
		userRepo:      u,
		positionRepo:  p,
		txManager:     tm,
		logger:        l,
		orderTypeRepo: otr,
	}
}

// Вспомогательный метод для проверки "Головного филиала" (Саридора)
func (s *OrderRoutingRuleService) checkIsHeadBranch(ctx context.Context, branchID *int) bool {
	if branchID == nil {
		return false
	}
	var name string
	err := s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT name FROM branches WHERE id = $1", *branchID).Scan(&name)
	})
	if err != nil {
		return false
	}

	headBranchNames := os.Getenv("HEAD_BRANCH_NAMES")
	if headBranchNames == "" {
		headBranchNames = "Саридора"
	}
	for _, n := range strings.Split(headBranchNames, ",") {
		if strings.TrimSpace(name) == strings.TrimSpace(n) {
			return true
		}
	}
	return false
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
		BranchID:     entity.BranchID,
		OfficeID:     entity.OfficeID,
		PositionID:   entity.PositionID,
		StatusID:     entity.StatusID,
		CreatedAt:    entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    entity.UpdatedAt.Format(time.RFC3339),
	}

	if entity.PositionID != nil {
		pos, err := s.positionRepo.FindByID(ctx, nil, uint64(*entity.PositionID))
		if err == nil && pos != nil && pos.Type != nil {
			response.PositionType = *pos.Type
			if name, ok := constants.PositionTypeNames[constants.PositionType(*pos.Type)]; ok {
				response.PositionTypeName = name
			}
		}
	}
	return response, nil
}

// === CREATE ===
func (s *OrderRoutingRuleService) Create(ctx context.Context, d dto.CreateOrderRoutingRuleDTO) (*dto.OrderRoutingRuleResponseDTO, error) {
	authContext, err := buildRuleAuthzContext(ctx, s.userRepo)
	if err != nil || !authz.CanDo(authz.OrderRuleCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	isHead := s.checkIsHeadBranch(ctx, d.BranchID)

	var searchDept, searchOtdel, searchBranch, searchOffice *uint64
	if d.DepartmentID != nil { v := uint64(*d.DepartmentID); searchDept = &v }
	if d.OtdelID != nil { v := uint64(*d.OtdelID); searchOtdel = &v }
	if d.BranchID != nil { v := uint64(*d.BranchID); searchBranch = &v }
	if d.OfficeID != nil { v := uint64(*d.OfficeID); searchOffice = &v }

	switch constants.PositionType(d.PositionType) {
	case constants.PositionTypeHeadOfDepartment, constants.PositionTypeDeputyHeadOfDepartment:
		searchOtdel = nil
		d.OtdelID = nil
		if !(isHead && d.OfficeID != nil) {
			searchBranch = nil
			searchOffice = nil
			d.BranchID = nil
			d.OfficeID = nil
		}

	case constants.PositionTypeHeadOfOtdel, constants.PositionTypeDeputyHeadOfOtdel, constants.PositionTypeManager:
		searchOffice = nil
		searchBranch = nil
		d.BranchID = nil
		d.OfficeID = nil

	case constants.PositionTypeBranchDirector, constants.PositionTypeDeputyBranchDirector:
		searchDept = nil; searchOtdel = nil; searchOffice = nil
		d.DepartmentID = nil; d.OtdelID = nil; d.OfficeID = nil

	case constants.PositionTypeHeadOfOffice, constants.PositionTypeDeputyHeadOfOffice:
		searchDept = nil; searchOtdel = nil
		d.DepartmentID = nil; d.OtdelID = nil
	}

	realPositionID, err := s.userRepo.FindPositionIDByStructureAndType(ctx, nil, searchBranch, searchOffice, searchDept, searchOtdel, d.PositionType)
	if err != nil { return nil, err }

	if realPositionID == 0 && constants.PositionType(d.PositionType) == constants.PositionTypeHeadOfOtdel {
		realPositionID, _ = s.userRepo.FindPositionIDByStructureAndType(ctx, nil, searchBranch, searchOffice, searchDept, searchOtdel, string(constants.PositionTypeManager))
	}

	if realPositionID == 0 {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Активный сотрудник с данной должностью в подразделении не найден.", nil, nil)
	}

	finalPosID := int(realPositionID)
	rule := &entities.OrderRoutingRule{
		RuleName:     d.RuleName,
		OrderTypeID:  d.OrderTypeID,
		DepartmentID: d.DepartmentID,
		OtdelID:      d.OtdelID,
		BranchID:     d.BranchID,
		OfficeID:     d.OfficeID,
		PositionID:   &finalPosID,
		StatusID:     d.StatusID,
	}

	var newID int
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		id, errTx := s.repo.Create(ctx, tx, rule)
		newID = id
		return errTx
	})
	if err != nil { return nil, err }

	created, _ := s.repo.FindByID(ctx, newID)
	return s.toResponseDTO(ctx, created)
}

// === UPDATE ===
func (s *OrderRoutingRuleService) Update(ctx context.Context, id int, d dto.UpdateOrderRoutingRuleDTO, rawBody []byte) (*dto.OrderRoutingRuleResponseDTO, error) {
	authContext, err := buildRuleAuthzContext(ctx, s.userRepo)
	if err != nil || !authz.CanDo(authz.OrderRuleUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil { return nil, err }

	var changes map[string]interface{}
	if err := json.Unmarshal(rawBody, &changes); err != nil {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат JSON", err, nil)
	}

	if d.RuleName.Valid { existing.RuleName = d.RuleName.String }
	if d.OrderTypeID.Valid { val := d.OrderTypeID.Int; existing.OrderTypeID = &val }
	if d.StatusID.Valid { existing.StatusID = d.StatusID.Int }

	needsReRouting := false
	if _, ok := changes["branch_id"]; ok {
		if d.BranchID.Valid { v := int(d.BranchID.Int); existing.BranchID = &v } else { existing.BranchID = nil }
		needsReRouting = true
	}
	if _, ok := changes["office_id"]; ok {
		if d.OfficeID.Valid { v := int(d.OfficeID.Int); existing.OfficeID = &v } else { existing.OfficeID = nil }
		needsReRouting = true
	}
	if _, ok := changes["department_id"]; ok {
		if d.DepartmentID.Valid { v := int(d.DepartmentID.Int); existing.DepartmentID = &v } else { existing.DepartmentID = nil }
		needsReRouting = true
	}
	if _, ok := changes["otdel_id"]; ok {
		if d.OtdelID.Valid { v := int(d.OtdelID.Int); existing.OtdelID = &v } else { existing.OtdelID = nil }
		needsReRouting = true
	}

	targetPosType := ""
	if posTypeVal, ok := changes["position_type"]; ok {
		needsReRouting = true
		targetPosType = posTypeVal.(string)
	} else if existing.PositionID != nil {
		pos, _ := s.positionRepo.FindByID(ctx, nil, uint64(*existing.PositionID))
		if pos != nil && pos.Type != nil { targetPosType = *pos.Type }
	}

	if needsReRouting && targetPosType != "" {
		isHead := s.checkIsHeadBranch(ctx, existing.BranchID)

		var sDept, sBranch, sOffice, sOtdel *uint64
		if existing.DepartmentID != nil { v := uint64(*existing.DepartmentID); sDept = &v }
		if existing.OtdelID != nil { v := uint64(*existing.OtdelID); sOtdel = &v }
		if existing.BranchID != nil { v := uint64(*existing.BranchID); sBranch = &v }
		if existing.OfficeID != nil { v := uint64(*existing.OfficeID); sOffice = &v }

		switch constants.PositionType(targetPosType) {
		case constants.PositionTypeHeadOfDepartment, constants.PositionTypeDeputyHeadOfDepartment:
			sOtdel = nil
			if !(isHead && existing.OfficeID != nil) {
				sBranch = nil; sOffice = nil
				existing.BranchID = nil; existing.OfficeID = nil
			}
		case constants.PositionTypeHeadOfOtdel, constants.PositionTypeDeputyHeadOfOtdel, constants.PositionTypeManager:
			sBranch = nil; sOffice = nil
		case constants.PositionTypeBranchDirector, constants.PositionTypeDeputyBranchDirector:
			sDept = nil; sOtdel = nil; sOffice = nil
		case constants.PositionTypeHeadOfOffice, constants.PositionTypeDeputyHeadOfOffice:
			sDept = nil; sOtdel = nil
		}

		realPosID, _ := s.userRepo.FindPositionIDByStructureAndType(ctx, nil, sBranch, sOffice, sDept, sOtdel, targetPosType)
		if realPosID == 0 && constants.PositionType(targetPosType) == constants.PositionTypeHeadOfOtdel {
			realPosID, _ = s.userRepo.FindPositionIDByStructureAndType(ctx, nil, sBranch, sOffice, sDept, sOtdel, string(constants.PositionTypeManager))
		}
		if realPosID == 0 {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Сотрудник не найден в выбранной структуре.", nil, nil)
		}
		newID := int(realPosID)
		existing.PositionID = &newID
	}

	now := time.Now(); existing.UpdatedAt = &now
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error { return s.repo.Update(ctx, tx, existing) })
	if err != nil { return nil, err }

	updated, _ := s.repo.FindByID(ctx, id)
	return s.toResponseDTO(ctx, updated)
}

func (s *OrderRoutingRuleService) GetByID(ctx context.Context, id int) (*dto.OrderRoutingRuleResponseDTO, error) {
	authContext, err := buildRuleAuthzContext(ctx, s.userRepo)
	if err != nil || !authz.CanDo(authz.OrderRuleView, *authContext) { return nil, apperrors.ErrForbidden }
	entity, err := s.repo.FindByID(ctx, id)
	if err != nil { return nil, err }
	return s.toResponseDTO(ctx, entity)
}

func (s *OrderRoutingRuleService) GetAll(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.OrderRoutingRuleResponseDTO], error) {
	authContext, err := buildRuleAuthzContext(ctx, s.userRepo)
	if err != nil || !authz.CanDo(authz.OrderRuleView, *authContext) { return nil, apperrors.ErrForbidden }
	entities, total, err := s.repo.GetAll(ctx, limit, offset, search)
	if err != nil { return nil, err }
	dtos := make([]dto.OrderRoutingRuleResponseDTO, 0, len(entities))
	for _, e := range entities {
		responseDTO, _ := s.toResponseDTO(ctx, e)
		dtos = append(dtos, *responseDTO)
	}
	return &dto.PaginatedResponse[dto.OrderRoutingRuleResponseDTO]{
		List: dtos,
		Pagination: dto.PaginationObject{TotalCount: total, Page: (offset / limit) + 1, Limit: limit},
	}, nil
}

func (s *OrderRoutingRuleService) Delete(ctx context.Context, id int) error {
	authContext, err := buildRuleAuthzContext(ctx, s.userRepo)
	if err != nil || !authz.CanDo(authz.OrderRuleDelete, *authContext) { return apperrors.ErrForbidden }
	return s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error { return s.repo.Delete(ctx, tx, id) })
}

func buildRuleAuthzContext(ctx context.Context, repo repositories.UserRepositoryInterface) (*authz.Context, error) {
	userID, _ := utils.GetUserIDFromCtx(ctx)
	perms, _ := utils.GetPermissionsMapFromCtx(ctx)
	user, err := repo.FindUserByID(ctx, userID)
	if err != nil { return nil, err }
	return &authz.Context{Actor: user, Permissions: perms}, nil
}

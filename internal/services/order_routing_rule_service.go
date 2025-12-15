package services

import (
	"context"
	"encoding/json"
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
		if err != nil {
			s.logger.Warn("Не удалось загрузить должность для правила",
				zap.Int("rule_id", entity.ID),
				zap.Int("position_id", *entity.PositionID),
				zap.Error(err))
		} else if pos != nil {
			if pos.Type != nil {
				response.PositionType = *pos.Type
				if name, ok := constants.PositionTypeNames[constants.PositionType(*pos.Type)]; ok {
					response.PositionTypeName = name
				}
			} else {
				s.logger.Warn("У должности отсутствует тип",
					zap.Int("position_id", *entity.PositionID))
			}
		}
	}
	return response, nil
}

// === CREATE ===
func (s *OrderRoutingRuleService) Create(ctx context.Context, d dto.CreateOrderRoutingRuleDTO) (*dto.OrderRoutingRuleResponseDTO, error) {
	// 1. Авторизация
	authContext, err := buildRuleAuthzContext(ctx, s.userRepo)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrderRuleCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	// [ВАЖНО] 2. Сначала готовим переменные поиска (преобразуем *int DTO в *uint64)
	var searchDept *uint64
	if d.DepartmentID != nil {
		v := uint64(*d.DepartmentID)
		searchDept = &v
	}
	var searchOtdel *uint64
	if d.OtdelID != nil {
		v := uint64(*d.OtdelID)
		searchOtdel = &v
	}
	var searchBranch *uint64
	if d.BranchID != nil {
		v := uint64(*d.BranchID)
		searchBranch = &v
	}
	var searchOffice *uint64
	if d.OfficeID != nil {
		v := uint64(*d.OfficeID)
		searchOffice = &v
	}

	// [ВАЖНО] 3. Очищаем лишние поля в зависимости от типа должности
	// (Если это Директор Департамента, нам неважно, что прислали BranchID - мы его обнуляем)
	switch constants.PositionType(d.PositionType) {
	case constants.PositionTypeHeadOfDepartment, constants.PositionTypeDeputyHeadOfDepartment:
		searchOtdel = nil
		searchBranch = nil
		searchOffice = nil
		d.OtdelID = nil
		d.BranchID = nil
		d.OfficeID = nil

	case constants.PositionTypeManagerOfOtdel:
		searchOffice = nil
		searchBranch = nil // Обычно отдел в структуре Head Office не имеет Branch
		d.BranchID = nil
		d.OfficeID = nil

	case constants.PositionTypeBranchDirector, constants.PositionTypeDeputyBranchDirector:
		searchDept = nil
		searchOtdel = nil
		searchOffice = nil
		d.DepartmentID = nil
		d.OtdelID = nil
		d.OfficeID = nil

	case constants.PositionTypeHeadOfOffice, constants.PositionTypeDeputyHeadOfOffice:
		searchDept = nil
		searchOtdel = nil
		d.DepartmentID = nil
		d.OtdelID = nil
	}

	// [ИСПРАВЛЕНИЕ ЗДЕСЬ!]
	// Мы используем новый метод репозитория. Он вернет точный ID должности (200),
	// который привязан к сотруднику в ЭТОМ департаменте.

	realPositionID, err := s.userRepo.FindPositionIDByStructureAndType(ctx, nil, searchBranch, searchOffice, searchDept, searchOtdel, d.PositionType)
	if err != nil {
		return nil, err
	}

	// Если вернулся 0 - значит сотрудника нет
	if realPositionID == 0 {
		msg := "В выбранном подразделении отсутствуют активные сотрудники с данной должностью. Сначала наймите сотрудника."
		return nil, apperrors.NewHttpError(http.StatusBadRequest, msg, nil, nil)
	}

	// 4. Создаем правило с ПРАВИЛЬНЫМ ID (realPositionID)
	finalPosID := int(realPositionID) // тут будет 200, а не 5

	rule := &entities.OrderRoutingRule{
		RuleName:     d.RuleName,
		OrderTypeID:  d.OrderTypeID,
		DepartmentID: d.DepartmentID, // Уже почищенные выше
		OtdelID:      d.OtdelID,
		BranchID:     d.BranchID,
		OfficeID:     d.OfficeID,
		PositionID:   &finalPosID, // <-- Используем ID из реальной структуры
		StatusID:     d.StatusID,
	}

	var newID int
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		id, errTx := s.repo.Create(ctx, tx, rule)
		newID = id
		return errTx
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
	authContext, err := buildRuleAuthzContext(ctx, s.userRepo)
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

	// 1. Обновление простых полей (изменений не было)
	if d.RuleName.Valid {
		existing.RuleName = d.RuleName.String
	}
	if d.OrderTypeID.Valid {
		val := d.OrderTypeID.Int
		existing.OrderTypeID = &val
	}
	if d.StatusID.Valid {
		existing.StatusID = d.StatusID.Int
	}

	// 2. Обработка изменений
	needsReRouting := false
	targetPosType := ""

	var changes map[string]interface{}
	if err := json.Unmarshal(rawBody, &changes); err != nil {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат JSON", err, nil)
	}

	// Если прислали новые значения полей, обновляем их в существующем объекте
	if d.BranchID.Valid {
		existing.BranchID = &d.BranchID.Int
		needsReRouting = true
	} else if _, ok := changes["branch_id"]; ok {
		existing.BranchID = nil
		needsReRouting = true
	}

	if d.OfficeID.Valid {
		existing.OfficeID = &d.OfficeID.Int
		needsReRouting = true
	} else if _, ok := changes["office_id"]; ok {
		existing.OfficeID = nil
		needsReRouting = true
	}

	if d.DepartmentID.Valid {
		existing.DepartmentID = &d.DepartmentID.Int
		needsReRouting = true
	} else if _, ok := changes["department_id"]; ok {
		existing.DepartmentID = nil
		needsReRouting = true
	}

	if d.OtdelID.Valid {
		existing.OtdelID = &d.OtdelID.Int
		needsReRouting = true
	} else if _, ok := changes["otdel_id"]; ok {
		existing.OtdelID = nil
		needsReRouting = true
	}

	// Проверяем изменение Типа Должности
	if posTypeVal, ok := changes["position_type"]; ok {
		needsReRouting = true
		targetPosType = posTypeVal.(string)
	} else if existing.PositionID != nil {
		pos, _ := s.positionRepo.FindByID(ctx, nil, uint64(*existing.PositionID))
		if pos != nil && pos.Type != nil {
			targetPosType = *pos.Type
		}
	}

	// 3. Динамический поиск нового PositionID
	if needsReRouting && targetPosType != "" {
		// Подготавливаем переменные ТОЛЬКО ДЛЯ ПОИСКА
		var sDept, sBranch, sOffice, sOtdel *uint64
		if existing.DepartmentID != nil {
			v := uint64(*existing.DepartmentID)
			sDept = &v
		}
		if existing.OtdelID != nil {
			v := uint64(*existing.OtdelID)
			sOtdel = &v
		}
		if existing.BranchID != nil {
			v := uint64(*existing.BranchID)
			sBranch = &v
		}
		if existing.OfficeID != nil {
			v := uint64(*existing.OfficeID)
			sOffice = &v
		}

		// Очищаем ПЕРЕМЕННЫЕ ПОИСКА согласно логике,
		// НО НЕ трогаем поля в `existing` (базе данных).
		switch constants.PositionType(targetPosType) {
		case constants.PositionTypeHeadOfDepartment, constants.PositionTypeDeputyHeadOfDepartment:
			// Ищем только в Департаменте, игнорируем отдел/офис при поиске
			sOtdel = nil
			sBranch = nil
			sOffice = nil

		case constants.PositionTypeManagerOfOtdel:
			// Для начальника отдела игнорируем офис и ветку
			sBranch = nil
			sOffice = nil

		case constants.PositionTypeBranchDirector, constants.PositionTypeDeputyBranchDirector:
			sDept = nil
			sOtdel = nil
			sOffice = nil

		case constants.PositionTypeHeadOfOffice, constants.PositionTypeDeputyHeadOfOffice:
			sDept = nil
			sOtdel = nil
		}

		// Вызываем новый метод, передавая отфильтрованные sDept, sOtdel...
		realPosID, err := s.userRepo.FindPositionIDByStructureAndType(ctx, nil, sBranch, sOffice, sDept, sOtdel, targetPosType)
		if err != nil {
			return nil, err
		}
		if realPosID == 0 {
			// Опционально: можно вернуть ошибку, если в такой конфигурации нет сотрудника
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "В данной структуре (согласно фильтрам) нет активного сотрудника с таким типом должности.", nil, nil)
		}

		// Сохраняем ТОЛЬКО новый PositionID (который стал 200)
		newID := int(realPosID)
		existing.PositionID = &newID
	}

	// 4. Сохраняем результат
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
	authContext, err := buildRuleAuthzContext(ctx, s.userRepo)
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
	authContext, err := buildRuleAuthzContext(ctx, s.userRepo)
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
	authContext, err := buildRuleAuthzContext(ctx, s.userRepo)
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

// Переименованная утилита, чтобы избежать конфликтов
func buildRuleAuthzContext(ctx context.Context, repo repositories.UserRepositoryInterface) (*authz.Context, error) {
	userID, _ := utils.GetUserIDFromCtx(ctx)
	perms, _ := utils.GetPermissionsMapFromCtx(ctx)
	user, err := repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &authz.Context{Actor: user, Permissions: perms}, nil
}

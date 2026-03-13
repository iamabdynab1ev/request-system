package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	sq "github.com/Masterminds/squirrel"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/events"
	"request-system/internal/repositories"

	apperrors "request-system/pkg/errors"
	"request-system/pkg/eventbus"
	"request-system/pkg/filestorage"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

type ValidationRule struct {
	FieldName    string
	ErrorMessage string
}

var OrderValidationRules = map[string][]string{
	"EQUIPMENT": {"equipment_id", "equipment_type_id", "priority_id"},
}

var ValidationRegistry = map[string][]ValidationRule{
	"EQUIPMENT": {
		{FieldName: "equipment_id", ErrorMessage: "Пожалуйста, укажите оборудование."},
		{FieldName: "equipment_type_id", ErrorMessage: "Пожалуйста, выберите тип оборудования."},
		{FieldName: "priority_id", ErrorMessage: "Пожалуйста, укажите приоритет."},
	},
}

type OrderServiceInterface interface {
	CreateOrder(ctx context.Context, createDTO dto.CreateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	GetOrders(ctx context.Context, filter types.Filter, onlyParticipant bool, onlyAssigned bool) (*dto.OrderListResponseDTO, error)
	FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error)
	UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader, explicitFields map[string]interface{}) (*dto.OrderResponseDTO, error)
	DeleteOrder(ctx context.Context, orderID uint64) error

	GetStatusByID(ctx context.Context, id uint64) (*entities.Status, error)
	GetPriorityByID(ctx context.Context, id uint64) (*entities.Priority, error)
	GetUserStats(ctx context.Context, userID uint64) (*types.UserOrderStats, error)
	GetValidationConfigForOrderType(ctx context.Context, orderTypeID uint64) (map[string]interface{}, error)
	FindOrderByIDForTelegram(ctx context.Context, userID uint64, orderID uint64) (*entities.Order, error)
}

type OrderService struct {
	txManager             repositories.TxManagerInterface
	orderRepo             repositories.OrderRepositoryInterface
	userRepo              repositories.UserRepositoryInterface
	statusRepo            repositories.StatusRepositoryInterface
	priorityRepo          repositories.PriorityRepositoryInterface
	attachRepo            repositories.AttachmentRepositoryInterface
	ruleEngine            RuleEngineServiceInterface
	historyRepo           repositories.OrderHistoryRepositoryInterface
	orderTypeRepo         repositories.OrderTypeRepositoryInterface
	fileStorage           filestorage.FileStorageInterface
	eventBus              *eventbus.Bus
	logger                *zap.Logger
	authPermissionService AuthPermissionServiceInterface
	notificationService   NotificationServiceInterface
}

func NewOrderService(
	txManager repositories.TxManagerInterface,
	orderRepo repositories.OrderRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface,
	attachRepo repositories.AttachmentRepositoryInterface,
	ruleEngine RuleEngineServiceInterface,
	historyRepo repositories.OrderHistoryRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	eventBus *eventbus.Bus,
	logger *zap.Logger,
	orderTypeRepo repositories.OrderTypeRepositoryInterface,
	authPermissionService AuthPermissionServiceInterface,
	notificationService NotificationServiceInterface,
) OrderServiceInterface {
	return &OrderService{
		txManager:             txManager,
		orderRepo:             orderRepo,
		userRepo:              userRepo,
		statusRepo:            statusRepo,
		priorityRepo:          priorityRepo,
		attachRepo:            attachRepo,
		ruleEngine:            ruleEngine,
		historyRepo:           historyRepo,
		fileStorage:           fileStorage,
		eventBus:              eventBus,
		logger:                logger,
		orderTypeRepo:         orderTypeRepo,
		authPermissionService: authPermissionService,
		notificationService:   notificationService,
	}
}

func (s *OrderService) GetOrders(ctx context.Context, filter types.Filter, onlyParticipant bool, onlyAssigned bool) (*dto.OrderListResponseDTO, error) {
	// 1. Получаем актора
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	// 2. AuthZ: Право на просмотр
	authCtx := authz.Context{Actor: actor, Permissions: permissionsMap}
	if !authz.CanDo(authz.OrdersView, authCtx) {
		s.logger.Warn("Попытка доступа без прав на просмотр заявок", zap.Uint64("userID", userID))
		return nil, apperrors.ErrForbidden
	}

	securityBuilder := sq.And{}

	if !authCtx.HasPermission(authz.ScopeAll) && !authCtx.HasPermission(authz.ScopeAllView) {
		scopeConditions := sq.Or{}

		if authCtx.HasPermission(authz.ScopeDepartment) && actor.DepartmentID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.department_id": *actor.DepartmentID})
		}
		if authCtx.HasPermission(authz.ScopeBranch) && actor.BranchID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.branch_id": *actor.BranchID})
		}
		if authCtx.HasPermission(authz.ScopeOtdel) && actor.OtdelID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.otdel_id": *actor.OtdelID})
		}
		if authCtx.HasPermission(authz.ScopeOffice) && actor.OfficeID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.office_id": *actor.OfficeID})
		}
		if authCtx.HasPermission(authz.ScopeOwn) {
			scopeConditions = append(scopeConditions, sq.Eq{"o.user_id": actor.ID})
			scopeConditions = append(scopeConditions, sq.Eq{"o.executor_id": actor.ID})

			scopeConditions = append(scopeConditions, sq.Expr("o.id IN (SELECT DISTINCT order_id FROM order_history WHERE user_id = ?)", actor.ID))
		}

		if len(scopeConditions) > 0 {
			securityBuilder = append(securityBuilder, scopeConditions)
		} else {

			return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
		}
	}

	if onlyParticipant {
		securityBuilder = append(securityBuilder, sq.Eq{"o.user_id": actor.ID})
	}

	// Назначены мне — только исполнитель
	if onlyAssigned {
		securityBuilder = append(securityBuilder, sq.Eq{"o.executor_id": actor.ID})
	}

	// 5. Запрос в БД
	orders, totalCount, err := s.orderRepo.GetOrders(ctx, filter, securityBuilder)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
	}

	// 6. Обогащение данных (Users, Attachments)
	dtos := s.mapOrdersToDTOs(ctx, orders)

	return &dto.OrderListResponseDTO{List: dtos, TotalCount: totalCount}, nil
}

func (s *OrderService) FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error) {
	authCtx, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	order := authCtx.Target.(*entities.Order)

	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, order.ID, 100, 0)

	return s.toResponseDTO(order, nil, nil, attachments), nil
}

func (s *OrderService) FindOrderByIDForTelegram(ctx context.Context, userID uint64, orderID uint64) (*entities.Order, error) {
	if userID == 0 {
		s.logger.Error("FindOrderByIDForTelegram вызван с userID=0",
			zap.Uint64("orderID", orderID),
			zap.Stack("stacktrace"))
		return nil, apperrors.ErrUserNotFound
	}

	if orderID == 0 {
		s.logger.Error("FindOrderByIDForTelegram вызван с orderID=0",
			zap.Uint64("userID", userID))
		return nil, apperrors.NewBadRequestError("ID заявки не указан")
	}

	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		s.logger.Warn("Заявка не найдена",
			zap.Uint64("orderID", orderID),
			zap.Uint64("userID", userID),
			zap.Error(err))
		return nil, err
	}

	// Fast auth check
	permissions, _ := s.authPermissionService.GetAllUserPermissions(ctx, userID)
	permMap := make(map[string]bool)
	for _, p := range permissions {
		permMap[p] = true
	}

	user, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("Пользователь не найден при проверке прав через Telegram",
			zap.Uint64("userID", userID),
			zap.Uint64("orderID", orderID),
			zap.Error(err))
		return nil, apperrors.ErrUserNotFound
	}

	authCtx := authz.Context{Actor: user, Permissions: permMap, Target: order}
	if !authz.CanDo(authz.OrdersView, authCtx) {
		s.logger.Warn("Попытка доступа к заявке без прав через Telegram",
			zap.Uint64("userID", userID),
			zap.Uint64("orderID", orderID),
			zap.String("user_fio", user.Fio))
		return nil, apperrors.ErrForbidden
	}

	return order, nil
}

func (s *OrderService) CreateOrder(ctx context.Context, createDTO dto.CreateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	// 1. AuthZ
	authCtx, err := s.buildAuthzContext(ctx, 0)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersCreate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	// 2. Business Rules Validation
	if err := s.validateOrderRules(ctx, createDTO); err != nil {
		return nil, err
	}

	hasDept := createDTO.DepartmentID != nil
	hasBranch := createDTO.BranchID != nil
	hasOtdel := createDTO.OtdelID != nil
	hasOffice := createDTO.OfficeID != nil

	// Проверяем: если ВСЕ четыре поля пусты, тогда ошибка.
	if !hasDept && !hasBranch && !hasOtdel && !hasOffice {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Необходимо указать хотя бы одно подразделение (Департамент, Филиал, Отдел или Офис ЦБО).", nil, nil)
	}

	var createdID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New()

		// 3. Rule Engine (определение исполнителя)
		orderCtx := OrderContext{
			OrderTypeID:  utils.SafeDeref(createDTO.OrderTypeID),
			DepartmentID: utils.SafeDeref(createDTO.DepartmentID),
			OtdelID:      createDTO.OtdelID,
			BranchID:     createDTO.BranchID,
			OfficeID:     createDTO.OfficeID,
		}

		// Если юзер выбрал исполнителя вручную
		var explicitExecutor *uint64 = createDTO.ExecutorID
		if explicitExecutor != nil && !authz.CanDo(authz.OrdersCreateExecutorID, *authCtx) {
			return apperrors.ErrForbidden
		}

		routingResult, err := s.ruleEngine.ResolveExecutor(ctx, tx, orderCtx, explicitExecutor)
		if err != nil {
			return err
		}
		if routingResult.Executor.ID == 0 {
			return apperrors.NewHttpError(
				http.StatusBadRequest,
				"Не найден руководитель для выбранной структуры. Настройте правила маршрутизации или укажите исполнителя вручную.",
				nil, nil)
		}

		// 4. Defaults
		status, err := s.statusRepo.FindByCodeInTx(ctx, tx, "OPEN")
		if err != nil {
			return apperrors.ErrInternalServer
		}

		// 5. Construct Entity
		orderEntity := &entities.Order{
			Name:            createDTO.Name,
			Address:         createDTO.Address,
			OrderTypeID:     createDTO.OrderTypeID,
			DepartmentID:    createDTO.DepartmentID,
			OtdelID:         createDTO.OtdelID,
			BranchID:        createDTO.BranchID,
			OfficeID:        createDTO.OfficeID,
			PriorityID:      createDTO.PriorityID,
			EquipmentID:     createDTO.EquipmentID,
			EquipmentTypeID: createDTO.EquipmentTypeID,

			StatusID:   uint64(status.ID),
			CreatorID:  authCtx.Actor.ID,
			ExecutorID: &routingResult.Executor.ID,
			Duration:   createDTO.Duration,
		}

		// 6. DB Create
		newID, err := s.orderRepo.Create(ctx, tx, orderEntity)
if err != nil {
    return err
}
		createdID = newID
		orderEntity.ID = newID

		// 7. Log History & EventBus
		commentStr := ""
		if createDTO.Comment != nil {
			commentStr = *createDTO.Comment
		}

		// Основное событие CREATE
		if err := s.logHistoryEvent(ctx, tx, orderEntity.ID, authCtx.Actor, "CREATE", &orderEntity.Name, nil, nil, txID, *orderEntity); err != nil {
			return err
		}

		// Событие Комментарий
		if commentStr != "" {
			if err := s.logHistoryEvent(ctx, tx, orderEntity.ID, authCtx.Actor, "COMMENT", nil, nil, &commentStr, txID, *orderEntity); err != nil {
				return err
			}
		}

		// Событие Назначение
		delegationTxt := "Назначено на: " + routingResult.Executor.Fio
		exIDStr := fmt.Sprintf("%d", routingResult.Executor.ID)
		if err := s.logHistoryEvent(ctx, tx, orderEntity.ID, authCtx.Actor, "DELEGATION", &exIDStr, nil, &delegationTxt, txID, *orderEntity); err != nil {
			return err
		}
		// Событие Status OPEN
		statusIDStr := fmt.Sprintf("%d", status.ID)
		if err := s.logHistoryEvent(ctx, tx, orderEntity.ID, authCtx.Actor, "STATUS_CHANGE", &statusIDStr, nil, nil, txID, *orderEntity); err != nil {
			return err
		}

		// 8. Файл
		if file != nil {
			if _, err := s.attachFileToOrderInTx(ctx, tx, orderEntity.ID, authCtx.Actor.ID, file, &txID, orderEntity); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.FindOrderByID(ctx, createdID)
}

func (s *OrderService) UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader, explicitFields map[string]interface{}) (*dto.OrderResponseDTO, error) {
	currentOrder, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// 1. Блокировка (только для CLOSED)
	status, _ := s.statusRepo.FindStatus(ctx, currentOrder.StatusID)
	if status != nil && status.Code != nil && *status.Code == "CLOSED" {
		return nil, apperrors.NewBadRequestError("Заявка закрыта. Редактирование запрещено.")
	}

	authCtx, err := s.buildAuthzContextWithTarget(ctx, currentOrder)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	// 2. Валидация Комментария
	orderTypeCode, _ := s.orderTypeRepo.FindCodeByID(ctx, *currentOrder.OrderTypeID)
	if orderTypeCode != "EQUIPMENT" {
		if updateDTO.Comment == nil || strings.TrimSpace(*updateDTO.Comment) == "" {
			return nil, apperrors.NewBadRequestError("Для сохранения изменений необходимо добавить комментарий с описанием действий.")
		}
	}

	// Базовая защита
	if len(explicitFields) == 0 && file == nil {
		return nil, apperrors.NewBadRequestError("Нет данных для обновления")
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New()

		// Синхронизация времени
		loc := time.Local
		now := time.Now().In(loc)

		updated := *currentOrder

		// === ОБНОВЛЕНИЕ ПОЛЕЙ ===
		fieldsChanged := utils.SmartUpdate(&updated, explicitFields)
		updated.UpdatedAt = now
		// === ЛОГИКА РЕРОУТИНГА ===
		structureChanged := utils.DiffPtr(currentOrder.DepartmentID, updated.DepartmentID) ||
			utils.DiffPtr(currentOrder.OtdelID, updated.OtdelID) ||
			utils.DiffPtr(currentOrder.BranchID, updated.BranchID) ||
			utils.DiffPtr(currentOrder.OfficeID, updated.OfficeID)

		if structureChanged {
			s.logger.Info("Изменение структуры -> Поиск исполнителя", zap.Uint64("order_id", orderID))
			orderCtx := OrderContext{
				DepartmentID: utils.SafeDeref(updated.DepartmentID),
				OtdelID:      updated.OtdelID,
				BranchID:     updated.BranchID,
				OfficeID:     updated.OfficeID,
			}
			res, err := s.ruleEngine.ResolveExecutor(ctx, tx, orderCtx, nil)
			if err != nil {
				updated.ExecutorID = nil
			} else {
				updated.ExecutorID = &res.Executor.ID
			}
			fieldsChanged = true
		}

		s.calculateMetrics(ctx, &updated, currentOrder, updateDTO, authCtx.Actor.ID, now)

		// ✅ УЛУЧШЕННАЯ ПРОВЕРКА МЕТРИК - форсируем сохранение
		metricsChanged := false

		// Проверка FirstResponseTimeSeconds
		if updated.FirstResponseTimeSeconds != nil {
			if currentOrder.FirstResponseTimeSeconds == nil {
				metricsChanged = true
				s.logger.Info("🆕 Новая метрика: first_response_time_seconds",
					zap.Uint64("order_id", orderID),
					zap.Uint64("value", *updated.FirstResponseTimeSeconds))
			} else if *updated.FirstResponseTimeSeconds != *currentOrder.FirstResponseTimeSeconds {
				metricsChanged = true
				s.logger.Info("🔄 Обновлена метрика: first_response_time_seconds",
					zap.Uint64("order_id", orderID),
					zap.Uint64("old", *currentOrder.FirstResponseTimeSeconds),
					zap.Uint64("new", *updated.FirstResponseTimeSeconds))
			}
		}

		// Проверка ResolutionTimeSeconds
		if updated.ResolutionTimeSeconds != nil {
			if currentOrder.ResolutionTimeSeconds == nil {
				metricsChanged = true
				s.logger.Info("🆕 Новая метрика: resolution_time_seconds",
					zap.Uint64("order_id", orderID),
					zap.Uint64("value", *updated.ResolutionTimeSeconds))
			} else if *updated.ResolutionTimeSeconds != *currentOrder.ResolutionTimeSeconds {
				metricsChanged = true
				s.logger.Info("🔄 Обновлена метрика: resolution_time_seconds",
					zap.Uint64("order_id", orderID),
					zap.Uint64("old", *currentOrder.ResolutionTimeSeconds),
					zap.Uint64("new", *updated.ResolutionTimeSeconds))
			}
		}

		// Проверка IsFirstContactResolution
		if updated.IsFirstContactResolution != nil {
			if currentOrder.IsFirstContactResolution == nil {
				metricsChanged = true
				s.logger.Info("🆕 Новая метрика: is_first_contact_resolution",
					zap.Uint64("order_id", orderID),
					zap.Bool("value", *updated.IsFirstContactResolution))
			} else if *updated.IsFirstContactResolution != *currentOrder.IsFirstContactResolution {
				metricsChanged = true
				s.logger.Info("🔄 Обновлена метрика: is_first_contact_resolution",
					zap.Uint64("order_id", orderID),
					zap.Bool("old", *currentOrder.IsFirstContactResolution),
					zap.Bool("new", *updated.IsFirstContactResolution))
			}
		}

		// Проверка CompletedAt
		if !timePointersEqual(currentOrder.CompletedAt, updated.CompletedAt) {
			metricsChanged = true
			s.logger.Info("🔄 Обновлена дата завершения",
				zap.Uint64("order_id", orderID),
				zap.Any("old", currentOrder.CompletedAt),
				zap.Any("new", updated.CompletedAt))
		}

		if metricsChanged {
			fieldsChanged = true
			s.logger.Info("✅ Обнаружены изменения метрик - форсируем сохранение",
				zap.Uint64("order_id", orderID))
		}

		// === ЛОГИРОВАНИЕ ИСТОРИИ ===
		histChanged, err := s.detectAndLogChanges(ctx, tx, currentOrder, &updated, updateDTO, authCtx.Actor, txID, now)
		if err != nil {
			return err
		}

		// Если есть файл
		if file != nil {
			if _, err := s.attachFileToOrderInTx(ctx, tx, orderID, authCtx.Actor.ID, file, &txID, &updated); err != nil {
				return err
			}
			fieldsChanged = true
		}

		if !fieldsChanged && !histChanged {
			return apperrors.ErrNoChanges
		}
		return s.orderRepo.Update(ctx, tx, &updated)
	})

	if err != nil {
		if errors.Is(err, apperrors.ErrNoChanges) {
			return s.FindOrderByID(ctx, orderID)
		}
		return nil, err
	}
	return s.FindOrderByID(ctx, orderID)
}

// detectAndLogChanges - ЯДРО логирования
func (s *OrderService) detectAndLogChanges(ctx context.Context, tx pgx.Tx, old, new *entities.Order, dto dto.UpdateOrderDTO, actor *entities.User, txID uuid.UUID, now time.Time) (bool, error) {
	hasLoggable := false

	if dto.Comment != nil && strings.TrimSpace(*dto.Comment) != "" {
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "COMMENT", nil, nil, dto.Comment, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if old.Name != new.Name {
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "NAME_CHANGE", &new.Name, &old.Name, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}
	if !utils.StringPtrEqual(old.Address, new.Address) {
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "ADDRESS_CHANGE", new.Address, old.Address, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if !utils.TimeEqual(old.Duration, new.Duration) {
		valNew := ""
		valOld := ""
		if new.Duration != nil {
			valNew = new.Duration.Format(time.RFC3339)
		}
		if old.Duration != nil {
			valOld = old.Duration.Format(time.RFC3339)
		}

		if valNew != "" || valOld != "" {
			newPtr := &valNew
			oldPtr := &valOld
			if valNew == "" {
				newPtr = nil
			}
			if valOld == "" {
				oldPtr = nil
			}
			if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "DURATION_CHANGE", newPtr, oldPtr, nil, txID, *new); err != nil {
				return false, err
			}
			hasLoggable = true
		}
	}

	if utils.DiffPtr(old.EquipmentID, new.EquipmentID) {
		valNew := utils.PtrToString(new.EquipmentID)
		valOld := utils.PtrToString(old.EquipmentID)
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "EQUIPMENT_CHANGE", &valNew, &valOld, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if utils.DiffPtr(old.EquipmentTypeID, new.EquipmentTypeID) {
		valNew := utils.PtrToString(new.EquipmentTypeID)
		valOld := utils.PtrToString(old.EquipmentTypeID)
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "EQUIPMENT_TYPE_CHANGE", &valNew, &valOld, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if utils.DiffPtr(old.OrderTypeID, new.OrderTypeID) {
		valNew := utils.PtrToString(new.OrderTypeID)
		valOld := utils.PtrToString(old.OrderTypeID)
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "ORDER_TYPE_CHANGE", &valNew, &valOld, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	if utils.DiffPtr(old.ExecutorID, new.ExecutorID) {
		newExName := s.resolveUserName(ctx, new.ExecutorID)
		txt := "Назначено на: " + newExName
		valNew := utils.PtrToString(new.ExecutorID)
		valOld := utils.PtrToString(old.ExecutorID)

		item := &repositories.OrderHistoryItem{
			OrderID: new.ID, UserID: actor.ID, EventType: "DELEGATION",
			OldValue: s.toNullStr(valOld), NewValue: s.toNullStr(valNew),
			Comment: s.toNullStr(txt), TxID: &txID, CreatedAt: now,
			ExecutorFio:  s.toNullStr(newExName),
			DelegatorFio: s.toNullStr(actor.Fio),
		}
		if err := s.addHistoryAndPublish(ctx, tx, item, *new, actor); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	// Статус
	if old.StatusID != new.StatusID {
		sStrOld := fmt.Sprintf("%d", old.StatusID)
		sStrNew := fmt.Sprintf("%d", new.StatusID)
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "STATUS_CHANGE", &sStrNew, &sStrOld, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	// Приоритет
	if utils.DiffPtr(old.PriorityID, new.PriorityID) {
		valNew := utils.PtrToString(new.PriorityID)
		valOld := utils.PtrToString(old.PriorityID)
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "PRIORITY_CHANGE", &valNew, &valOld, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	// Структура
	if utils.DiffPtr(old.DepartmentID, new.DepartmentID) ||
		utils.DiffPtr(old.OtdelID, new.OtdelID) ||
		utils.DiffPtr(old.BranchID, new.BranchID) ||
		utils.DiffPtr(old.OfficeID, new.OfficeID) {

		changes := []string{}
		if utils.DiffPtr(old.DepartmentID, new.DepartmentID) {
			changes = append(changes, fmt.Sprintf("department_id: %s → %s", utils.PtrToString(old.DepartmentID), utils.PtrToString(new.DepartmentID)))
		}
		if utils.DiffPtr(old.OtdelID, new.OtdelID) {
			changes = append(changes, fmt.Sprintf("otdel_id: %s → %s", utils.PtrToString(old.OtdelID), utils.PtrToString(new.OtdelID)))
		}
		if utils.DiffPtr(old.BranchID, new.BranchID) {
			changes = append(changes, fmt.Sprintf("branch_id: %s → %s", utils.PtrToString(old.BranchID), utils.PtrToString(new.BranchID)))
		}
		if utils.DiffPtr(old.OfficeID, new.OfficeID) {
			changes = append(changes, fmt.Sprintf("office_id: %s → %s", utils.PtrToString(old.OfficeID), utils.PtrToString(new.OfficeID)))
		}

		txt := "Смена структуры: " + strings.Join(changes, "; ")

		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "STRUCTURE_CHANGE", nil, nil, &txt, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}
	return hasLoggable, nil
}

func (s *OrderService) DeleteOrder(ctx context.Context, orderID uint64) error {
	authCtx, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.OrdersDelete, *authCtx) {
		return apperrors.ErrForbidden
	}
	return s.orderRepo.DeleteOrder(ctx, orderID)
}

func (s *OrderService) GetStatusByID(ctx context.Context, id uint64) (*entities.Status, error) {
	return s.statusRepo.FindStatus(ctx, id)
}

func (s *OrderService) GetPriorityByID(ctx context.Context, id uint64) (*entities.Priority, error) {
	return s.priorityRepo.FindByID(ctx, id)
}

func (s *OrderService) GetUserStats(ctx context.Context, userID uint64) (*types.UserOrderStats, error) {
	return s.orderRepo.GetUserOrderStats(ctx, userID, time.Now().AddDate(0, 0, -30))
}

// attachFileToOrderInTx - физическое сохранение и запись в БД + История
func (s *OrderService) attachFileToOrderInTx(ctx context.Context, tx pgx.Tx, orderID, userID uint64, file *multipart.FileHeader, txID *uuid.UUID, order *entities.Order) (uint64, error) {
	reader, err := file.Open()
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	filePath, err := s.fileStorage.Save(reader, file.Filename, "orders")
	if err != nil {
		return 0, err
	}

	attach := &entities.Attachment{
		OrderID: orderID, UserID: userID, FileName: file.Filename, FilePath: filePath,
		FileType: file.Header.Get("Content-Type"), FileSize: file.Size, CreatedAt: time.Now(),
	}
	id, err := s.attachRepo.CreateInTx(ctx, tx, attach)
	if err != nil {
		return 0, err
	}

	// History event (нужен чтобы уведомить пользователей о файле)
	actor, _ := s.userRepo.FindUserByIDInTx(ctx, tx, userID) // safely ignore err

	// Create struct with Attachment populated for Listener
	historyItem := &repositories.OrderHistoryItem{
		OrderID: orderID, UserID: userID, EventType: "ATTACHMENT_ADD",
		NewValue: s.toNullStr(file.Filename), Attachment: attach, AttachmentID: sql.NullInt64{Int64: int64(id), Valid: true},
		TxID: txID, CreatedAt: time.Now(), CreatorFio: s.toNullStr(actor.Fio),
	}
	if err := s.addHistoryAndPublish(ctx, tx, historyItem, *order, actor); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *OrderService) mapOrdersToDTOs(ctx context.Context, orders []entities.Order) []dto.OrderResponseDTO {
	orderIDs := make([]uint64, len(orders))
	for i, o := range orders {
		orderIDs[i] = o.ID
	}

	attachMap, _ := s.attachRepo.FindAttachmentsByOrderIDs(ctx, orderIDs)

	res := make([]dto.OrderResponseDTO, len(orders))
	for i, o := range orders {
		atts := attachMap[o.ID]
		res[i] = *s.toResponseDTO(&o, nil, nil, atts)
	}
	return res
}

func (s *OrderService) toResponseDTO(o *entities.Order, cr, ex *entities.User, atts []entities.Attachment) *dto.OrderResponseDTO {
	d := &dto.OrderResponseDTO{
		ID:                       o.ID,
		Name:                     o.Name,
		StatusID:                 o.StatusID,
		CreatedAt:                o.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                o.UpdatedAt.Format(time.RFC3339),
		OrderTypeID:              o.OrderTypeID,
		Address:                  o.Address,
		DepartmentID:             o.DepartmentID,
		OtdelID:                  o.OtdelID,
		BranchID:                 o.BranchID,
		OfficeID:                 o.OfficeID,
		EquipmentID:              o.EquipmentID,
		EquipmentTypeID:          o.EquipmentTypeID,
		PriorityID:               o.PriorityID,
		Duration:                 o.Duration,
		CompletedAt:              o.CompletedAt,
		ResolutionTimeSeconds:    o.ResolutionTimeSeconds,
		FirstResponseTimeSeconds: o.FirstResponseTimeSeconds,

		// ID и FIO
		CreatorID:   o.CreatorID,
		CreatorName: o.CreatorName,
	}

	if o.ExecutorID != nil {
		d.ExecutorID = o.ExecutorID
		d.ExecutorName = o.ExecutorName
	}

	if o.ResolutionTimeSeconds != nil {
		d.ResolutionTimeFormatted = utils.FormatSecondsToHumanReadable(*o.ResolutionTimeSeconds)
	}
	if o.FirstResponseTimeSeconds != nil {
		d.FirstResponseTimeFormatted = utils.FormatSecondsToHumanReadable(*o.FirstResponseTimeSeconds)
	}

	d.Attachments = make([]dto.AttachmentResponseDTO, len(atts))
	for i, a := range atts {
		d.Attachments[i] = dto.AttachmentResponseDTO{ID: a.ID, FileName: a.FileName, URL: "/uploads/" + a.FilePath}
	}
	return d
}

func (s *OrderService) buildAuthzContext(ctx context.Context, orderID uint64) (*authz.Context, error) {
	if orderID == 0 {
		userID, err := utils.GetUserIDFromCtx(ctx)
		if err != nil {
			return nil, apperrors.ErrUnauthorized
		}
		permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
		if err != nil {
			return nil, apperrors.ErrUnauthorized
		}
		actor, err := s.userRepo.FindUserByID(ctx, userID)
		if err != nil {
			return nil, apperrors.ErrUserNotFound
		}
		return &authz.Context{Actor: actor, Permissions: permissionsMap}, nil
	}
	t, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return s.buildAuthzContextWithTarget(ctx, t)
}

func (s *OrderService) buildAuthzContextWithTarget(ctx context.Context, t *entities.Order) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}
	perms, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	ctxAuth := &authz.Context{Actor: actor, Permissions: perms, Target: t}
	// Участник?
	was, _ := s.historyRepo.IsUserParticipant(ctx, t.ID, userID)
	ctxAuth.IsParticipant = (t.CreatorID == userID) || (t.ExecutorID != nil && *t.ExecutorID == userID) || was
	return ctxAuth, nil
}

// calculateMetrics вызывается внутри UpdateOrder, чтобы обновить статистику времени
// calculateMetrics вызывается внутри UpdateOrder, чтобы обновить статистику времени
// calculateMetrics вызывается внутри UpdateOrder, чтобы обновить статистику времени
func (s *OrderService) calculateMetrics(ctx context.Context, newOrder, oldOrder *entities.Order, dto dto.UpdateOrderDTO, actorID uint64, now time.Time) {
	newStatus, _ := s.statusRepo.FindStatus(ctx, newOrder.StatusID)
	newCode := ""
	if newStatus != nil && newStatus.Code != nil {
		newCode = *newStatus.Code
	}
	oldStatus, _ := s.statusRepo.FindStatus(ctx, oldOrder.StatusID)
	oldCode := ""
	if oldStatus != nil && oldStatus.Code != nil {
		oldCode = *oldStatus.Code
	}

	// === ФИКС ВРЕМЕННЫХ ЗОН: Используем локальное время базы (Asia/Tashkent) ===
	loc := time.Local

	// ✅ ИСПРАВЛЕНО: Конвертируем оба времени в Asia/Tashkent
	createdInTashkent := oldOrder.CreatedAt.In(loc)
	nowInTashkent := now.In(loc)

	// Вычисляем разницу в секундах
	diff := int64(nowInTashkent.Sub(createdInTashkent).Seconds())
	if diff < 0 {
		s.logger.Warn("⚠️ Отрицательная разница времени",
			zap.Time("created", createdInTashkent),
			zap.Time("now", nowInTashkent),
			zap.Int64("diff", diff))
		diff = 0
	}
	val := uint64(diff)

	s.logger.Info("📊 Расчёт метрик времени",
		zap.Uint64("order_id", newOrder.ID),
		zap.Time("created_at", createdInTashkent),
		zap.Time("now", nowInTashkent),
		zap.Uint64("diff_seconds", val),
		zap.Uint64("actor_id", actorID),
		zap.Uint64("creator_id", oldOrder.CreatorID),
		zap.String("old_status", oldCode),
		zap.String("new_status", newCode))

	// --- 1. ВРЕМЯ ПЕРВОГО ОТКЛИКА (Reaction Time) ---
	// ✅ ИСПРАВЛЕНО: Учитываем ТОЛЬКО действия исполнителя, а не создателя
	if oldOrder.FirstResponseTimeSeconds == nil || *oldOrder.FirstResponseTimeSeconds == 0 {
		// Проверяем, является ли актор исполнителем
		isExecutorAction := false

		// Случай 1: Исполнитель уже назначен и он делает действие
		if oldOrder.ExecutorID != nil && *oldOrder.ExecutorID == actorID {
			isExecutorAction = true
		}

		// Случай 2: Исполнитель только что назначен (делегация)
		if newOrder.ExecutorID != nil && *newOrder.ExecutorID == actorID {
			isExecutorAction = true
		}

		hasComment := dto.Comment != nil && strings.TrimSpace(*dto.Comment) != ""
		statusChanged := (newOrder.StatusID != oldOrder.StatusID)
		executorChanged := (oldOrder.ExecutorID == nil && newOrder.ExecutorID != nil) ||
			(oldOrder.ExecutorID != nil && newOrder.ExecutorID != nil && *oldOrder.ExecutorID != *newOrder.ExecutorID)

		// ✅ Отклик = ЛЮБОЕ изменение от исполнителя
		if isExecutorAction && (statusChanged || executorChanged || hasComment) {
			newOrder.FirstResponseTimeSeconds = &val
			s.logger.Info("✅ Записан первый отклик",
				zap.Uint64("order_id", newOrder.ID),
				zap.Uint64("seconds", val),
				zap.Bool("status_changed", statusChanged),
				zap.Bool("executor_changed", executorChanged),
				zap.Bool("has_comment", hasComment))
		} else {
			s.logger.Info("⏭️ Первый отклик не записан",
				zap.Uint64("order_id", newOrder.ID),
				zap.Bool("is_executor", isExecutorAction),
				zap.Bool("status_changed", statusChanged),
				zap.Bool("executor_changed", executorChanged),
				zap.Bool("has_comment", hasComment))
		}
	}

	// --- 2. ВРЕМЯ РЕШЕНИЯ (Resolution Time) ---
	if newCode == "CLOSED" {
		if oldOrder.ResolutionTimeSeconds == nil || *oldOrder.ResolutionTimeSeconds == 0 {
			newOrder.CompletedAt = &now
			newOrder.ResolutionTimeSeconds = &val

			// SLA FCR: Решено за 10 минут (600 секунд)
			if val <= 600 {
				t := true
				newOrder.IsFirstContactResolution = &t
			} else {
				f := false
				newOrder.IsFirstContactResolution = &f
			}

			// Если закрыли сразу без отклика, то отклик = решению
			if newOrder.FirstResponseTimeSeconds == nil || *newOrder.FirstResponseTimeSeconds == 0 {
				newOrder.FirstResponseTimeSeconds = &val
				s.logger.Info("📝 Первый отклик установлен равным времени решения",
					zap.Uint64("order_id", newOrder.ID),
					zap.Uint64("seconds", val))
			}

			s.logger.Info("✅ Заявка закрыта - записано время решения",
				zap.Uint64("order_id", newOrder.ID),
				zap.Uint64("resolution_seconds", val),
				zap.Bool("is_fcr", val <= 600))
		}
	}

	// --- 3. ПЕРЕОТКРЫТИЕ (Reopen) ---
	if oldCode == "CLOSED" && newCode != "CLOSED" {
		s.logger.Info("🔄 Переоткрытие заявки - сброс метрик",
			zap.Uint64("order_id", newOrder.ID),
			zap.String("old_status", oldCode),
			zap.String("new_status", newCode))

		newOrder.CompletedAt = nil
		newOrder.ResolutionTimeSeconds = nil
		newOrder.IsFirstContactResolution = nil
	}
}

// --- Utils wrappers ---
func (s *OrderService) addHistoryAndPublish(ctx context.Context, tx pgx.Tx, item *repositories.OrderHistoryItem, o entities.Order, a *entities.User) error {
	if err := s.historyRepo.CreateInTx(ctx, tx, item); err != nil {
		return err
	}
	s.eventBus.Publish(ctx, events.OrderHistoryCreatedEvent{HistoryItem: *item, Order: &o, Actor: a})
	return nil
}

func (s *OrderService) logHistoryEvent(ctx context.Context, tx pgx.Tx, oid uint64, actor *entities.User, evtType string, newVal, oldVal, comment *string, txID uuid.UUID, ord entities.Order) error {
	item := &repositories.OrderHistoryItem{
		OrderID: oid, UserID: actor.ID, EventType: evtType,
		NewValue: s.toNullStrPtr(newVal), OldValue: s.toNullStrPtr(oldVal),
		Comment:    s.toNullStrPtr(comment),
		TxID:       &txID,
		CreatedAt:  time.Now(),
		CreatorFio: s.toNullStr(actor.Fio),
	}
	return s.addHistoryAndPublish(ctx, tx, item, ord, actor)
}

func (s *OrderService) resolveUserName(ctx context.Context, uid *uint64) string {
	if uid == nil {
		return ""
	}
	u, _ := s.userRepo.FindUserByID(ctx, *uid)
	if u != nil {
		return u.Fio
	}
	return ""
}

func (s *OrderService) toNullStr(v string) sql.NullString {
	return sql.NullString{String: v, Valid: true}
}

func (s *OrderService) toNullStrPtr(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

func timePointersEqual(t1, t2 *time.Time) bool {
	if t1 == nil && t2 == nil {
		return true
	}
	if t1 == nil || t2 == nil {
		return false
	}
	return t1.Equal(*t2)
}

func (s *OrderService) GetValidationConfigForOrderType(ctx context.Context, orderTypeID uint64) (map[string]interface{}, error) {
	code, err := s.orderTypeRepo.FindCodeByID(ctx, orderTypeID)
	if err != nil {
		return nil, err
	}
	if rules, ok := ValidationRegistry[code]; ok {
		m := make(map[string]interface{})
		for _, r := range rules {
			m[r.FieldName] = r.ErrorMessage
		}
		return m, nil
	}
	return map[string]interface{}{}, nil
}

func (s *OrderService) validateOrderRules(ctx context.Context, d dto.CreateOrderDTO) error {
	if d.OrderTypeID == nil {
		return nil
	}
	code, err := s.orderTypeRepo.FindCodeByID(ctx, *d.OrderTypeID)
	if err != nil {
		return nil
	}

	// 1. Проверяем поля для оборудования
	if rules, ok := OrderValidationRules[code]; ok {
		for _, field := range rules {
			if !s.checkFieldPresence(d, field) {
				return apperrors.NewBadRequestError(fmt.Sprintf("Поле %s обязательно", field))
			}
		}
	}

	if code != "EQUIPMENT" {
		if !s.checkFieldPresence(d, "comment") {
			return apperrors.NewBadRequestError("Для данного типа заявки необходимо заполнить поле 'Комментарий'.")
		}
	}

	return nil
}

func (s *OrderService) checkFieldPresence(d dto.CreateOrderDTO, field string) bool {
	switch field {
	case "equipment_id":
		return d.EquipmentID != nil
	case "executor_id":
		return d.ExecutorID != nil
	case "priority_id":
		return d.PriorityID != nil
	case "otdel_id":
		return d.OtdelID != nil
	case "comment":
		return d.Comment != nil && strings.TrimSpace(*d.Comment) != ""
	default:
		return true
	}
}

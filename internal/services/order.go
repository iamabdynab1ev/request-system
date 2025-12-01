package services

import (
	"context"
	"database/sql"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/aarondl/null/v8"
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

var ValidationRegistry = map[string][]ValidationRule{
	"EQUIPMENT": {
		{FieldName: "equipment_id", ErrorMessage: "Пожалуйста, укажите оборудование."},
		{FieldName: "equipment_type_id", ErrorMessage: "Пожалуйста, выберите тип оборудования."},
		{FieldName: "priority_id", ErrorMessage: "Пожалуйста, укажите приоритет."},
	},
}

type OrderServiceInterface interface {
	CreateOrder(ctx context.Context, createDTO dto.CreateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	GetOrders(ctx context.Context, filter types.Filter, onlyParticipant bool) (*dto.OrderListResponseDTO, error)
	FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error)
	UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	DeleteOrder(ctx context.Context, orderID uint64) error
	GetStatusByID(ctx context.Context, id uint64) (*entities.Status, error)
	GetPriorityByID(ctx context.Context, id uint64) (*entities.Priority, error)
	GetValidationConfigForOrderType(ctx context.Context, orderTypeID uint64) (map[string]interface{}, error)
	FindOrderByIDForTelegram(ctx context.Context, userID uint64, orderID uint64) (*entities.Order, error)
	GetUserStats(ctx context.Context, userID uint64) (*types.UserOrderStats, error)
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

func (s *OrderService) GetUserStats(ctx context.Context, userID uint64) (*types.UserOrderStats, error) {
	// Статистика за последние 30 дней
	fromDate := time.Now().AddDate(0, 0, -30)
	return s.orderRepo.GetUserOrderStats(ctx, userID, fromDate)
}

func (s *OrderService) addHistoryAndPublish(ctx context.Context, tx pgx.Tx, item *repositories.OrderHistoryItem, order entities.Order, actor *entities.User) error {
	if err := s.historyRepo.CreateInTx(ctx, tx, item); err != nil {
		s.logger.Error("Не удалось создать запись в истории", zap.Error(err),
			zap.Uint64("orderID", item.OrderID), zap.String("eventType", item.EventType))
		return err
	}
	s.eventBus.Publish(ctx, events.OrderHistoryCreatedEvent{
		HistoryItem: *item,
		Order:       &order,
		Actor:       actor,
	})
	return nil
}

func (s *OrderService) GetStatusByID(ctx context.Context, id uint64) (*entities.Status, error) {
	return s.statusRepo.FindStatus(ctx, id)
}

func (s *OrderService) GetPriorityByID(ctx context.Context, id uint64) (*entities.Priority, error) {
	return s.priorityRepo.FindByID(ctx, id)
}

func (s *OrderService) GetOrders(ctx context.Context, filter types.Filter, onlyParticipant bool) (*dto.OrderListResponseDTO, error) {
	// 1. Получаем всю информацию об акторе (кто запрашивает) и его правах
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		s.logger.Error("Не удалось получить карту прав из контекста", zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	// 2. Проверяем базовое право на просмотр заявок
	authContext := authz.Context{Actor: actor, Permissions: permissionsMap}
	if !authz.CanDo(authz.OrdersView, authContext) {
		s.logger.Warn("Попытка доступа к списку заявок без права order:view", zap.Uint64("userID", userID))
		return nil, apperrors.ErrForbidden
	}

	// 3. <<-- ИЗМЕНЕНИЕ: Используем sq.And для всех условий -->>
	securityBuilder := sq.And{}

	// 4. Строим ДИНАМИЧЕСКИЙ фильтр безопасности, если у пользователя нет глобального доступа
	if !authContext.HasPermission(authz.ScopeAll) && !authContext.HasPermission(authz.ScopeAllView) {

		scopeConditions := sq.Or{}

		if authContext.HasPermission(authz.ScopeDepartment) && actor.DepartmentID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.department_id": *actor.DepartmentID})
		}
		if authContext.HasPermission(authz.ScopeBranch) && actor.BranchID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.branch_id": *actor.BranchID})
		}
		if authContext.HasPermission(authz.ScopeOtdel) && actor.OtdelID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.otdel_id": *actor.OtdelID})
		}
		if authContext.HasPermission(authz.ScopeOffice) && actor.OfficeID != nil {
			scopeConditions = append(scopeConditions, sq.Eq{"o.office_id": *actor.OfficeID})
		}

		if authContext.HasPermission(authz.ScopeOwn) {
			scopeConditions = append(scopeConditions, sq.Eq{"o.user_id": actor.ID})
			scopeConditions = append(scopeConditions, sq.Eq{"o.executor_id": actor.ID})
			scopeConditions = append(scopeConditions, sq.Expr("o.id IN (SELECT DISTINCT order_id FROM order_history WHERE user_id = ?)", actor.ID))
		}

		if len(scopeConditions) > 0 {
			securityBuilder = append(securityBuilder, scopeConditions)
		} else {
			s.logger.Warn("У пользователя нет ни одного scope-права для просмотра заявок, возвращен пустой список.", zap.Uint64("userID", userID))
			return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
		}
	}

	// 5. <<-- ИЗМЕНЕНИЕ: ДОБАВЛЯЕМ УСЛОВИЕ УЧАСТИЯ, если флаг true -->>
	if onlyParticipant {
		participantCondition := sq.Or{
			sq.Eq{"o.user_id": actor.ID},     // Я создатель
			sq.Eq{"o.executor_id": actor.ID}, // Я исполнитель
			// Это условие для истории теперь становится частью "или", а не обязательным.
			// Оно может дублировать `ScopeOwn`, но здесь оно в контексте `или`.
			sq.Expr("o.id IN (SELECT DISTINCT order_id FROM order_history WHERE user_id = ?)", actor.ID),
		}
		securityBuilder = append(securityBuilder, participantCondition)
	}

	// 6. Вызываем репозиторий с ГОТОВЫМ объектом squirrel
	orders, totalCount, err := s.orderRepo.GetOrders(ctx, filter, securityBuilder)
	if err != nil {
		s.logger.Error("Ошибка получения списка заявок из репозитория", zap.Error(err))
		return nil, err
	}
	if len(orders) == 0 {
		return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
	}

	// 7. Обогащаем данные для ответа (эта часть остается без изменений)
	userIDsMap := make(map[uint64]struct{})
	orderIDs := make([]uint64, len(orders))
	for i, order := range orders {
		orderIDs[i] = order.ID
		userIDsMap[order.CreatorID] = struct{}{}
		if order.ExecutorID != nil {
			userIDsMap[*order.ExecutorID] = struct{}{}
		}
	}
	userIDs := make([]uint64, 0, len(userIDsMap))
	for id := range userIDsMap {
		userIDs = append(userIDs, id)
	}

	usersMap, _ := s.userRepo.FindUsersByIDs(ctx, userIDs)
	attachmentsMap, _ := s.attachRepo.FindAttachmentsByOrderIDs(ctx, orderIDs)

	dtos := make([]dto.OrderResponseDTO, len(orders))
	for i := range orders {
		order := &orders[i]
		var creator, executor *entities.User

		if c, ok := usersMap[order.CreatorID]; ok {
			creator = &c
		}

		if order.ExecutorID != nil {
			if exec, ok := usersMap[*order.ExecutorID]; ok {
				executor = &exec
			}
		}

		attachments := attachmentsMap[order.ID]
		dtos[i] = *buildOrderResponse(order, creator, executor, attachments)
	}

	return &dto.OrderListResponseDTO{List: dtos, TotalCount: totalCount}, nil
}

func (s *OrderService) FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersView, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	order := authContext.Target.(*entities.Order)

	creator, _ := s.userRepo.FindUserByID(ctx, order.CreatorID)
	var executor *entities.User
	if order.ExecutorID != nil {
		exec, err := s.userRepo.FindUserByID(ctx, *order.ExecutorID)
		if err != nil {
			s.logger.Warn("Executor not found for response", zap.Uint64("userID", *order.ExecutorID), zap.Error(err))
			executor = nil
		} else {
			executor = exec
		}
	}
	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, order.ID, 50, 0)

	return buildOrderResponse(order, creator, executor, attachments), nil
}

func (s *OrderService) CreateOrder(ctx context.Context, createDTO dto.CreateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	// 1. Авторизация
	authContext, err := s.buildAuthzContext(ctx, 0)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	// 2. Валидация типа
	if createDTO.OrderTypeID.Valid {
		orderTypeCode, err := s.orderTypeRepo.FindCodeByID(ctx, uint64(createDTO.OrderTypeID.Int))
		if err == nil {
			if _, ok := ValidationRegistry[orderTypeCode]; ok {
				// Ваша логика валидации...
			}
		}
	}

	// 3. ВАЛИДАЦИЯ: Либо Деп, Либо Филиал (ИСПРАВЛЕННЫЙ ВЫЗОВ ОШИБКИ)
	hasDept := createDTO.DepartmentID.Valid && createDTO.DepartmentID.Int > 0
	hasBranch := createDTO.BranchID.Valid && createDTO.BranchID.Int > 0

	if !hasDept && !hasBranch {
		// <-- ИСПРАВЛЕНО ТУТ (4 аргумента)
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Необходимо выбрать либо Департамент, либо Филиал.", nil, nil)
	}

	var finalOrderID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New()
		now := time.Now()

		// ПОДГОТОВКА RuleEngine Context
		ruleCtxDeptID := uint64(0)
		if hasDept {
			ruleCtxDeptID = uint64(createDTO.DepartmentID.Int)
		}

		orderCtx := OrderContext{
			OrderTypeID:  uint64(createDTO.OrderTypeID.Int),
			DepartmentID: ruleCtxDeptID,
			OtdelID:      utils.NullIntToUint64Ptr(createDTO.OtdelID),
			BranchID:     utils.NullIntToUint64Ptr(createDTO.BranchID),
			OfficeID:     utils.NullIntToUint64Ptr(createDTO.OfficeID),
		}

		var userSelectedExecutorID *uint64
		if createDTO.ExecutorID.Valid {
			v := uint64(createDTO.ExecutorID.Int)
			userSelectedExecutorID = &v
		}

		if userSelectedExecutorID != nil && !authz.CanDo(authz.OrdersCreateExecutorID, *authContext) {
			return apperrors.ErrForbidden
		}

		result, err := s.ruleEngine.ResolveExecutor(ctx, tx, orderCtx, userSelectedExecutorID)
		if err != nil {
			return err
		}

		status, err := s.statusRepo.FindByCodeInTx(ctx, tx, "OPEN")
		if err != nil || status == nil {
			s.logger.Error("Статус 'OPEN' не найден", zap.Error(err))
			return apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации: статус 'OPEN' не найден", err, nil)
		}

		var duration *time.Time
		if createDTO.Duration.Valid {
			duration = &createDTO.Duration.Time
		}

		// ПОДГОТОВКА ENTITY (pointer for Department)
		var dbDeptIDPtr *uint64
		if hasDept {
			v := uint64(createDTO.DepartmentID.Int)
			dbDeptIDPtr = &v
		}

		orderEntity := &entities.Order{
			OrderTypeID:     utils.NullIntToUint64Ptr(createDTO.OrderTypeID),
			Name:            createDTO.Name,
			Address:         utils.NullStringToStrPtr(createDTO.Address),
			DepartmentID:    dbDeptIDPtr, // pointer
			OtdelID:         utils.NullIntToUint64Ptr(createDTO.OtdelID),
			BranchID:        utils.NullIntToUint64Ptr(createDTO.BranchID),
			OfficeID:        utils.NullIntToUint64Ptr(createDTO.OfficeID),
			EquipmentID:     utils.NullIntToUint64Ptr(createDTO.EquipmentID),
			EquipmentTypeID: utils.NullIntToUint64Ptr(createDTO.EquipmentTypeID),
			StatusID:        uint64(status.ID),
			PriorityID:      utils.NullIntToUint64Ptr(createDTO.PriorityID),
			CreatorID:       authContext.Actor.ID,
			ExecutorID:      &result.Executor.ID,
			Duration:        duration,
		}

		orderID, err := s.orderRepo.Create(ctx, tx, orderEntity)
		if err != nil {
			s.logger.Error("Ошибка при создании заявки в БД", zap.Error(err))
			return err
		}
		finalOrderID = orderID
		orderEntity.ID = orderID

		itemCreate := &repositories.OrderHistoryItem{
			OrderID:    orderID,
			UserID:     authContext.Actor.ID,
			EventType:  "CREATE",
			NewValue:   sql.NullString{String: orderEntity.Name, Valid: true},
			TxID:       &txID,
			CreatedAt:  now,
			CreatorFio: sql.NullString{String: authContext.Actor.Fio, Valid: true},
		}
		if err := s.addHistoryAndPublish(ctx, tx, itemCreate, *orderEntity, authContext.Actor); err != nil {
			return err
		}

		delegationText := "Назначено на: " + result.Executor.Fio
		itemDelegation := &repositories.OrderHistoryItem{
			OrderID:      orderID,
			UserID:       authContext.Actor.ID,
			EventType:    "DELEGATION",
			NewValue:     sql.NullString{String: fmt.Sprintf("%d", result.Executor.ID), Valid: true},
			Comment:      sql.NullString{String: delegationText, Valid: true},
			TxID:         &txID,
			CreatedAt:    now,
			DelegatorFio: sql.NullString{String: authContext.Actor.Fio, Valid: true},
			ExecutorFio:  sql.NullString{String: result.Executor.Fio, Valid: true},
		}
		if err := s.addHistoryAndPublish(ctx, tx, itemDelegation, *orderEntity, authContext.Actor); err != nil {
			return err
		}

		itemStatus := &repositories.OrderHistoryItem{
			OrderID: orderID, UserID: authContext.Actor.ID, EventType: "STATUS_CHANGE",
			NewValue: sql.NullString{String: fmt.Sprintf("%d", status.ID), Valid: true}, TxID: &txID, CreatedAt: now,
		}
		if err := s.addHistoryAndPublish(ctx, tx, itemStatus, *orderEntity, authContext.Actor); err != nil {
			return err
		}

		if orderEntity.PriorityID != nil {
			itemPriority := &repositories.OrderHistoryItem{
				OrderID: orderID, UserID: authContext.Actor.ID, EventType: "PRIORITY_CHANGE",
				NewValue: sql.NullString{String: fmt.Sprintf("%d", *orderEntity.PriorityID), Valid: true},
				TxID:     &txID, CreatedAt: now,
			}
			if err := s.addHistoryAndPublish(ctx, tx, itemPriority, *orderEntity, authContext.Actor); err != nil {
				return err
			}
		}

		if createDTO.Comment.Valid {
			itemComment := &repositories.OrderHistoryItem{
				OrderID: orderID, UserID: authContext.Actor.ID, EventType: "COMMENT",
				Comment: sql.NullString{String: createDTO.Comment.String, Valid: true}, TxID: &txID, CreatedAt: now,
			}
			if err := s.addHistoryAndPublish(ctx, tx, itemComment, *orderEntity, authContext.Actor); err != nil {
				return err
			}
		}

		if orderEntity.Duration != nil {
			itemDuration := &repositories.OrderHistoryItem{
				OrderID: orderID, UserID: authContext.Actor.ID, EventType: "DURATION_CHANGE",
				NewValue: utils.TimeToNullString(orderEntity.Duration), TxID: &txID, CreatedAt: now,
			}
			if err := s.addHistoryAndPublish(ctx, tx, itemDuration, *orderEntity, authContext.Actor); err != nil {
				return err
			}
		}

		if file != nil {
			if _, err := s.attachFileToOrderInTx(ctx, tx, orderID, authContext.Actor.ID, file, &txID, orderEntity); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.FindOrderByID(ctx, finalOrderID)
}

func (s *OrderService) UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	// 1. Авторизация
	targetOrder, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	authContext, err := s.buildAuthzContextWithTarget(ctx, targetOrder)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	// 2. Логика обновления в транзакции
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New()
		now := time.Now()
		updated := false
		actorID := authContext.Actor.ID

		// === УЛУЧШЕННЫЙ БЛОК: РАСЧЕТ FRT и FCR ===
		// Логика сработает только один раз, при первом осмысленном действии НЕ создателя заявки.
		// Добавлена НАДЕЖНАЯ проверка, что CreatedAt является корректной датой.
		isMeaningfulAction := updateDTO.StatusID != nil || updateDTO.Comment.Valid || updateDTO.ExecutorID != nil
		if targetOrder.FirstResponseTimeSeconds == nil && actorID != targetOrder.CreatorID && isMeaningfulAction && !targetOrder.CreatedAt.IsZero() {
			// 1. Рассчитываем время первого ответа (First Response Time)
			responseTime := uint64(now.Sub(targetOrder.CreatedAt).Seconds())
			targetOrder.FirstResponseTimeSeconds = &responseTime
			updated = true

			// 2. Проверяем, было ли это решение при первом контакте (First Contact Resolution)
			isResolvedNow := false
			if updateDTO.StatusID != nil {
				// Проверяем, является ли новый статус закрывающим ("Выполнено" или "Закрыто")
				newStatus, err := s.statusRepo.FindByIDInTx(ctx, tx, uint64(*updateDTO.StatusID))
				if err == nil && newStatus.Code != nil {
					code := strings.ToUpper(*newStatus.Code)
					if code == "COMPLETED" || code == "CLOSED" {
						isResolvedNow = true
					}
				}
			}
			// Устанавливаем флаг FCR. Если заявка решена этим первым действием - true, иначе - false.
			// Это значение фиксируется и больше не меняется.
			targetOrder.IsFirstContactResolution = &isResolvedNow
			s.logger.Info("Рассчитаны метрики первого ответа (FRT и FCR)",
				zap.Uint64("orderID", orderID),
				zap.Uint64("actorID", actorID),
				zap.Uint64("FRT_seconds", responseTime),
				zap.Bool("FCR", isResolvedNow),
			)
		}

		// --- I. ПРИОРИТЕТ: ПРОВЕРЯЕМ СМЕНУ ПОДРАЗДЕЛЕНИЯ ---
		var err error
		deptChanged := false
		if updateDTO.DepartmentID != nil {
			deptChanged, err = s.updateDepartment(ctx, tx, *authContext, null.IntFrom(int(*updateDTO.DepartmentID)), targetOrder, &txID, now)
			if err != nil {
				return err
			}
			if deptChanged {
				updated = true
			}
		}

		otdelChanged := false
		if !deptChanged && updateDTO.OtdelID != nil {
			otdelChanged, err = s.updateOtdel(ctx, tx, *authContext, null.IntFrom(int(*updateDTO.OtdelID)), targetOrder, &txID, now)
			if err != nil {
				return err
			}
			if otdelChanged {
				updated = true
			}
		}

		orgStructureChanged := deptChanged || otdelChanged

		if orgStructureChanged {
			s.logger.Info("Подразделение изменено, переназначение...", zap.Uint64("orderID", orderID))

			currentDeptID := uint64(0)
			if targetOrder.DepartmentID != nil {
				currentDeptID = *targetOrder.DepartmentID
			}

			orderCtx := OrderContext{
				DepartmentID: currentDeptID,
				OtdelID:      targetOrder.OtdelID,
				OrderTypeID:  0,
				BranchID:     targetOrder.BranchID,
				OfficeID:     targetOrder.OfficeID,
			}

			result, err := s.ruleEngine.ResolveExecutor(ctx, tx, orderCtx, nil)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Не удалось найти исполнителя в новом подразделении.", err, nil)
			}
			if err := s.updateExecutor(ctx, tx, *authContext, null.IntFrom(int(result.Executor.ID)), targetOrder, txID, now); err != nil {
				return err
			}
		} else if updateDTO.ExecutorID != nil {
			if err := s.updateExecutor(ctx, tx, *authContext, null.IntFrom(int(*updateDTO.ExecutorID)), targetOrder, txID, now); err != nil {
				return err
			}
			updated = true
		}
		// --- II. ОБНОВЛЕНИЕ ОСТАЛЬНЫХ ПОЛЕЙ ---
		if updateDTO.Comment.Valid {
			item := &repositories.OrderHistoryItem{
				OrderID: orderID, UserID: authContext.Actor.ID, EventType: "COMMENT",
				Comment: sql.NullString{String: updateDTO.Comment.String, Valid: true}, TxID: &txID, CreatedAt: now,
			}
			if err := s.addHistoryAndPublish(ctx, tx, item, *targetOrder, authContext.Actor); err != nil {
				return err
			}
			updated = true
		}
		if updateDTO.StatusID != nil {
			if err := s.updateStatus(ctx, tx, *authContext, null.IntFrom(int(*updateDTO.StatusID)), targetOrder, txID, now); err != nil {
				return err
			}
			updated = true
		}
		if updateDTO.PriorityID != nil {
			if err := s.updatePriority(ctx, tx, *authContext, null.IntFrom(int(*updateDTO.PriorityID)), targetOrder, &txID, now); err != nil {
				return err
			}
			updated = true
		}
		if updateDTO.Duration.Valid {
			if err := s.updateDuration(ctx, tx, *authContext, updateDTO.Duration, targetOrder, txID, now); err != nil {
				return err
			}
			updated = true
		}

		if file != nil {
			if _, err := s.attachFileToOrderInTx(ctx, tx, orderID, authContext.Actor.ID, file, &txID, targetOrder); err != nil {
				return err
			}
			updated = true
		}
		if !updated {
			return apperrors.ErrNoChanges
		}

		targetOrder.UpdatedAt = now
		return s.orderRepo.Update(ctx, tx, targetOrder)
	})
	if err != nil {
		return nil, err
	}
	return s.FindOrderByID(ctx, orderID)
}

func (s *OrderService) updateStatus(ctx context.Context, tx pgx.Tx, authCtx authz.Context, statusID null.Int, order *entities.Order, txID uuid.UUID, now time.Time) error {
	if !statusID.Valid {
		return nil
	}

	oldStatusID, newStatusID := order.StatusID, uint64(statusID.Int)
	if oldStatusID == newStatusID {
		return nil
	}

	newStatus, err := s.statusRepo.FindByIDInTx(ctx, tx, newStatusID)
	if err != nil || newStatus == nil {
		return apperrors.NewHttpError(http.StatusBadRequest, "Указанный статус не найден", err, nil)
	}

	if newStatus.Code != nil && strings.ToUpper(*newStatus.Code) == "CLOSED" {
		if order.CompletedAt == nil {
			order.CompletedAt = &now
		}
	} else {
		order.CompletedAt = nil
	}
	order.StatusID = newStatusID

	historyItem := &repositories.OrderHistoryItem{
		OrderID: order.ID, UserID: authCtx.Actor.ID, EventType: "STATUS_CHANGE",
		OldValue: utils.Uint64ToNullString(oldStatusID), NewValue: utils.Uint64ToNullString(newStatusID),
		TxID: &txID, CreatedAt: now,
	}

	err = s.historyRepo.CreateInTx(ctx, tx, historyItem)
	if err == nil {
		s.eventBus.Publish(ctx, events.OrderHistoryCreatedEvent{
			HistoryItem: *historyItem,
			Order:       order,
			Actor:       authCtx.Actor,
		})
	}

	return err
}

func (s *OrderService) updateExecutor(ctx context.Context, tx pgx.Tx, authCtx authz.Context, executorID null.Int, order *entities.Order, txID uuid.UUID, now time.Time) error {
	if !executorID.Valid {
		return nil
	}

	newExecutorID := uint64(executorID.Int)
	if order.ExecutorID != nil && *order.ExecutorID == newExecutorID {
		return nil
	}

	newExec, err := s.userRepo.FindUserByIDInTx(ctx, tx, newExecutorID)
	if err != nil || newExec == nil {
		return apperrors.NewHttpError(http.StatusBadRequest, "Новый исполнитель не найден", err, nil)
	}

	oldExecutorIDPtr := order.ExecutorID
	order.ExecutorID = &newExecutorID
	delegationText := "Переназначено на: " + newExec.Fio

	historyItem := &repositories.OrderHistoryItem{
		OrderID:   order.ID,
		UserID:    authCtx.Actor.ID,
		EventType: "DELEGATION",
		OldValue:  utils.Uint64PtrToNullString(oldExecutorIDPtr), NewValue: utils.Uint64PtrToNullString(order.ExecutorID),
		Comment: sql.NullString{String: delegationText, Valid: true}, TxID: &txID, CreatedAt: now,
		DelegatorFio: sql.NullString{String: authCtx.Actor.Fio, Valid: true}, // <-- ДОБАВЛЕНО
		ExecutorFio:  sql.NullString{String: newExec.Fio, Valid: true},       // <-- ДОБАВЛЕНО
	}

	err = s.historyRepo.CreateInTx(ctx, tx, historyItem)
	if err == nil {
		s.eventBus.Publish(ctx, events.OrderHistoryCreatedEvent{
			HistoryItem: *historyItem, Order: order, Actor: authCtx.Actor,
		})
	}
	return err
}

func (s *OrderService) updateDuration(ctx context.Context, tx pgx.Tx, authCtx authz.Context, duration null.Time, order *entities.Order, txID uuid.UUID, now time.Time) error {
	var newDuration *time.Time
	if duration.Valid {
		newDuration = &duration.Time
	}

	// Сравниваем старое и новое значение, чтобы не создавать лишних записей
	isChanged := (order.Duration == nil && newDuration != nil) || (order.Duration != nil && newDuration == nil) || (order.Duration != nil && newDuration != nil && !order.Duration.Equal(*newDuration))

	if !isChanged {
		return nil
	}

	oldDuration := order.Duration
	order.Duration = newDuration

	historyItem := &repositories.OrderHistoryItem{
		OrderID: order.ID, UserID: authCtx.Actor.ID, EventType: "DURATION_CHANGE",
		OldValue: utils.TimeToNullString(oldDuration), NewValue: utils.TimeToNullString(order.Duration),
		TxID: &txID, CreatedAt: now,
	}

	err := s.historyRepo.CreateInTx(ctx, tx, historyItem)
	if err == nil {
		s.eventBus.Publish(ctx, events.OrderHistoryCreatedEvent{
			HistoryItem: *historyItem, Order: order, Actor: authCtx.Actor,
		})
	}
	return err
}

func (s *OrderService) updateDepartment(ctx context.Context, tx pgx.Tx, authCtx authz.Context, deptID null.Int, order *entities.Order, txID *uuid.UUID, now time.Time) (bool, error) {
	if !deptID.Valid {
		return false, nil
	}

	newDeptID := uint64(deptID.Int)

	// Сравниваем: если текущий nil ИЛИ значения разные -> меняем
	if order.DepartmentID != nil && *order.DepartmentID == newDeptID {
		return false, nil
	}

	oldDeptIDPtr := order.DepartmentID
	order.DepartmentID = &newDeptID // Присваиваем указатель
	order.OtdelID = nil             // Сбрасываем отдел

	historyItem := &repositories.OrderHistoryItem{
		OrderID:   order.ID,
		UserID:    authCtx.Actor.ID,
		EventType: "DEPARTMENT_CHANGE",
		OldValue:  utils.Uint64PtrToNullString(oldDeptIDPtr),
		NewValue:  utils.Uint64PtrToNullString(order.DepartmentID),
		TxID:      txID, CreatedAt: now,
	}

	err := s.historyRepo.CreateInTx(ctx, tx, historyItem)
	if err == nil {
		s.eventBus.Publish(ctx, events.OrderHistoryCreatedEvent{
			HistoryItem: *historyItem, Order: order, Actor: authCtx.Actor,
		})
	}

	return err == nil, err
}

func (s *OrderService) updateOtdel(ctx context.Context, tx pgx.Tx, authCtx authz.Context, otdelID null.Int, order *entities.Order, txID *uuid.UUID, now time.Time) (bool, error) {
	if !otdelID.Valid {
		return false, nil
	}

	newOtdelID := uint64(otdelID.Int)
	if order.OtdelID != nil && *order.OtdelID == newOtdelID {
		return false, nil
	}

	oldOtdelIDPtr := order.OtdelID
	order.OtdelID = &newOtdelID

	historyItem := &repositories.OrderHistoryItem{
		OrderID: order.ID, UserID: authCtx.Actor.ID, EventType: "OTDEL_CHANGE",
		OldValue: utils.Uint64PtrToNullString(oldOtdelIDPtr), NewValue: utils.Uint64PtrToNullString(order.OtdelID),
		TxID: txID, CreatedAt: now,
	}

	err := s.historyRepo.CreateInTx(ctx, tx, historyItem)
	if err == nil {
		s.eventBus.Publish(ctx, events.OrderHistoryCreatedEvent{
			HistoryItem: *historyItem, Order: order, Actor: authCtx.Actor,
		})
	}

	return err == nil, err
}

func (s *OrderService) updatePriority(ctx context.Context, tx pgx.Tx, authCtx authz.Context, priorityID null.Int, order *entities.Order, txID *uuid.UUID, now time.Time) error {
	if !priorityID.Valid {
		return nil
	}

	newVal := uint64(priorityID.Int)
	if order.PriorityID != nil && *order.PriorityID == newVal {
		return nil
	}

	oldVal := order.PriorityID
	order.PriorityID = &newVal

	historyItem := &repositories.OrderHistoryItem{
		OrderID: order.ID, UserID: authCtx.Actor.ID, EventType: "PRIORITY_CHANGE",
		OldValue: utils.Uint64PtrToNullString(oldVal), NewValue: utils.Uint64PtrToNullString(order.PriorityID),
		TxID: txID, CreatedAt: now,
	}

	err := s.historyRepo.CreateInTx(ctx, tx, historyItem)
	if err == nil {
		s.eventBus.Publish(ctx, events.OrderHistoryCreatedEvent{
			HistoryItem: *historyItem, Order: order, Actor: authCtx.Actor,
		})
	}
	return err
}

func (s *OrderService) DeleteOrder(ctx context.Context, orderID uint64) error {
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.OrdersDelete, *authContext) {
		return apperrors.ErrForbidden
	}
	return s.orderRepo.DeleteOrder(ctx, uint64(orderID))
}

func (s *OrderService) GetValidationConfigForOrderType(ctx context.Context, orderTypeID uint64) (map[string]interface{}, error) {
	orderTypeCode, err := s.orderTypeRepo.FindCodeByID(ctx, orderTypeID)
	if err != nil {
		return nil, err
	}
	rules, ok := ValidationRegistry[orderTypeCode]
	if !ok {
		return map[string]interface{}{}, nil
	}
	config := make(map[string]interface{})
	for _, rule := range rules {
		config[rule.FieldName] = rule.ErrorMessage
	}
	return config, nil
}

func (s *OrderService) buildAuthzContext(ctx context.Context, orderID uint64) (*authz.Context, error) {
	if orderID == 0 {
		userID, _ := utils.GetUserIDFromCtx(ctx)
		permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
		actor, err := s.userRepo.FindUserByID(ctx, userID)
		if err != nil {
			return nil, apperrors.ErrUserNotFound
		}
		return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: nil}, nil
	}

	targetOrder, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return s.buildAuthzContextWithTarget(ctx, targetOrder)
}

func (s *OrderService) buildAuthzContextWithTarget(ctx context.Context, target *entities.Order) (*authz.Context, error) {
	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	authContext := &authz.Context{Actor: actor, Permissions: permissionsMap, Target: target}

	wasInHistory, _ := s.historyRepo.IsUserParticipant(ctx, target.ID, userID)
	isCreator := target.CreatorID == userID
	isExecutor := target.ExecutorID != nil && *target.ExecutorID == userID
	authContext.IsParticipant = isCreator || isExecutor || wasInHistory

	return authContext, nil
}

func buildOrderResponse(order *entities.Order, creator *entities.User, executor *entities.User, attachments []entities.Attachment) *dto.OrderResponseDTO {
	var attachDTOs []dto.AttachmentResponseDTO
	for _, attach := range attachments {
		attachDTOs = append(attachDTOs, dto.AttachmentResponseDTO{
			ID:       attach.ID,
			FileName: attach.FileName,
			URL:      "/uploads/" + attach.FilePath,
		})
	}

	var execDTO dto.ShortUserDTO
	if executor != nil {
		execDTO = dto.ShortUserDTO{
			ID:  executor.ID,
			Fio: executor.Fio,
		}
	}

	response := &dto.OrderResponseDTO{
		ID:                         order.ID,
		Name:                       order.Name,
		OrderTypeID:                utils.Uint64PtrToNullInt(order.OrderTypeID),
		Address:                    utils.StrPtrToNull(order.Address),
		Creator:                    dto.ShortUserDTO{ID: creator.ID, Fio: creator.Fio},
		Executor:                   execDTO,
		DepartmentID:               utils.Uint64PtrToNullInt(order.DepartmentID),
		OtdelID:                    utils.Uint64PtrToNullInt(order.OtdelID),
		BranchID:                   utils.Uint64PtrToNullInt(order.BranchID),
		OfficeID:                   utils.Uint64PtrToNullInt(order.OfficeID),
		EquipmentID:                utils.Uint64PtrToNullInt(order.EquipmentID),
		EquipmentTypeID:            utils.Uint64PtrToNullInt(order.EquipmentTypeID),
		StatusID:                   order.StatusID,
		PriorityID:                 utils.Uint64PtrToNullInt(order.PriorityID),
		Attachments:                attachDTOs,
		Duration:                   utils.TimeToNull(order.Duration),
		CreatedAt:                  order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:                  order.UpdatedAt.Format(time.RFC3339),
		CompletedAt:                utils.TimeToNull(order.CompletedAt),
		ResolutionTimeSeconds:      utils.Uint64PtrToNullInt(order.ResolutionTimeSeconds),
		ResolutionTimeFormatted:    "",
		FirstResponseTimeSeconds:   utils.Uint64PtrToNullInt(order.FirstResponseTimeSeconds),
		FirstResponseTimeFormatted: "",
	}

	if order.ResolutionTimeSeconds != nil {
		response.ResolutionTimeFormatted = utils.FormatSecondsToHumanReadable(*order.ResolutionTimeSeconds)
	}
	if order.FirstResponseTimeSeconds != nil {
		response.FirstResponseTimeFormatted = utils.FormatSecondsToHumanReadable(*order.FirstResponseTimeSeconds)
	}

	return response
}

func (s *OrderService) attachFileToOrderInTx(ctx context.Context, tx pgx.Tx, orderID, userID uint64, file *multipart.FileHeader, txID *uuid.UUID, order *entities.Order) (uint64, error) {
	// Шаг 1: Сохраняем файл физически (ваш код)
	reader, err := file.Open()
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	filePath, err := s.fileStorage.Save(reader, file.Filename, "orders")
	if err != nil {
		return 0, err
	}

	// Шаг 2: Создаем запись о вложении в базе (ваш код)
	now := time.Now()
	attach := &entities.Attachment{
		OrderID:   orderID,
		UserID:    userID,
		FileName:  file.Filename,
		FilePath:  filePath,
		FileType:  file.Header.Get("Content-Type"),
		FileSize:  file.Size,
		CreatedAt: now,
	}
	attachID, err := s.attachRepo.CreateInTx(ctx, tx, attach)
	if err != nil {
		return 0, err
	}

	// Шаг 3: Создаем запись в истории (ваш код)
	historyItem := &repositories.OrderHistoryItem{
		OrderID:      orderID,
		UserID:       userID,
		EventType:    "ATTACHMENT_ADD",
		NewValue:     sql.NullString{String: file.Filename, Valid: true},
		Attachment:   attach, // Вложение уже здесь, так что для Listener'а все есть
		AttachmentID: sql.NullInt64{Int64: int64(attachID), Valid: true},
		CreatedAt:    now,
		TxID:         txID,
	}

	actor, err := s.userRepo.FindUserByIDInTx(ctx, tx, userID)
	if err != nil {
		return 0, err
	}

	// Здесь ошибка 'undefined: order' теперь тоже исправлена, так как order приходит в аргументах.
	if err := s.addHistoryAndPublish(ctx, tx, historyItem, *order, actor); err != nil {
		return 0, err
	}

	return attachID, nil
}

func (s *OrderService) FindOrderByIDForTelegram(ctx context.Context, userID uint64, orderID uint64) (*entities.Order, error) {
	s.logger.Debug("FindOrderByIDForTelegram: --- НАЧАЛО ---", zap.Uint64("userID", userID), zap.Uint64("orderID", orderID))

	// ШАГ 1: Быстро получаем заявку из репозитория.
	s.logger.Debug("FindOrderByIDForTelegram: Шаг 1: Поиск заявки...")
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		s.logger.Error("FindOrderByIDForTelegram: Ошибка на шаге 1", zap.Error(err))
		return nil, err
	}
	s.logger.Debug("FindOrderByIDForTelegram: Шаг 1.1: Заявка найдена.")

	// ШАГ 2: Быстро получаем пользователя, который делает запрос.
	s.logger.Debug("FindOrderByIDForTelegram: Шаг 2: Поиск пользователя...")
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("FindOrderByIDForTelegram: Ошибка на шаге 2", zap.Error(err))
		return nil, apperrors.ErrUserNotFound
	}
	s.logger.Debug("FindOrderByIDForTelegram: Шаг 2.1: Пользователь найден.")

	// ШАГ 3: Загружаем права пользователя.
	s.logger.Debug("FindOrderByIDForTelegram: Шаг 3: Получение прав доступа... (возможно зависание)")
	permissions, err := s.authPermissionService.GetAllUserPermissions(ctx, userID)
	if err != nil {
		s.logger.Error("FindOrderByIDForTelegram: Ошибка на шаге 3", zap.Error(err), zap.Uint64("userID", userID))
		return nil, err
	}
	permissionsMap := make(map[string]bool)
	for _, p := range permissions {
		permissionsMap[p] = true
	}
	s.logger.Debug("FindOrderByIDForTelegram: Шаг 3.1: Права доступа получены.", zap.Int("count", len(permissionsMap)))

	// ШАГ 4: Собираем контекст авторизации.
	authContext := &authz.Context{
		Actor:       actor,
		Permissions: permissionsMap,
		Target:      order,
	}
	wasInHistory, _ := s.historyRepo.IsUserParticipant(ctx, order.ID, userID)
	isCreator := order.CreatorID == userID
	isExecutor := order.ExecutorID != nil && *order.ExecutorID == userID
	authContext.IsParticipant = isCreator || isExecutor || wasInHistory
	s.logger.Debug("FindOrderByIDForTelegram: Шаг 4: Контекст авторизации собран.")

	// ШАГ 5: Проверяем право на обновление.
	s.logger.Debug("FindOrderByIDForTelegram: Шаг 5: Проверка прав...")
	if !authz.CanDo(authz.OrdersUpdate, *authContext) {
		s.logger.Warn("FindOrderByIDForTelegram: Доступ запрещен", zap.Uint64("userID", userID), zap.Uint64("orderID", orderID))
		return nil, apperrors.ErrForbidden
	}
	s.logger.Debug("FindOrderByIDForTelegram: Шаг 5.1: Проверка прав пройдена.")

	s.logger.Debug("FindOrderByIDForTelegram: --- УСПЕШНОЕ ЗАВЕРШЕНИЕ ---")
	return order, nil
}

func (s *OrderService) validateOrderRules(ctx context.Context, createDTO dto.CreateOrderDTO) error {
	// Если тип не указан, пропускаем (базовая валидация required это уже проверила бы)
	if !createDTO.OrderTypeID.Valid {
		return nil
	}

	// 1. Получаем строковый КОД типа из базы (например ID 1 -> "EQUIPMENT")
	orderTypeCode, err := s.orderTypeRepo.FindCodeByID(ctx, uint64(createDTO.OrderTypeID.Int))
	if err != nil {
		s.logger.Warn("Не удалось определить код типа заявки для валидации", zap.Error(err))
		return nil
	}

	// 2. Ищем правила для этого кода в реестре
	requiredFields, exists := OrderValidationRules[orderTypeCode]
	if !exists {
		return nil // Для этого типа нет особых правил
	}

	// 3. Проверяем каждое поле из списка
	for _, field := range requiredFields {
		if !s.checkFieldPresence(createDTO, field) {
			return apperrors.NewHttpError(
				http.StatusBadRequest,

				fmt.Sprintf("Для типа заявки '%s' обязательно заполнение поля: %s",
					getFieldLabel(orderTypeCode), // <-- Превращает "EQUIPMENT" в "Оборудование"
					getFieldLabel(field),         // <-- Превращает "priority_id" в "Приоритет"
				),
				nil,
				nil,
			)
		}
	}

	return nil
}

// checkFieldPresence проверяет физическое наличие данных в поле DTO
func (s *OrderService) checkFieldPresence(d dto.CreateOrderDTO, fieldName string) bool {
	switch fieldName {
	case "equipment_id":
		// Было .Int64 -> Стало .Int
		return d.EquipmentID.Valid && d.EquipmentID.Int > 0

	case "equipment_type_id":
		return d.EquipmentTypeID.Valid && d.EquipmentTypeID.Int > 0

	case "priority_id":
		return d.PriorityID.Valid && d.PriorityID.Int > 0

	case "executor_id":
		return d.ExecutorID.Valid && d.ExecutorID.Int > 0

	case "otdel_id":
		return d.OtdelID.Valid && d.OtdelID.Int > 0

	case "address":
		return d.Address.Valid && strings.TrimSpace(d.Address.String) != ""

	case "comment":
		return d.Comment.Valid && strings.TrimSpace(d.Comment.String) != ""

	case "duration":
		return d.Duration.Valid && !d.Duration.Time.IsZero()

	default:
		return true
	}
}

// getFieldLabel переводит системные названия на русский для ошибки
func getFieldLabel(code string) string {
	switch code {
	// Поля
	case "equipment_id":
		return "Оборудование"
	case "equipment_type_id":
		return "Тип оборудования"
	case "priority_id":
		return "Приоритет"
	case "address":
		return "Адрес / Кабинет"
	case "otdel_id":
		return "Отдел"
	case "comment":
		return "Описание / Комментарий"

	// Типы заявок (для красивого текста ошибки)
	case "EQUIPMENT":
		return "Оборудование"
	case "ADMINISTRATIVE":
		return "Простая заявка"

	default:
		return code
	}
}

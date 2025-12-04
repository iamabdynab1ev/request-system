package services

import (
	"context"
	"database/sql"
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

// Этот реестр раньше был тут или в validation_registry.go
var OrderValidationRules = map[string][]string{
	"EQUIPMENT": {"equipment_id", "equipment_type_id", "priority_id"},
}

// Для API конфига фронтенда
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

// =========================================================================
// READ OPERATIONS
// =========================================================================

func (s *OrderService) GetOrders(ctx context.Context, filter types.Filter, onlyParticipant bool) (*dto.OrderListResponseDTO, error) {
	// 1. Получаем актора
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	// 2. AuthZ: Право на просмотр
	authCtx := authz.Context{Actor: actor, Permissions: permissionsMap}
	if !authz.CanDo(authz.OrdersView, authCtx) {
		s.logger.Warn("Попытка доступа без order:view", zap.Uint64("userID", userID))
		return nil, apperrors.ErrForbidden
	}

	// 3. Строим SQL фильтры по Scopes
	securityBuilder := sq.And{}

	// Если НЕТ глобального доступа - добавляем ограничения
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
			scopeConditions = append(scopeConditions, sq.Eq{"o.user_id": actor.ID})     // Создатель
			scopeConditions = append(scopeConditions, sq.Eq{"o.executor_id": actor.ID}) // Исполнитель
			// Был участником истории
			scopeConditions = append(scopeConditions, sq.Expr("o.id IN (SELECT DISTINCT order_id FROM order_history WHERE user_id = ?)", actor.ID))
		}

		if len(scopeConditions) > 0 {
			securityBuilder = append(securityBuilder, scopeConditions)
		} else {
			// Если прав совсем нет -> пустой список
			return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
		}
	}

	// 4. Флаг "Только мое участие"
	if onlyParticipant {
		participantCondition := sq.Or{
			sq.Eq{"o.user_id": actor.ID},
			sq.Eq{"o.executor_id": actor.ID},
			sq.Expr("o.id IN (SELECT DISTINCT order_id FROM order_history WHERE user_id = ?)", actor.ID),
		}
		securityBuilder = append(securityBuilder, participantCondition)
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
	dtos := s.bulkMapOrders(ctx, orders)

	return &dto.OrderListResponseDTO{List: dtos, TotalCount: totalCount}, nil
}

func (s *OrderService) FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error) {
	// Авторизация + загрузка target
	authCtx, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	order := authCtx.Target.(*entities.Order)

	// Загружаем связи
	creator, _ := s.userRepo.FindUserByID(ctx, order.CreatorID)
	var executor *entities.User
	if order.ExecutorID != nil {
		executor, _ = s.userRepo.FindUserByID(ctx, *order.ExecutorID)
	}
	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, order.ID, 100, 0)

	return s.toResponseDTO(order, creator, executor, attachments), nil
}

func (s *OrderService) FindOrderByIDForTelegram(ctx context.Context, userID uint64, orderID uint64) (*entities.Order, error) {
	// (Оставил без изменений, используется для ТГ бота)
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
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
		return nil, err
	}

	authCtx := authz.Context{Actor: user, Permissions: permMap, Target: order}
	if !authz.CanDo(authz.OrdersView, authCtx) {
		return nil, apperrors.ErrForbidden
	}
	return order, nil
}

// =========================================================================
// CREATE OPERATION
// =========================================================================

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
	if !hasDept && !hasBranch {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Необходимо выбрать либо Департамент, либо Филиал.", nil, nil)
	}

	var createdID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New()

		// 3. Rule Engine (определение исполнителя)
		// Используем SafeDeref для безопасного получения значений из указателей DTO
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

		// 4. Defaults
		status, err := s.statusRepo.FindByCodeInTx(ctx, tx, "OPEN")
		if err != nil {
			return apperrors.ErrInternalServer
		}

		// 5. Construct Entity (DTO Ptr -> Entity Ptr matches perfectly)
		orderEntity := &entities.Order{
			Name:    createDTO.Name,
			Address: createDTO.Address,
			// DTO Pointers mapping
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

	// Возвращаем полностью заполненную структуру
	return s.FindOrderByID(ctx, createdID)
}

// =========================================================================
// UPDATE OPERATION (CLEAN ARCHITECTURE)
// =========================================================================

func (s *OrderService) UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	currentOrder, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	authCtx, err := s.buildAuthzContextWithTarget(ctx, currentOrder)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersUpdate, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New()
		now := time.Now()

		// 1. Делаем копию заявки для изменений
		updated := *currentOrder

		// 2. ПАТЧИНГ: ApplyUpdates автоматически копирует поля *string/*uint64/etc из DTO в Entity
		// (Если поле в DTO nil, оно не трогается)
		fieldsChanged := utils.ApplyUpdates(&updated, &updateDTO)

		// 3. БИЗНЕС-ЛОГИКА (Side Effects)

		// А. Если сменилась структура (Департамент/Отдел) -> пересчитываем исполнителя
		structureChanged := utils.DiffPtr(currentOrder.DepartmentID, updated.DepartmentID) || utils.DiffPtr(currentOrder.OtdelID, updated.OtdelID)
		if structureChanged {
			s.logger.Info("Изменение структуры, переназначение", zap.Uint64("order_id", orderID))
			orderCtx := OrderContext{
				DepartmentID: utils.SafeDeref(updated.DepartmentID),
				OtdelID:      updated.OtdelID,
				BranchID:     updated.BranchID,
			}
			res, err := s.ruleEngine.ResolveExecutor(ctx, tx, orderCtx, nil)
			if err != nil {
				return apperrors.NewBadRequestError("Не найден исполнитель для новой структуры")
			}
			updated.ExecutorID = &res.Executor.ID

			// Специфическое правило: сброс отдела при смене департамента
			if utils.DiffPtr(currentOrder.DepartmentID, updated.DepartmentID) {
				updated.OtdelID = nil
			}
		}

		// Б. Обработка смены Статуса (CLOSED/COMPLETED)
		if utils.DiffPtr(&currentOrder.StatusID, &updated.StatusID) {
			st, _ := s.statusRepo.FindByIDInTx(ctx, tx, updated.StatusID)
			if st != nil && st.Code != nil {
				code := strings.ToUpper(*st.Code)
				if code == "CLOSED" || code == "COMPLETED" {
					updated.CompletedAt = &now
				} else {
					updated.CompletedAt = nil // Reopen
				}
			}
		}

		// В. Метрики (First Response / FCR)
		// Срабатывает только 1 раз при первом осмысленном действии
		s.calculateMetrics(&updated, currentOrder, updateDTO, authCtx.Actor.ID, now)

		// 4. ЗАПИСЬ В ИСТОРИЮ + EVENTBUS
		// Метод сам сравнит currentOrder vs updated и запишет изменения
		histChanged, err := s.detectAndLogChanges(ctx, tx, currentOrder, &updated, updateDTO, authCtx.Actor, txID, now)
		if err != nil {
			return err
		}

		// 5. Файл
		fileAttached := false
		if file != nil {
			if _, err := s.attachFileToOrderInTx(ctx, tx, orderID, authCtx.Actor.ID, file, &txID, &updated); err != nil {
				return err
			}
			fileAttached = true
		}

		if !fieldsChanged && !histChanged && !fileAttached {
			return apperrors.ErrNoChanges
		}

		// 6. Сохранение в БД
		return s.orderRepo.Update(ctx, tx, &updated)
	})
	if err != nil {
		return nil, err
	}
	return s.FindOrderByID(ctx, orderID)
}

// detectAndLogChanges - ЯДРО логирования
func (s *OrderService) detectAndLogChanges(ctx context.Context, tx pgx.Tx, old, new *entities.Order, dto dto.UpdateOrderDTO, actor *entities.User, txID uuid.UUID, now time.Time) (bool, error) {
	hasLoggable := false

	// Комментарий (из DTO всегда, т.к. в Entity нет поля current comment)
	if dto.Comment != nil && strings.TrimSpace(*dto.Comment) != "" {
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "COMMENT", nil, nil, dto.Comment, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	// Делегация (Исполнитель)
	if utils.DiffPtr(old.ExecutorID, new.ExecutorID) {
		newExName := s.resolveUserName(ctx, new.ExecutorID)
		txt := "Назначено на: " + newExName
		// Для SQL значения берем указатели ID, преобразованные в строку
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

	// Срок выполнения (Duration)
	if !timePointersEqual(old.Duration, new.Duration) {
		// time logic handled if necessary or simply logged
		valNew := ""
		if new.Duration != nil {
			valNew = new.Duration.Format(time.RFC3339)
		}
		if err := s.logHistoryEvent(ctx, tx, new.ID, actor, "DURATION_CHANGE", &valNew, nil, nil, txID, *new); err != nil {
			return false, err
		}
		hasLoggable = true
	}

	// Подразделение (системное событие)
	if utils.DiffPtr(old.DepartmentID, new.DepartmentID) || utils.DiffPtr(old.BranchID, new.BranchID) {
		// Log generic change if needed
	}

	return hasLoggable, nil
}

// =========================================================================
// DELETE
// =========================================================================

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

// =========================================================================
// HELPERS & GETTERS
// =========================================================================

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

// bulkMapOrders - ускоренный маппинг списка
func (s *OrderService) bulkMapOrders(ctx context.Context, orders []entities.Order) []dto.OrderResponseDTO {
	userIDsMap := make(map[uint64]bool)
	orderIDs := make([]uint64, len(orders))

	for i, o := range orders {
		orderIDs[i] = o.ID
		userIDsMap[o.CreatorID] = true
		if o.ExecutorID != nil {
			userIDsMap[*o.ExecutorID] = true
		}
	}
	uids := make([]uint64, 0, len(userIDsMap))
	for id := range userIDsMap {
		uids = append(uids, id)
	}

	usersMap, _ := s.userRepo.FindUsersByIDs(ctx, uids)
	attachMap, _ := s.attachRepo.FindAttachmentsByOrderIDs(ctx, orderIDs)

	res := make([]dto.OrderResponseDTO, len(orders))
	for i, o := range orders {
		var creator, executor *entities.User
		if u, ok := usersMap[o.CreatorID]; ok {
			creator = &u
		}
		if o.ExecutorID != nil {
			if u, ok := usersMap[*o.ExecutorID]; ok {
				executor = &u
			}
		}
		atts := attachMap[o.ID]
		res[i] = *s.toResponseDTO(&o, creator, executor, atts)
	}
	return res
}

func (s *OrderService) toResponseDTO(o *entities.Order, cr, ex *entities.User, atts []entities.Attachment) *dto.OrderResponseDTO {
	// Основные поля
	d := &dto.OrderResponseDTO{
		ID: o.ID, Name: o.Name, StatusID: o.StatusID, CreatedAt: o.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   o.UpdatedAt.Format(time.RFC3339),
		OrderTypeID: o.OrderTypeID, Address: o.Address, DepartmentID: o.DepartmentID,
		OtdelID: o.OtdelID, BranchID: o.BranchID, OfficeID: o.OfficeID,
		EquipmentID: o.EquipmentID, EquipmentTypeID: o.EquipmentTypeID, PriorityID: o.PriorityID,
		Duration: o.Duration, CompletedAt: o.CompletedAt,
		ResolutionTimeSeconds: o.ResolutionTimeSeconds, FirstResponseTimeSeconds: o.FirstResponseTimeSeconds,
	}

	if o.ResolutionTimeSeconds != nil {
		d.ResolutionTimeFormatted = utils.FormatSecondsToHumanReadable(*o.ResolutionTimeSeconds)
	}
	if o.FirstResponseTimeSeconds != nil {
		d.FirstResponseTimeFormatted = utils.FormatSecondsToHumanReadable(*o.FirstResponseTimeSeconds)
	}

	if cr != nil {
		d.Creator = dto.ShortUserDTO{ID: cr.ID, Fio: cr.Fio}
	}
	if ex != nil {
		d.Executor = &dto.ShortUserDTO{ID: ex.ID, Fio: ex.Fio}
	}

	d.Attachments = make([]dto.AttachmentResponseDTO, len(atts))
	for i, a := range atts {
		d.Attachments[i] = dto.AttachmentResponseDTO{ID: a.ID, FileName: a.FileName, URL: "/uploads/" + a.FilePath}
	}
	return d
}

// ----------------- AuthZ & Internal Helpers -----------------

func (s *OrderService) buildAuthzContext(ctx context.Context, orderID uint64) (*authz.Context, error) {
	if orderID == 0 {
		userID, _ := utils.GetUserIDFromCtx(ctx)
		perms, _ := utils.GetPermissionsMapFromCtx(ctx)
		actor, err := s.userRepo.FindUserByID(ctx, userID)
		if err != nil {
			return nil, apperrors.ErrUserNotFound
		}
		return &authz.Context{Actor: actor, Permissions: perms}, nil
	}
	t, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return s.buildAuthzContextWithTarget(ctx, t)
}

func (s *OrderService) buildAuthzContextWithTarget(ctx context.Context, t *entities.Order) (*authz.Context, error) {
	userID, _ := utils.GetUserIDFromCtx(ctx)
	perms, _ := utils.GetPermissionsMapFromCtx(ctx)
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

func (s *OrderService) calculateMetrics(newOrder, oldOrder *entities.Order, dto dto.UpdateOrderDTO, actorID uint64, now time.Time) {
	meaningful := dto.StatusID != nil || (dto.Comment != nil && *dto.Comment != "") || dto.ExecutorID != nil

	isValidDate := !oldOrder.CreatedAt.IsZero() && oldOrder.CreatedAt.Year() > 2000

	if oldOrder.FirstResponseTimeSeconds == nil && actorID != oldOrder.CreatorID && meaningful && isValidDate {
		diff := now.Sub(oldOrder.CreatedAt).Seconds()

		if diff < 0 {
			diff = 0
		}

		seconds := uint64(diff)
		newOrder.FirstResponseTimeSeconds = &seconds

		isFCR := false
		if dto.StatusID != nil {
		}
		newOrder.IsFirstContactResolution = &isFCR
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

// ------------------------------------------
// Validation Logic
// ------------------------------------------
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
	} // ignore if type not found

	if rules, ok := OrderValidationRules[code]; ok {
		for _, field := range rules {
			if !s.checkFieldPresence(d, field) {
				return apperrors.NewBadRequestError(fmt.Sprintf("Поле %s обязательно", field))
			}
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
		return d.Comment != nil && *d.Comment != ""
	default:
		return true
	}
}

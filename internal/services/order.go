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

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
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
	GetOrders(ctx context.Context, filter types.Filter) (*dto.OrderListResponseDTO, error)
	FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error)
	UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader, rawRequestBody []byte) (*dto.OrderResponseDTO, error)
	DeleteOrder(ctx context.Context, orderID uint64) error
	GetStatusByID(ctx context.Context, id uint64) (*entities.Status, error)
	GetPriorityByID(ctx context.Context, id uint64) (*entities.Priority, error)
	GetValidationConfigForOrderType(ctx context.Context, orderTypeID uint64) (map[string]interface{}, error)
}

type OrderService struct {
	txManager     repositories.TxManagerInterface
	orderRepo     repositories.OrderRepositoryInterface
	userRepo      repositories.UserRepositoryInterface
	statusRepo    repositories.StatusRepositoryInterface
	priorityRepo  repositories.PriorityRepositoryInterface
	attachRepo    repositories.AttachmentRepositoryInterface
	ruleEngine    RuleEngineServiceInterface
	historyRepo   repositories.OrderHistoryRepositoryInterface
	orderTypeRepo repositories.OrderTypeRepositoryInterface
	fileStorage   filestorage.FileStorageInterface
	logger        *zap.Logger
}

func NewOrderService(
	txManager repositories.TxManagerInterface, orderRepo repositories.OrderRepositoryInterface,
	userRepo repositories.UserRepositoryInterface, statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface, attachRepo repositories.AttachmentRepositoryInterface,
	ruleEngine RuleEngineServiceInterface, historyRepo repositories.OrderHistoryRepositoryInterface,
	fileStorage filestorage.FileStorageInterface, logger *zap.Logger,
	orderTypeRepo repositories.OrderTypeRepositoryInterface,
) OrderServiceInterface {
	return &OrderService{
		txManager: txManager, orderRepo: orderRepo, userRepo: userRepo, statusRepo: statusRepo,
		priorityRepo: priorityRepo, attachRepo: attachRepo, ruleEngine: ruleEngine, historyRepo: historyRepo,
		fileStorage: fileStorage, logger: logger, orderTypeRepo: orderTypeRepo,
	}
}

func (s *OrderService) GetStatusByID(ctx context.Context, id uint64) (*entities.Status, error) {
	return s.statusRepo.FindStatus(ctx, id)
}

func (s *OrderService) GetPriorityByID(ctx context.Context, id uint64) (*entities.Priority, error) {
	return s.priorityRepo.FindByID(ctx, id)
}

func (s *OrderService) GetOrders(ctx context.Context, filter types.Filter) (*dto.OrderListResponseDTO, error) {
	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	authContext := authz.Context{Actor: actor, Permissions: permissionsMap}
	if !authz.CanDo(authz.OrdersView, authContext) {
		return nil, apperrors.ErrForbidden
	}

	var securityFilter string
	var securityArgs []interface{}
	if !authContext.HasPermission(authz.ScopeAll) && !authContext.HasPermission(authz.ScopeAllView) {
		var conditions []string
		scopeRules := map[string]func(){
			authz.ScopeDepartment: func() {
				if actor.DepartmentID > 0 {
					conditions = append(conditions, "o.department_id = ?")
					securityArgs = append(securityArgs, actor.DepartmentID)
				}
			},
			authz.ScopeBranch: func() {
				if actor.BranchID != nil {
					conditions = append(conditions, "o.branch_id = ?")
					securityArgs = append(securityArgs, *actor.BranchID)
				}
			},
			authz.ScopeOwn: func() {
				condition := `(o.user_id = ? OR o.executor_id = ? OR o.id IN (SELECT DISTINCT order_id FROM order_history WHERE user_id = ?))`
				conditions = append(conditions, condition)
				securityArgs = append(securityArgs, actor.ID, actor.ID, actor.ID)
			},
		}
		for scope, ruleFunc := range scopeRules {
			if authContext.HasPermission(scope) {
				ruleFunc()
			}
		}
		if len(conditions) == 0 {
			return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
		}
		securityFilter = "(" + strings.Join(conditions, " OR ") + ")"
	}

	orders, totalCount, err := s.orderRepo.GetOrders(ctx, filter, securityFilter, securityArgs)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
	}

	userIDsMap := make(map[uint64]struct{})
	orderIDs := make([]uint64, 0, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
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

	dtos := make([]dto.OrderResponseDTO, 0, len(orders))
	for i := range orders {
		order := &orders[i]
		creator := usersMap[order.CreatorID]
		var executorPtr *entities.User
		if order.ExecutorID != nil {
			if exec, ok := usersMap[*order.ExecutorID]; ok && &exec != nil {
				executorPtr = &exec
			}
		}
		attachments := attachmentsMap[order.ID]
		dtos = append(dtos, *buildOrderResponse(order, &creator, executorPtr, attachments))
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
	authContext, err := s.buildAuthzContext(ctx, 0)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	if createDTO.OrderTypeID.Valid {
		orderTypeCode, err := s.orderTypeRepo.FindCodeByID(ctx, uint64(createDTO.OrderTypeID.Int))
		if err == nil {
			if _, ok := ValidationRegistry[orderTypeCode]; ok {
				// Здесь в будущем можно добавить логику валидации на основе правил
			}
		}
	}

	var finalOrderID uint64

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New() // Один ID на всю операцию
		now := time.Now()  // Одно время на всю операцию

		// --- 1. Определение исполнителя ---
		var userSelectedExecutorID *uint64
		if createDTO.ExecutorID.Valid {
			v := uint64(createDTO.ExecutorID.Int)
			userSelectedExecutorID = &v
		}

		orderCtx := OrderContext{
			OrderTypeID:  uint64(createDTO.OrderTypeID.Int),
			DepartmentID: uint64(createDTO.DepartmentID.Int),
			OtdelID:      utils.NullIntToUint64Ptr(createDTO.OtdelID),
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

		// --- 2. Создание основной записи о заявке ---
		orderEntity := &entities.Order{
			OrderTypeID:     utils.NullIntToUint64Ptr(createDTO.OrderTypeID),
			Name:            createDTO.Name,
			Address:         utils.NullStringToStrPtr(createDTO.Address),
			DepartmentID:    uint64(createDTO.DepartmentID.Int),
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
		s.logger.Debug("Заявка успешно создана в БД", zap.Uint64("orderID", orderID))

		// --- 3. Создание записей в ИСТОРИИ ---

		// Запись 1: Создание заявки
		if err := s.historyRepo.CreateInTx(ctx, tx, &repositories.OrderHistoryItem{
			OrderID:   orderID,
			UserID:    authContext.Actor.ID,
			EventType: "CREATE",
			NewValue:  sql.NullString{String: orderEntity.Name, Valid: true},
			TxID:      &txID,
			CreatedAt: now,
		}); err != nil {
			return err
		}

		// Запись 2: Назначение исполнителя
		delegationText := "Назначено на: " + result.Executor.Fio
		if err := s.historyRepo.CreateInTx(ctx, tx, &repositories.OrderHistoryItem{
			OrderID:   orderID,
			UserID:    authContext.Actor.ID,
			EventType: "DELEGATION",
			NewValue:  sql.NullString{String: fmt.Sprintf("%d", result.Executor.ID), Valid: true},
			Comment:   sql.NullString{String: delegationText, Valid: true},
			TxID:      &txID,
			CreatedAt: now,
		}); err != nil {
			return err
		}

		// Запись 3: Установка статуса
		if err := s.historyRepo.CreateInTx(ctx, tx, &repositories.OrderHistoryItem{
			OrderID:   orderID,
			UserID:    authContext.Actor.ID,
			EventType: "STATUS_CHANGE",
			NewValue:  sql.NullString{String: fmt.Sprintf("%d", status.ID), Valid: true},
			TxID:      &txID,
			CreatedAt: now,
		}); err != nil {
			return err
		}

		// Запись 4: Установка приоритета (если был указан)
		if orderEntity.PriorityID != nil {
			if err := s.historyRepo.CreateInTx(ctx, tx, &repositories.OrderHistoryItem{
				OrderID:   orderID,
				UserID:    authContext.Actor.ID,
				EventType: "PRIORITY_CHANGE",
				NewValue:  sql.NullString{String: fmt.Sprintf("%d", *orderEntity.PriorityID), Valid: true},
				TxID:      &txID,
				CreatedAt: now,
			}); err != nil {
				return err
			}
		}

		// Запись 5: Добавление комментария (если был указан)
		if createDTO.Comment.Valid {
			if err := s.historyRepo.CreateInTx(ctx, tx, &repositories.OrderHistoryItem{
				OrderID:   orderID,
				UserID:    authContext.Actor.ID,
				EventType: "COMMENT",
				Comment:   sql.NullString{String: createDTO.Comment.String, Valid: true},
				TxID:      &txID,
				CreatedAt: now,
			}); err != nil {
				return err
			}
		}

		// Запись 6: Прикрепление файла (если был прикреплен)
		if file != nil {
			if _, err := s.attachFileToOrderInTx(ctx, tx, orderID, authContext.Actor.ID, file, &txID); err != nil {
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

func (s *OrderService) UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader, rawRequestBody []byte) (*dto.OrderResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	currentOrder := authContext.Target.(*entities.Order)

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New()
		now := time.Now()
		updated := false

		// --- ОБРАБОТКА ОБНОВЛЕНИЯ ПОЛЕЙ С ЗАПИСЬЮ В ИСТОРИЮ ---

		// Простой комментарий
		if updateDTO.Comment.Valid {
			if err := s.historyRepo.CreateInTx(ctx, tx, &repositories.OrderHistoryItem{
				OrderID: orderID, UserID: authContext.Actor.ID, EventType: "COMMENT",
				Comment: sql.NullString{String: updateDTO.Comment.String, Valid: true},
				TxID:    &txID, CreatedAt: now,
			}); err != nil {
				return err
			}
			updated = true
		}

		// Изменение статуса
		if updateDTO.StatusID.Valid {
			oldStatusID := currentOrder.StatusID
			newStatusID := uint64(updateDTO.StatusID.Int)

			if oldStatusID != newStatusID {
				newStatus, err := s.statusRepo.FindByIDInTx(ctx, tx, newStatusID)
				if err != nil || newStatus == nil {
					return apperrors.NewHttpError(http.StatusBadRequest, "Указанный статус не найден", err, nil)
				}

				if newStatus.Code != nil && *newStatus.Code == "CLOSED" {
					currentOrder.CompletedAt = &now
					if currentOrder.ResolutionTimeSeconds == nil {
						resTime := uint64(now.Sub(currentOrder.CreatedAt).Seconds())
						currentOrder.ResolutionTimeSeconds = &resTime
					}
				}
				currentOrder.StatusID = newStatusID

				if err := s.historyRepo.CreateInTx(ctx, tx, &repositories.OrderHistoryItem{
					OrderID: orderID, UserID: authContext.Actor.ID, EventType: "STATUS_CHANGE",
					OldValue: utils.Uint64ToNullString(oldStatusID), NewValue: utils.Uint64ToNullString(newStatusID),
					TxID: &txID, CreatedAt: now,
				}); err != nil {
					return err
				}
				updated = true
			}
		}

		// Переназначение исполнителя
		if updateDTO.ExecutorID.Valid {
			newExecutorID := uint64(updateDTO.ExecutorID.Int)
			oldExecutorIDPtr := currentOrder.ExecutorID

			if oldExecutorIDPtr == nil || *oldExecutorIDPtr != newExecutorID {
				if !authz.CanDo(authz.OrdersUpdateExecutorID, *authContext) {
					return apperrors.ErrForbidden
				}

				newExec, err := s.userRepo.FindUserByIDInTx(ctx, tx, newExecutorID)
				if err != nil || newExec == nil {
					return apperrors.NewHttpError(http.StatusBadRequest, "Новый исполнитель не найден", err, nil)
				}

				currentOrder.ExecutorID = &newExecutorID
				delegationText := "Переназначено на: " + newExec.Fio

				if err := s.historyRepo.CreateInTx(ctx, tx, &repositories.OrderHistoryItem{
					OrderID: orderID, UserID: authContext.Actor.ID, EventType: "DELEGATION",
					OldValue:  utils.Uint64PtrToNullString(oldExecutorIDPtr),
					NewValue:  utils.Uint64PtrToNullString(currentOrder.ExecutorID),
					Comment:   sql.NullString{String: delegationText, Valid: true},
					TxID:      &txID,
					CreatedAt: now,
				}); err != nil {
					return err
				}
				updated = true
			}
		}

		// Изменение отдела (OtdelID)
		if updateDTO.OtdelID.Valid {
			newVal := uint64(updateDTO.OtdelID.Int)
			if currentOrder.OtdelID == nil || *currentOrder.OtdelID != newVal {
				oldVal := currentOrder.OtdelID
				currentOrder.OtdelID = &newVal
				if err := s.historyRepo.CreateInTx(ctx, tx, &repositories.OrderHistoryItem{
					OrderID: orderID, UserID: authContext.Actor.ID, EventType: "OTDEL_CHANGE",
					OldValue: utils.Uint64PtrToNullString(oldVal), NewValue: utils.Uint64PtrToNullString(currentOrder.OtdelID),
					TxID: &txID, CreatedAt: now,
				}); err != nil {
					return err
				}
				updated = true
			}
		}
		// изменение приоритета
		if updateDTO.PriorityID.Valid {
			newVal := uint64(updateDTO.PriorityID.Int)
			if currentOrder.PriorityID == nil || *currentOrder.PriorityID != newVal {
				oldVal := currentOrder.PriorityID
				currentOrder.PriorityID = &newVal
				if err := s.historyRepo.CreateInTx(ctx, tx, &repositories.OrderHistoryItem{
					OrderID: orderID, UserID: authContext.Actor.ID, EventType: "PRIORITY_CHANGE",
					OldValue: utils.Uint64PtrToNullString(oldVal), NewValue: utils.Uint64PtrToNullString(currentOrder.PriorityID),
					TxID: &txID, CreatedAt: now,
				}); err != nil {
					return err
				}
				updated = true
			}
		}
		// Изменение Duration
		if updateDTO.Duration.Valid {
			var newDuration *time.Time
			if updateDTO.Duration.Time.After(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)) {
				newDuration = &updateDTO.Duration.Time
			}

			oldTimeStr, newTimeStr := "null", "null"
			if currentOrder.Duration != nil {
				oldTimeStr = currentOrder.Duration.UTC().Format(time.RFC3339)
			}
			if newDuration != nil {
				newTimeStr = newDuration.UTC().Format(time.RFC3339)
			}

			s.logger.Debug("Сравнение Duration",
				zap.String("old_value", oldTimeStr),
				zap.String("new_value", newTimeStr),
				zap.Bool("is_different", oldTimeStr != newTimeStr),
			)

			if oldTimeStr != newTimeStr {
				oldDuration := currentOrder.Duration
				currentOrder.Duration = newDuration

				if err := s.historyRepo.CreateInTx(ctx, tx, &repositories.OrderHistoryItem{
					OrderID: orderID, UserID: authContext.Actor.ID, EventType: "DURATION_CHANGE",
					OldValue: utils.TimeToNullString(oldDuration), NewValue: utils.TimeToNullString(currentOrder.Duration),
					TxID: &txID, CreatedAt: now,
				}); err != nil {
					return err
				}
				updated = true
			}
		}

		// Прикрепление файла
		if file != nil {
			if _, err := s.attachFileToOrderInTx(ctx, tx, orderID, authContext.Actor.ID, file, &txID); err != nil {
				return err
			}
			updated = true
		}

		if !updated {
			return apperrors.ErrNoChanges
		}

		currentOrder.UpdatedAt = now
		if err := s.orderRepo.Update(ctx, tx, currentOrder); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.FindOrderByID(ctx, orderID)
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
	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	authContext := &authz.Context{Actor: actor, Permissions: permissionsMap}
	if orderID > 0 {
		targetOrder, err := s.orderRepo.FindByID(ctx, uint64(orderID))
		if err != nil {
			return nil, err
		}
		authContext.Target = targetOrder
		wasInHistory, _ := s.historyRepo.IsUserParticipant(ctx, orderID, userID)
		isCreator := targetOrder.CreatorID == userID
		isExecutor := targetOrder.ExecutorID != nil && *targetOrder.ExecutorID == userID
		authContext.IsParticipant = isCreator || isExecutor || wasInHistory
	}
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

	return &dto.OrderResponseDTO{
		ID:                    order.ID,
		Name:                  order.Name,
		OrderTypeID:           utils.Uint64PtrToNullInt(order.OrderTypeID),
		Address:               utils.StrPtrToNull(order.Address),
		Creator:               dto.ShortUserDTO{ID: creator.ID, Fio: creator.Fio},
		Executor:              execDTO,
		DepartmentID:          order.DepartmentID,
		OtdelID:               utils.Uint64PtrToNullInt(order.OtdelID),
		BranchID:              utils.Uint64PtrToNullInt(order.BranchID),
		OfficeID:              utils.Uint64PtrToNullInt(order.OfficeID),
		EquipmentID:           utils.Uint64PtrToNullInt(order.EquipmentID),
		EquipmentTypeID:       utils.Uint64PtrToNullInt(order.EquipmentTypeID),
		StatusID:              order.StatusID,
		PriorityID:            utils.Uint64PtrToNullInt(order.PriorityID),
		Attachments:           attachDTOs,
		Duration:              utils.TimeToNull(order.Duration),
		CreatedAt:             order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             order.UpdatedAt.Format(time.RFC3339),
		CompletedAt:           utils.TimeToNull(order.CompletedAt),
		ResolutionTimeSeconds: utils.Uint64PtrToNullInt(order.ResolutionTimeSeconds),
	}
}

func (s *OrderService) attachFileToOrderInTx(ctx context.Context, tx pgx.Tx, orderID, userID uint64, file *multipart.FileHeader, txID *uuid.UUID) (uint64, error) {
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
		OrderID:   orderID,
		UserID:    userID,
		FileName:  file.Filename,
		FilePath:  filePath,
		FileType:  file.Header.Get("Content-Type"),
		FileSize:  file.Size,
		CreatedAt: time.Now(),
	}
	attachID, err := s.attachRepo.CreateInTx(ctx, tx, attach)
	if err != nil {
		return 0, err
	}
	historyItem := repositories.OrderHistoryItem{
		OrderID:      orderID,
		UserID:       userID,
		EventType:    "ATTACHMENT_ADD",
		NewValue:     sql.NullString{String: file.Filename, Valid: true},
		Attachment:   attach,
		AttachmentID: sql.NullInt64{Int64: int64(attachID), Valid: true},
		CreatedAt:    time.Now(),
		TxID:         txID,
	}
	err = s.historyRepo.CreateInTx(ctx, tx, &historyItem)
	if err != nil {
		return 0, err
	}
	return attachID, nil
}

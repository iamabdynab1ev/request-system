package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/config"
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
		{FieldName: "equipment_id", ErrorMessage: "Поле 'Оборудование' обязательно для заявок на оборудование."},
		{FieldName: "equipment_type_id", ErrorMessage: "Поле 'Тип оборудования' обязательно для заявок на оборудование."},
		{FieldName: "priority_id", ErrorMessage: "Поле 'Приоритет' обязательно для заявок на оборудование."},
	},
}

type OrderServiceInterface interface {
	CreateOrder(ctx context.Context, createDTO dto.CreateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	GetOrders(ctx context.Context, filter types.Filter) (*dto.OrderListResponseDTO, error)
	FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error)
	UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	DeleteOrder(ctx context.Context, orderID uint64) error
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
	txManager repositories.TxManagerInterface,
	orderRepo repositories.OrderRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface,
	attachRepo repositories.AttachmentRepositoryInterface,
	ruleEngine RuleEngineServiceInterface,
	historyRepo repositories.OrderHistoryRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
	orderTypeRepo repositories.OrderTypeRepositoryInterface,
) OrderServiceInterface {
	return &OrderService{
		txManager: txManager, orderRepo: orderRepo, userRepo: userRepo,
		statusRepo: statusRepo, priorityRepo: priorityRepo, attachRepo: attachRepo,
		ruleEngine: ruleEngine, historyRepo: historyRepo, fileStorage: fileStorage,
		logger: logger, orderTypeRepo: orderTypeRepo,
	}
}

func (s *OrderService) GetOrders(ctx context.Context, filter types.Filter) (*dto.OrderListResponseDTO, error) {
	// --- Получаем контекст пользователя и права ---
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	authContext := authz.Context{Actor: actor, Permissions: permissionsMap, Target: nil}
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
				if actor.BranchID > 0 {
					conditions = append(conditions, "o.branch_id = ?")
					securityArgs = append(securityArgs, actor.BranchID)
				}
			},
			authz.ScopeOffice: func() {
				if actor.OfficeID != nil {
					conditions = append(conditions, "o.office_id = ?")
					securityArgs = append(securityArgs, *actor.OfficeID)
				}
			},
			authz.ScopeOtdel: func() {
				if actor.OtdelID != nil {
					conditions = append(conditions, "o.otdel_id = ?")
					securityArgs = append(securityArgs, *actor.OtdelID)
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
			s.logger.Info("Нет разрешений на просмотр заявок, возвращаем пустой список")
			return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
		}
		securityFilter = "(" + strings.Join(conditions, " OR ") + ")"
	}

	// --- Получаем заявки из репозитория ---
	orders, totalCount, err := s.orderRepo.GetOrders(ctx, filter, securityFilter, securityArgs)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		s.logger.Info("Заявки не найдены")
		return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
	}

	// --- Сбор уникальных пользователей и вложений ---
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

	usersMap, err := s.userRepo.FindUsersByIDs(ctx, userIDs)
	if err != nil {
		s.logger.Error("Не удалось получить пользователей по ID", zap.Error(err))
		usersMap = make(map[uint64]entities.User)
	}

	attachmentsMap, err := s.attachRepo.FindAttachmentsByOrderIDs(ctx, orderIDs)
	if err != nil {
		s.logger.Error("Не удалось получить вложения по ID заявок", zap.Error(err))
		attachmentsMap = make(map[uint64][]entities.Attachment)
	}

	// --- Построение DTO для фронта ---
	dtos := make([]dto.OrderResponseDTO, 0, len(orders))
	for i := range orders {
		order := &orders[i]
		creator := usersMap[order.CreatorID]

		var executor entities.User
		if order.ExecutorID != nil {
			executor = usersMap[*order.ExecutorID]
		}

		attachments := attachmentsMap[order.ID]
		orderResponse := buildOrderResponse(order, &creator, &executor, attachments)
		dtos = append(dtos, *orderResponse)
	}

	s.logger.Info("Заявки успешно подготовлены для ответа", zap.Int("количество", len(dtos)))

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
	if order.ExecutorID != nil && *order.ExecutorID > 0 {
		executor, _ = s.userRepo.FindUserByID(ctx, *order.ExecutorID)
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

	if createDTO.OrderTypeID > 0 {
		orderTypeCode, err := s.orderTypeRepo.FindCodeByID(ctx, createDTO.OrderTypeID)
		if err != nil {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Указанный order_type_id не найден.", err, nil)
		}

		rules, rulesExist := ValidationRegistry[orderTypeCode]
		if rulesExist {
			var dtoMap map[string]interface{}
			dtoBytes, _ := json.Marshal(createDTO)
			json.Unmarshal(dtoBytes, &dtoMap)

			var validationErrors []string
			for _, rule := range rules {
				value, exists := dtoMap[rule.FieldName]
				isMissing := !exists || value == nil
				if !isMissing {
					if num, ok := value.(float64); ok && num == 0 {
						isMissing = true
					}
				}
				if isMissing {
					validationErrors = append(validationErrors, rule.ErrorMessage)
				}
			}

			if len(validationErrors) > 0 {
				combinedMessage := strings.Join(validationErrors, " ")
				return nil, apperrors.NewHttpError(http.StatusBadRequest, combinedMessage, nil, nil)
			}
		}
	}
	var finalOrderID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New()
		if createDTO.DepartmentID == nil {
			return apperrors.NewHttpError(http.StatusBadRequest, "Поле 'department_id' обязательно", nil, nil)
		}

		orderCtx := OrderContext{
			OrderTypeID: createDTO.OrderTypeID, DepartmentID: *createDTO.DepartmentID, OtdelID: createDTO.OtdelID,
		}
		var userSelectedExecutorID *uint64
		if createDTO.ExecutorID != nil {
			if !authz.CanDo(authz.OrdersCreateExecutorID, *authContext) {
				return apperrors.NewHttpError(http.StatusForbidden, "Нет прав назначать исполнителя", nil, nil)
			}
			userSelectedExecutorID = createDTO.ExecutorID
		}

		result, err := s.ruleEngine.ResolveExecutor(ctx, tx, orderCtx, userSelectedExecutorID)
		if err != nil {
			s.logger.Error("!!! ОШИБКА в ruleEngine.ResolveExecutor !!!", zap.Error(err))
			return fmt.Errorf("ошибка в ruleEngine.ResolveExecutor: %w", err)
		}

		status, _ := s.statusRepo.FindByCodeInTx(ctx, tx, "OPEN")
		var durationTime *time.Time
		if createDTO.Duration != nil && *createDTO.Duration != "" {
			parsedTime, _ := time.Parse(time.RFC3339, *createDTO.Duration)
			durationTime = &parsedTime
		}
		orderEntity := &entities.Order{
			OrderTypeID: createDTO.OrderTypeID, Name: createDTO.Name, Address: &createDTO.Address,
			DepartmentID: result.DepartmentID, OtdelID: result.OtdelID, BranchID: createDTO.BranchID, OfficeID: createDTO.OfficeID,
			EquipmentID: createDTO.EquipmentID, EquipmentTypeID: createDTO.EquipmentTypeID, StatusID: uint64(status.ID), PriorityID: createDTO.PriorityID,
			CreatorID: authContext.Actor.ID, ExecutorID: &result.Executor.ID, Duration: durationTime,
		}
		orderID, err := s.orderRepo.Create(ctx, tx, orderEntity)
		if err != nil {
			s.logger.Error("!!! ОШИБКА в orderRepo.Create !!!", zap.Error(err))
			return fmt.Errorf("ошибка в orderRepo.Create: %w", err)
		}
		finalOrderID = orderID

		// Запись в историю...
		createMeta, _ := json.Marshal(createDTO)
		if err := s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
			OrderID: orderID, UserID: authContext.Actor.ID, EventType: "CREATE",
			NewValue: utils.StringToNullString(orderEntity.Name), TxID: &txID, Metadata: createMeta,
		}, nil); err != nil {
			s.logger.Error("!!! ОШИБКА в historyRepo (CREATE) !!!", zap.Error(err))
			return fmt.Errorf("ошибка в historyRepo (CREATE): %w", err)
		}

		delegationMeta, _ := json.Marshal(map[string]uint64{"new_executor_id": result.Executor.ID})
		if err := s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
			OrderID: orderID, UserID: authContext.Actor.ID, EventType: "DELEGATION",
			NewValue: utils.StringToNullString("Назначено на: " + result.Executor.Fio),
			TxID:     &txID, Metadata: delegationMeta,
		}, nil); err != nil {
			s.logger.Error("!!! ОШИБКА в historyRepo (DELEGATION) !!!", zap.Error(err))
			return fmt.Errorf("ошибка в historyRepo (DELEGATION): %w", err)
		}

		if createDTO.PriorityID != nil {
			priority, err := s.priorityRepo.FindByIDInTx(ctx, tx, *createDTO.PriorityID)
			if err != nil {
				s.logger.Error("!!! ОШИБКА в priorityRepo.FindByIDInTx !!!", zap.Error(err))
				return fmt.Errorf("ошибка в priorityRepo.FindByIDInTx: %w", err)
			}
			if priority == nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Приоритет не найден", nil, nil)
			}

			priorityMeta, _ := json.Marshal(map[string]uint64{"new_priority_id": *createDTO.PriorityID})
			if err := s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
				OrderID: orderID, UserID: authContext.Actor.ID,
				EventType: "PRIORITY_CHANGE", NewValue: utils.StringToNullString(priority.Name),
				TxID: &txID, Metadata: priorityMeta,
			}, nil); err != nil {
				s.logger.Error("!!! ОШИБКА в historyRepo (PRIORITY) !!!", zap.Error(err))
				return fmt.Errorf("ошибка в historyRepo (PRIORITY): %w", err)
			}
		}

		var attachmentIDForComment *uint64
		if file != nil {
			createdID, err := s.attachFileToOrderInTx(ctx, tx, file, orderID, authContext.Actor.ID, &txID)
			if err != nil {
				return err
			}
			attachmentIDForComment = &createdID
		}
		if createDTO.Comment != nil && *createDTO.Comment != "" {
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{OrderID: orderID, UserID: authContext.Actor.ID, EventType: "COMMENT", Comment: utils.StringPointerToNullString(createDTO.Comment), TxID: &txID}, attachmentIDForComment)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.FindOrderByID(ctx, finalOrderID)
}

func (s *OrderService) UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	actor := authContext.Actor
	orderToUpdate := authContext.Target.(*entities.Order)

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		txID := uuid.New()
		hasChanges := false
		permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
		if err != nil {
			return err // Не удалось получить права, прерываем
		}

		originalOrder := *orderToUpdate // Копируем оригинальное состояние для сравнения

		// 1. Изменение имени
		if updateDTO.Name != nil && *updateDTO.Name != originalOrder.Name {
			if !authz.CanDo(authz.OrdersUpdateName, *authContext) {
				return apperrors.ErrForbidden
			}
			record := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "NAME_CHANGE", TxID: &txID,
				OldValue: utils.StringToNullString(originalOrder.Name),
				NewValue: utils.StringToNullString(*updateDTO.Name),
			}
			s.historyRepo.CreateInTx(ctx, tx, record, nil)
			orderToUpdate.Name = *updateDTO.Name
			hasChanges = true
		}

		// 2. Изменение адреса
		if updateDTO.Address != nil && (originalOrder.Address == nil || *updateDTO.Address != *originalOrder.Address) {
			if !authz.CanDo(authz.OrdersUpdateAddress, *authContext) {
				return apperrors.ErrForbidden
			}
			oldAddress := ""
			if originalOrder.Address != nil {
				oldAddress = *originalOrder.Address
			}
			record := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "ADDRESS_CHANGE", TxID: &txID,
				OldValue: utils.StringToNullString(oldAddress),
				NewValue: utils.StringToNullString(*updateDTO.Address),
			}
			s.historyRepo.CreateInTx(ctx, tx, record, nil)
			orderToUpdate.Address = updateDTO.Address
			hasChanges = true
		}

		// 3. Изменение приоритета (с metadata)
		if updateDTO.PriorityID != nil && !utils.AreUint64PointersEqual(updateDTO.PriorityID, originalOrder.PriorityID) {
			if !authz.CanDo(authz.OrdersUpdatePriorityID, *authContext) {
				return apperrors.ErrForbidden
			}
			var oldPriorityID uint64 = 0
			oldPriorityName := "не задан"
			if originalOrder.PriorityID != nil {
				oldPriorityID = *originalOrder.PriorityID
				if p, _ := s.priorityRepo.FindByIDInTx(ctx, tx, oldPriorityID); p != nil {
					oldPriorityName = p.Name
				}
			}
			newPriority, _ := s.priorityRepo.FindByIDInTx(ctx, tx, *updateDTO.PriorityID)

			metadata, _ := json.Marshal(map[string]uint64{"old_priority_id": oldPriorityID, "new_priority_id": *updateDTO.PriorityID})
			record := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "PRIORITY_CHANGE", TxID: &txID,
				OldValue: utils.StringToNullString(oldPriorityName), NewValue: utils.StringToNullString(newPriority.Name),
				Metadata: metadata,
			}
			s.historyRepo.CreateInTx(ctx, tx, record, nil)
			orderToUpdate.PriorityID = updateDTO.PriorityID
			hasChanges = true
		}

		// 4. Логика смены исполнителя (автоматическая и ручная)
		newExecutorID, err := resolveNewExecutor(ctx, s, tx, updateDTO, orderToUpdate, actor, permissionsMap, &txID)
		if err != nil {
			return err
		}
		if newExecutorID != nil {
			if !authz.CanDo(authz.OrdersUpdateExecutorID, *authContext) {
				return apperrors.ErrForbidden
			}
			newExec, err := s.userRepo.FindUserByIDInTx(ctx, tx, *newExecutorID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Указанный исполнитель не найден.", err, nil)
			}

			var oldExecID uint64 = 0
			oldExecName := "не назначен"
			if originalOrder.ExecutorID != nil { // Используем originalOrder для получения старого значения
				oldExecID = *originalOrder.ExecutorID
				if old, _ := s.userRepo.FindUserByIDInTx(ctx, tx, oldExecID); old != nil {
					oldExecName = old.Fio
				}
			}

			metadata, _ := json.Marshal(map[string]uint64{"old_executor_id": oldExecID, "new_executor_id": *newExecutorID})
			record := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION", TxID: &txID,
				OldValue: utils.StringToNullString(oldExecName), NewValue: utils.StringToNullString("Назначено на: " + newExec.Fio),
				Metadata: metadata,
			}
			s.historyRepo.CreateInTx(ctx, tx, record, nil)
			orderToUpdate.ExecutorID = newExecutorID
			hasChanges = true
		}

		// 5. Логика смены статуса (с metadata)
		if updateDTO.StatusID != nil && *updateDTO.StatusID != originalOrder.StatusID {
			if !authz.CanDo(authz.OrdersUpdateStatusID, *authContext) {
				return apperrors.ErrForbidden
			}
			oldStatus, _ := s.statusRepo.FindStatusInTx(ctx, tx, originalOrder.StatusID)
			newStatus, _ := s.statusRepo.FindStatusInTx(ctx, tx, *updateDTO.StatusID)

			metadata, _ := json.Marshal(map[string]uint64{"old_status_id": originalOrder.StatusID, "new_status_id": *updateDTO.StatusID})
			record := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "STATUS_CHANGE", TxID: &txID,
				OldValue: utils.StringToNullString(oldStatus.Name), NewValue: utils.StringToNullString(newStatus.Name),
				Metadata: metadata,
			}
			s.historyRepo.CreateInTx(ctx, tx, record, nil)
			orderToUpdate.StatusID = *updateDTO.StatusID
			hasChanges = true
		}

		// 6. Работа с файлами и комментариями
		var attachmentIDForComment *uint64
		if file != nil {
			if !authz.CanDo(authz.OrdersUpdateFile, *authContext) {
				return apperrors.ErrForbidden
			}
			createdID, err := s.attachFileToOrderInTx(ctx, tx, file, orderID, actor.ID, &txID)
			if err != nil {
				return err
			}
			attachmentIDForComment = &createdID
		}
		if updateDTO.Comment != nil && *updateDTO.Comment != "" {
			if !authz.CanDo(authz.OrdersUpdateComment, *authContext) {
				return apperrors.ErrForbidden
			}
			record := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "COMMENT", TxID: &txID,
				Comment: utils.StringPointerToNullString(updateDTO.Comment),
			}
			s.historyRepo.CreateInTx(ctx, tx, record, attachmentIDForComment)
		}

		// Если были хоть какие-то изменения, обновляем заявку в БД
		if hasChanges {
			now := time.Now()
			orderToUpdate.UpdatedAt = now
			if err := s.orderRepo.Update(ctx, tx, orderToUpdate); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		var httpErr *apperrors.HttpError
		if errors.As(err, &httpErr) {
			return nil, httpErr
		}
		s.logger.Error("Ошибка при обновлении заявки", zap.Error(err), zap.Uint64("orderID", orderID))
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
	return s.orderRepo.DeleteOrder(ctx, orderID)
}

func (s *OrderService) buildAuthzContext(ctx context.Context, orderID uint64) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	authContext := &authz.Context{Actor: actor, Permissions: permissionsMap}

	if orderID > 0 {
		targetOrder, err := s.orderRepo.FindByID(ctx, orderID)
		if err != nil {
			return nil, err
		}
		authContext.Target = targetOrder

		wasInHistory, err := s.historyRepo.IsUserParticipant(ctx, orderID, userID)
		if err != nil {
			s.logger.Warn("buildAuthzContext: ошибка при проверке участия в истории", zap.Error(err), zap.Uint64("orderID", orderID), zap.Uint64("userID", userID))
		}

		authContext.IsParticipant = targetOrder.CreatorID == userID ||
			(targetOrder.ExecutorID != nil && *targetOrder.ExecutorID == userID) ||
			wasInHistory
	}

	return authContext, nil
}

func (s *OrderService) attachFileToOrderInTx(ctx context.Context, tx pgx.Tx, file *multipart.FileHeader, orderID, userID uint64, txID *uuid.UUID) (uint64, error) {
	src, err := file.Open()
	if err != nil {
		return 0, apperrors.NewHttpError(http.StatusInternalServerError, "Не удалось открыть файл", err, nil)
	}
	defer src.Close()

	const uploadContext = "order_document"
	if err = utils.ValidateFile(file, src, uploadContext); err != nil {
		return 0, apperrors.NewHttpError(http.StatusBadRequest, fmt.Sprintf("Файл не прошел валидацию: %s", err.Error()), err, nil)
	}

	rules, _ := config.UploadContexts[uploadContext]
	relativePath, err := s.fileStorage.Save(src, file.Filename, rules.PathPrefix)
	if err != nil {
		return 0, apperrors.NewHttpError(http.StatusInternalServerError, "Не удалось сохранить файл", err, nil)
	}

	fullFilePath := "/uploads/" + relativePath
	attach := &entities.Attachment{
		OrderID:  orderID,
		UserID:   userID,
		FileName: file.Filename,
		FilePath: fullFilePath,
		FileType: file.Header.Get("Content-Type"),
		FileSize: file.Size,
	}

	attachmentID, err := s.attachRepo.Create(ctx, tx, attach)
	if err != nil {
		s.fileStorage.Delete(fullFilePath)
		return 0, fmt.Errorf("не удалось создать запись о вложении в БД: %w", err)
	}
	metadataPayload := map[string]interface{}{
		"file_name":    file.Filename,
		"file_size":    file.Size,
		"content_type": file.Header.Get("Content-Type"),
	}
	metadataBytes, _ := json.Marshal(metadataPayload)

	attachHistory := &entities.OrderHistory{
		OrderID:   orderID,
		UserID:    userID,
		EventType: "ATTACHMENT_ADDED",
		NewValue:  utils.StringToNullString(file.Filename),
		TxID:      txID,
		Metadata:  metadataBytes,
	}
	if err := s.historyRepo.CreateInTx(ctx, tx, attachHistory, &attachmentID); err != nil {
		return 0, err
	}

	return attachmentID, nil
}

func buildOrderResponse(
	order *entities.Order,
	creator *entities.User,
	executor *entities.User,
	attachments []entities.Attachment,
) *dto.OrderResponseDTO {
	creatorDTO := dto.ShortUserDTO{ID: order.CreatorID}
	if creator != nil && creator.ID != 0 {
		creatorDTO.Fio = creator.Fio
	}

	var executorDTO dto.ShortUserDTO
	if order.ExecutorID != nil {
		executorDTO.ID = *order.ExecutorID
		if executor != nil && executor.ID != 0 {
			executorDTO.Fio = executor.Fio
		}
	}

	var attachmentsDTO []dto.AttachmentResponseDTO
	for _, att := range attachments {
		attachmentsDTO = append(attachmentsDTO, dto.AttachmentResponseDTO{
			ID:       att.ID,
			FileName: att.FileName,
			FileSize: att.FileSize,
			FileType: att.FileType,
			URL:      att.FilePath,
		})
	}
	if attachmentsDTO == nil {
		attachmentsDTO = make([]dto.AttachmentResponseDTO, 0)
	}

	var durationStr *string
	if order.Duration != nil {
		formatted := order.Duration.Format(time.RFC3339)
		durationStr = &formatted
	}

	address := ""
	if order.Address != nil {
		address = *order.Address
	}

	return &dto.OrderResponseDTO{
		ID:              order.ID,
		Name:            order.Name,
		OrderTypeID:     order.OrderTypeID,
		Address:         address,
		Creator:         creatorDTO,
		Executor:        executorDTO,
		DepartmentID:    order.DepartmentID,
		OtdelID:         order.OtdelID,
		BranchID:        order.BranchID,
		OfficeID:        order.OfficeID,
		EquipmentID:     order.EquipmentID,
		EquipmentTypeID: order.EquipmentTypeID,
		StatusID:        order.StatusID,
		PriorityID:      order.PriorityID,
		Attachments:     attachmentsDTO,
		Duration:        durationStr,
		CreatedAt:       order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       order.UpdatedAt.Format(time.RFC3339),
	}
}

func (s *OrderService) GetValidationConfigForOrderType(ctx context.Context, orderTypeID uint64) (map[string]interface{}, error) {
	orderTypeCode, err := s.orderTypeRepo.FindCodeByID(ctx, orderTypeID)
	if err != nil {
		return nil, apperrors.NewHttpError(http.StatusNotFound, "Тип заявки с таким ID не найден", err, nil)
	}

	requiredFields := []string{}

	if rules, ok := ValidationRegistry[orderTypeCode]; ok {
		for _, rule := range rules {
			requiredFields = append(requiredFields, rule.FieldName)
		}
	}

	response := map[string]interface{}{
		"required_fields": requiredFields,
	}

	return response, nil
}

func resolveNewExecutor(
	ctx context.Context,
	s *OrderService,
	tx pgx.Tx,
	updateDTO dto.UpdateOrderDTO,
	originalOrder *entities.Order,
	actor *entities.User,
	permissions map[string]bool,
	txID *uuid.UUID,
) (*uint64, error) {
	if updateDTO.ExecutorID != nil && !utils.AreUint64PointersEqual(updateDTO.ExecutorID, originalOrder.ExecutorID) {
		return updateDTO.ExecutorID, nil
	}

	if updateDTO.DepartmentID != nil && *updateDTO.DepartmentID != originalOrder.DepartmentID {

		authContext := authz.Context{Actor: actor, Target: originalOrder, Permissions: map[string]bool{}}
		if !authz.CanDo(authz.OrdersUpdateDepartmentID, authContext) {
			return nil, apperrors.ErrForbidden
		}

		s.logger.Debug("Департамент изменен, запускаем движок для поиска нового исполнителя")

		result, err := s.ruleEngine.ResolveExecutor(ctx, tx, OrderContext{
			OrderTypeID:  originalOrder.OrderTypeID,
			DepartmentID: *updateDTO.DepartmentID,
			OtdelID:      originalOrder.OtdelID,
		}, nil)
		if err != nil {
			return nil, err // Возвращаем ошибку от RuleEngine
		}
		if result.Executor == nil || result.Executor.ID == 0 {
			return nil, apperrors.NewHttpError(http.StatusConflict, "Не найден исполнитель для заявки в целевом департаменте.", nil, nil)
		}

		if originalOrder.ExecutorID == nil || *originalOrder.ExecutorID != result.Executor.ID {

			record := &entities.OrderHistory{
				OrderID: originalOrder.ID, UserID: actor.ID, EventType: "DEPARTMENT_CHANGE", TxID: txID,
				OldValue: utils.StringToNullString(fmt.Sprintf("Департамент ID %d", originalOrder.DepartmentID)),
				NewValue: utils.StringToNullString(fmt.Sprintf("Департамент ID %d", *updateDTO.DepartmentID)),
				// Можно добавить metadata с ID
			}
			s.historyRepo.CreateInTx(ctx, tx, record, nil)

			// Обновляем саму заявку в памяти
			originalOrder.DepartmentID = result.DepartmentID
			originalOrder.OtdelID = result.OtdelID

			return &result.Executor.ID, nil
		}
	}

	// Если ни один сценарий не сработал, значит исполнитель не меняется.
	return nil, nil
}

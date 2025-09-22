package services

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	"request-system/config"
	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/filestorage"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// Интерфейс
type OrderServiceInterface interface {
	GetOrders(ctx context.Context, filter types.Filter) (*dto.OrderListResponseDTO, error)
	FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error)
	CreateOrder(ctx context.Context, data string, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	DeleteOrder(ctx context.Context, orderID uint64) error
}

// Структура сервиса
type OrderService struct {
	txManager    repositories.TxManagerInterface
	orderRepo    repositories.OrderRepositoryInterface
	userRepo     repositories.UserRepositoryInterface
	statusRepo   repositories.StatusRepositoryInterface
	priorityRepo repositories.PriorityRepositoryInterface
	attachRepo   repositories.AttachmentRepositoryInterface
	historyRepo  repositories.OrderHistoryRepositoryInterface
	fileStorage  filestorage.FileStorageInterface
	logger       *zap.Logger
}

// Конструктор
func NewOrderService(
	txManager repositories.TxManagerInterface, orderRepo repositories.OrderRepositoryInterface,
	userRepo repositories.UserRepositoryInterface, statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface, attachRepo repositories.AttachmentRepositoryInterface,
	historyRepo repositories.OrderHistoryRepositoryInterface, fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) OrderServiceInterface {
	return &OrderService{
		txManager:    txManager,
		orderRepo:    orderRepo,
		userRepo:     userRepo,
		statusRepo:   statusRepo,
		priorityRepo: priorityRepo,
		attachRepo:   attachRepo,
		historyRepo:  historyRepo,
		fileStorage:  fileStorage,
		logger:       logger,
	}
}

// Файл: internal/services/order_service.go
// ЗАМЕНИ ВЕСЬ МЕТОД CreateOrder

// Файл: internal/services/order_service.go
// ЗАМЕНИТЕ ВЕСЬ ЭТОТ МЕТОД

func (s *OrderService) CreateOrder(ctx context.Context, data string, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	// 1. Авторизация на 'order:create' и парсинг данных
	authContext, err := s.buildAuthzContext(ctx, 0)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	actor := authContext.Actor
	creatorID := actor.ID

	var createDTO dto.CreateOrderDTO
	if err = json.Unmarshal([]byte(data), &createDTO); err != nil {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный JSON в поле 'data'", err, nil)
	}

	s.logger.Debug("Получены данные для создания заявки (CreateOrder)", zap.Any("createDTO", createDTO))

	var finalOrderID uint64
	var finalExecutor entities.User

	// 2. Выполнение в транзакции
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		// Блок определения исполнителя
		var executorIDToFind uint64
		canDelegate := authContext.HasPermission(authz.Superuser) || authContext.HasPermission(authz.OrdersDelegate)

		if canDelegate && createDTO.ExecutorID != nil && *createDTO.ExecutorID > 0 {
			// Случай 1: Пользователь ИМЕЕТ ПРАВО назначать И он УКАЗАЛ исполнителя.
			s.logger.Debug("Пользователь с правом делегирования создает заявку с указанием исполнителя.",
				zap.Uint64("actorID", actor.ID), zap.Uint64p("executorID", createDTO.ExecutorID))
			executorIDToFind = *createDTO.ExecutorID
		} else {
			// Случай 2: Пользователь НЕ ИМЕЕТ ПРАВА назначать ИЛИ имеет право, но НЕ УКАЗАЛ исполнителя.
			s.logger.Debug("Автоматическое назначение руководителя департамента в качестве исполнителя.", zap.Uint64("actorID", actor.ID))
			head, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, createDTO.DepartmentID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusNotFound, "Не удалось найти руководителя для указанного департамента.", err, nil)
			}
			executorIDToFind = head.ID
		}

		executor, err := s.userRepo.FindUserByID(ctx, executorIDToFind)
		if err != nil {
			return apperrors.NewHttpError(http.StatusNotFound, "Указанный или автоматический исполнитель не найден в системе.", err, nil)
		}
		finalExecutor = *executor

		// Блок определения статуса
		status, err := s.statusRepo.FindByCode(ctx, "OPEN")
		if err != nil {
			return apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации: статус 'OPEN' не найден", err, nil)
		}

		// Блок проверки приоритета
		if createDTO.PriorityID != nil && *createDTO.PriorityID > 0 {
			if _, err := s.priorityRepo.FindByID(ctx, *createDTO.PriorityID); err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Указанный приоритет не найден", err, nil)
			}
		}

		// Блок обработки Duration (срок выполнения)
		var durationTime *time.Time
		if createDTO.Duration != nil && *createDTO.Duration != "" {
			parsedTime, err := time.Parse(time.RFC3339, *createDTO.Duration)
			if err != nil {
				s.logger.Warn("Неверный формат даты в поле duration", zap.Stringp("duration", createDTO.Duration), zap.Error(err))
				return apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат даты в поле duration. Ожидается формат RFC3339 (например, '2025-09-20T12:00:00Z')", err, nil)
			}
			durationTime = &parsedTime
		}

		// Сборка финальной Entity для сохранения в БД
		orderEntity := &entities.Order{
			Name:            createDTO.Name,
			Address:         &createDTO.Address,
			DepartmentID:    createDTO.DepartmentID,
			OtdelID:         createDTO.OtdelID,
			BranchID:        createDTO.BranchID,
			OfficeID:        createDTO.OfficeID,
			EquipmentID:     createDTO.EquipmentID,
			EquipmentTypeID: createDTO.EquipmentTypeID,
			StatusID:        status.ID,
			PriorityID:      createDTO.PriorityID,
			CreatorID:       creatorID,
			ExecutorID:      &finalExecutor.ID,
			Duration:        durationTime,
		}

		// Сохранение и запись в историю
		orderID, err := s.orderRepo.Create(ctx, tx, orderEntity)
		if err != nil {
			return err
		}
		finalOrderID = orderID

		historyCreate := &entities.OrderHistory{OrderID: orderID, UserID: creatorID, EventType: "CREATE"}
		if err := s.historyRepo.CreateInTx(ctx, tx, historyCreate, nil); err != nil {
			return err
		}

		historyDelegate := &entities.OrderHistory{OrderID: orderID, UserID: creatorID, EventType: "DELEGATION", NewValue: &finalExecutor.Fio}
		if err := s.historyRepo.CreateInTx(ctx, tx, historyDelegate, nil); err != nil {
			return err
		}

		if createDTO.Comment != nil && *createDTO.Comment != "" {
			historyComment := &entities.OrderHistory{OrderID: orderID, UserID: creatorID, EventType: "COMMENT", Comment: createDTO.Comment}
			if err := s.historyRepo.CreateInTx(ctx, tx, historyComment, nil); err != nil {
				return err
			}
		}

		if file != nil {
			return s.attachFileToOrderInTx(ctx, tx, file, orderID, creatorID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Формирование финального ответа
	createdOrder, err := s.orderRepo.FindByID(ctx, finalOrderID)
	if err != nil {
		return nil, err
	}
	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, finalOrderID, 50, 0)

	return buildOrderResponse(createdOrder, actor, &finalExecutor, attachments), nil
}

func (s *OrderService) UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	// 1. ПОДГОТОВКА И АВТОРИЗАЦИЯ
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	actor := authContext.Actor
	orderToUpdate := authContext.Target.(*entities.Order)
	currentStatus, err := s.statusRepo.FindStatus(ctx, orderToUpdate.StatusID)
	if err != nil {
		s.logger.Error("UpdateOrder: Не удалось получить текущий статус заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}

	// 2. ВЫПОЛНЕНИЕ В ТРАНЗАКЦИИ
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		hasChanges := false

		// Определяем роли для проверок
		isCreator := actor.ID == orderToUpdate.CreatorID
		isExecutor := orderToUpdate.ExecutorID != nil && actor.ID == *orderToUpdate.ExecutorID
		isManager := actor.DepartmentID == orderToUpdate.DepartmentID && authContext.HasPermission(authz.OrdersDelegate)
		isGlobalAdmin := authContext.HasPermission(authz.ScopeAll)
		isSuperuser := authContext.HasPermission(authz.Superuser)

		// Блок 1: Смена Департамента
		if updateDTO.DepartmentID != nil && (isGlobalAdmin || isSuperuser) && *updateDTO.DepartmentID != orderToUpdate.DepartmentID {
			newDepartmentID := *updateDTO.DepartmentID
			newExecutor, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, newDepartmentID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusNotFound, "В целевом департаменте не найден руководитель.", err, nil)
			}

			historyDeptChange := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DEPARTMENT_CHANGE"}
			if err := s.historyRepo.CreateInTx(ctx, tx, historyDeptChange, nil); err != nil {
				return fmt.Errorf("ошибка истории (смена департамента): %w", err)
			}
			historyNewExecutor := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION", NewValue: &newExecutor.Fio}
			if err = s.historyRepo.CreateInTx(ctx, tx, historyNewExecutor, nil); err != nil {
				return fmt.Errorf("ошибка истории (авто-делегирование): %w", err)
			}

			orderToUpdate.DepartmentID = newDepartmentID
			orderToUpdate.ExecutorID = &newExecutor.ID
			hasChanges = true
		}

		// Блок 2: Смена Исполнителя
		if updateDTO.ExecutorID != nil && (isManager || isGlobalAdmin || isSuperuser) && !utils.AreUint64PointersEqual(updateDTO.ExecutorID, orderToUpdate.ExecutorID) {
			newExec, err := s.userRepo.FindUserByID(ctx, *updateDTO.ExecutorID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Указанный исполнитель не найден.", err, nil)
			}

			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION", NewValue: &newExec.Fio}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (делегирование): %w", err)
			}

			orderToUpdate.ExecutorID = updateDTO.ExecutorID
			hasChanges = true
		}

		// Блок 3: Изменение Названия и Адреса
		if (currentStatus.Code == "OPEN" && isCreator) || isGlobalAdmin || isSuperuser {
			if updateDTO.Name != nil && *updateDTO.Name != orderToUpdate.Name {
				history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "NAME_CHANGE", NewValue: updateDTO.Name}
				if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
					return fmt.Errorf("ошибка истории (смена названия): %w", err)
				}
				orderToUpdate.Name = *updateDTO.Name
				hasChanges = true
			}
			if updateDTO.Address != nil && *updateDTO.Address != *orderToUpdate.Address {
				history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "ADDRESS_CHANGE", NewValue: updateDTO.Address}
				if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
					return fmt.Errorf("ошибка истории (смена адреса): %w", err)
				}
				*orderToUpdate.Address = *updateDTO.Address
				hasChanges = true
			}
		}

		// Блок 4: Обновление доп. полей (Otdel, Branch и т.д.)
		if isManager || isGlobalAdmin || isSuperuser {
			if updateDTO.OtdelID != nil && !utils.AreUint64PointersEqual(updateDTO.OtdelID, orderToUpdate.OtdelID) {
				orderToUpdate.OtdelID = updateDTO.OtdelID
				hasChanges = true
			}
			if updateDTO.BranchID != nil && !utils.AreUint64PointersEqual(updateDTO.BranchID, orderToUpdate.BranchID) {
				orderToUpdate.BranchID = updateDTO.BranchID
				hasChanges = true
			}
			if updateDTO.OfficeID != nil && !utils.AreUint64PointersEqual(updateDTO.OfficeID, orderToUpdate.OfficeID) {
				orderToUpdate.OfficeID = updateDTO.OfficeID
				hasChanges = true
			}
			if updateDTO.EquipmentID != nil && !utils.AreUint64PointersEqual(updateDTO.EquipmentID, orderToUpdate.EquipmentID) {
				orderToUpdate.EquipmentID = updateDTO.EquipmentID
				hasChanges = true
			}
			if updateDTO.EquipmentTypeID != nil && !utils.AreUint64PointersEqual(updateDTO.EquipmentTypeID, orderToUpdate.EquipmentTypeID) {
				orderToUpdate.EquipmentTypeID = updateDTO.EquipmentTypeID
				hasChanges = true
			}
			if updateDTO.PriorityID != nil && !utils.AreUint64PointersEqual(updateDTO.PriorityID, orderToUpdate.PriorityID) {
				priority, err := s.priorityRepo.FindPriority(ctx, *updateDTO.PriorityID)
				if err != nil {
					return apperrors.NewHttpError(http.StatusBadRequest, "Целевой приоритет не найден.", err, nil)
				}

				history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "PRIORITY_CHANGE", NewValue: &priority.Name}
				if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
					return fmt.Errorf("ошибка истории (смена приоритета): %w", err)
				}

				orderToUpdate.PriorityID = updateDTO.PriorityID
				hasChanges = true
			}
		}

		// Блок 5: Смена Статуса
		if updateDTO.StatusID != nil && *updateDTO.StatusID != orderToUpdate.StatusID {
			newStatus, err := s.statusRepo.FindStatus(ctx, *updateDTO.StatusID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Целевой статус не найден.", err, nil)
			}

			canChange := false
			isReopening := currentStatus.Code == "CLOSED" && newStatus.Code == "OPEN"
			if isSuperuser {
				canChange = true
			} else if isReopening {
				canChange = isGlobalAdmin || (isCreator && authContext.HasPermission(authz.OrdersReopen))
			} else {
				if currentStatus.Code == "CLOSED" {
					return apperrors.NewHttpError(http.StatusBadRequest, "Закрытую заявку менять нельзя.", nil, nil)
				}
				canChange = isExecutor || isManager || isGlobalAdmin
			}

			if !canChange {
				return apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав на изменение статуса.", nil, nil)
			}

			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "STATUS_CHANGE", NewValue: &newStatus.Name}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (смена статуса): %w", err)
			}

			orderToUpdate.StatusID = *updateDTO.StatusID
			hasChanges = true
		}

		// Блок 6: Смена Срока
		if updateDTO.Duration != nil && (isManager || isGlobalAdmin || isSuperuser) {
			parsedTime, err := time.Parse(time.RFC3339, *updateDTO.Duration)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат даты в поле duration.", err, nil)
			}

			timeForHistory := parsedTime.Format("02.01.2006 15:04")
			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DURATION_CHANGE", NewValue: &timeForHistory}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (смена срока): %w", err)
			}

			orderToUpdate.Duration = &parsedTime
			hasChanges = true
		}

		// Блок 7: Комментарий и Файл
		if updateDTO.Comment != nil && *updateDTO.Comment != "" {
			historyComment := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "COMMENT", Comment: updateDTO.Comment}
			if err := s.historyRepo.CreateInTx(ctx, tx, historyComment, nil); err != nil {
				return fmt.Errorf("ошибка истории (комментарий): %w", err)
			}
		}
		if file != nil {
			if err = s.attachFileToOrderInTx(ctx, tx, file, orderID, actor.ID); err != nil {
				return err
			}
		}

		// 3. СОХРАНЕНИЕ
		if hasChanges {
			if err = s.orderRepo.Update(ctx, tx, orderToUpdate); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 4. ФОРМИРОВАНИЕ ОТВЕТА
	finalOrder, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, apperrors.ErrInternalServer
	}
	creator, _ := s.userRepo.FindUserByID(ctx, finalOrder.CreatorID)
	var executor *entities.User
	if finalOrder.ExecutorID != nil {
		executor, _ = s.userRepo.FindUserByID(ctx, *finalOrder.ExecutorID)
	}
	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, finalOrder.ID, 50, 0)

	return buildOrderResponse(finalOrder, creator, executor, attachments), nil
}

func (s *OrderService) GetOrders(ctx context.Context, filter types.Filter) (*dto.OrderListResponseDTO, error) {
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
	if !(permissionsMap[authz.Superuser] || permissionsMap[authz.ScopeAll]) {
		if permissionsMap[authz.ScopeDepartment] {
			securityFilter = "department_id = ?"
			securityArgs = append(securityArgs, actor.DepartmentID)
		} else if permissionsMap[authz.ScopeOwn] {
			securityFilter = "(user_id = ? OR executor_id = ?)"
			securityArgs = append(securityArgs, actor.ID, actor.ID)
		} else {
			return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
		}
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
		if _, ok := userIDsMap[order.CreatorID]; !ok {
			userIDsMap[order.CreatorID] = struct{}{}
		}
		if order.ExecutorID != nil {
			if _, ok := userIDsMap[*order.ExecutorID]; !ok {
				userIDsMap[*order.ExecutorID] = struct{}{}
			}
		}
	}

	userIDs := make([]uint64, 0, len(userIDsMap))
	for id := range userIDsMap {
		userIDs = append(userIDs, id)
	}

	usersMap, err := s.userRepo.FindUsersByIDs(ctx, userIDs)
	if err != nil {
		s.logger.Error("GetOrders: не удалось получить пользователей по IDs", zap.Error(err))
		usersMap = make(map[uint64]entities.User)
	}

	attachmentsMap, err := s.attachRepo.FindAttachmentsByOrderIDs(ctx, orderIDs)
	if err != nil {
		s.logger.Error("GetOrders: не удалось получить вложения по IDs", zap.Error(err))
		attachmentsMap = make(map[uint64][]entities.Attachment)
	}

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
	return &dto.OrderListResponseDTO{List: dtos, TotalCount: totalCount}, nil
}

func (s *OrderService) FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error) {
	s.logger.Info("--- ЗАПУСК FindOrderByID ---", zap.Uint64("orderID", orderID))
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		s.logger.Warn("--- ОШИБКА в orderRepo.FindByID ---", zap.Error(err))
		return nil, err
	}
	s.logger.Info("--- Заявка успешно найдена в repo ---", zap.Any("order", order))

	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	authContext := authz.Context{Actor: actor, Permissions: permissionsMap, Target: order}
	if !authz.CanDo(authz.OrdersView, authContext) {
		s.logger.Warn("--- ДОСТУП ЗАПРЕЩЕН со стороны authz.CanDo ---")
		return nil, apperrors.ErrForbidden
	}
	s.logger.Info("--- Права успешно проверены ---")

	creator, _ := s.userRepo.FindUserByID(ctx, order.CreatorID)
	var executor *entities.User
	if order.ExecutorID != nil && *order.ExecutorID > 0 {
		executor, _ = s.userRepo.FindUserByID(ctx, *order.ExecutorID)
	}
	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, order.ID, 50, 0)
	return buildOrderResponse(order, creator, executor, attachments), nil
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
	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	var targetOrder *entities.Order
	if orderID > 0 {
		foundOrder, err := s.orderRepo.FindByID(ctx, orderID)
		if err != nil {
			return nil, err
		}
		targetOrder = foundOrder
	}
	return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: targetOrder}, nil
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
			ID: att.ID, FileName: att.FileName, FileSize: att.FileSize, FileType: att.FileType, URL: att.FilePath,
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

	// БЛОК С ЛИШНЕЙ ПЕРЕМЕННОЙ `priorityID` ПОЛНОСТЬЮ УДАЛЕН.
	// Мы просто передаем значение напрямую.

	return &dto.OrderResponseDTO{
		ID:              order.ID,
		Name:            order.Name,
		Address:         *order.Address,
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

func (s *OrderService) attachFileToOrderInTx(ctx context.Context, tx pgx.Tx, file *multipart.FileHeader, orderID, userID uint64) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	const uploadContext = "order_document"
	if err = utils.ValidateFile(file, src, uploadContext); err != nil {
		return fmt.Errorf("файл не прошел валидацию: %w", err)
	}

	rules, _ := config.UploadContexts[uploadContext]
	relativePath, err := s.fileStorage.Save(src, file.Filename, rules.PathPrefix)
	if err != nil {
		return fmt.Errorf("не удалось сохранить файл: %w", err)
	}

	fullFilePath := "/uploads/" + relativePath
	attach := &entities.Attachment{
		OrderID: orderID, UserID: userID, FileName: file.Filename,
		FilePath: fullFilePath, FileType: file.Header.Get("Content-Type"), FileSize: file.Size,
	}

	attachmentID, err := s.attachRepo.Create(ctx, tx, attach)
	if err != nil {
		return fmt.Errorf("не удалось создать вложение: %w", err)
	}

	attachHistory := &entities.OrderHistory{
		OrderID: orderID, UserID: userID, EventType: "ATTACHMENT_ADDED", NewValue: &file.Filename,
	}
	return s.historyRepo.CreateInTx(ctx, tx, attachHistory, &attachmentID)
}

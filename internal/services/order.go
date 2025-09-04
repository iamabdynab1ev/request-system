// internal/services/order_service.go

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

// Интерфейс с правильной сигнатурой UpdateOrder
type OrderServiceInterface interface {
	GetOrders(ctx context.Context, filter types.Filter) (*dto.OrderListResponseDTO, error)
	FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error)
	CreateOrder(ctx context.Context, data string, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	DeleteOrder(ctx context.Context, orderID uint64) error
}

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

func NewOrderService(
	txManager repositories.TxManagerInterface, orderRepo repositories.OrderRepositoryInterface,
	userRepo repositories.UserRepositoryInterface, statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface, attachRepo repositories.AttachmentRepositoryInterface,
	historyRepo repositories.OrderHistoryRepositoryInterface, fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) OrderServiceInterface {
	return &OrderService{
		txManager: txManager, orderRepo: orderRepo, userRepo: userRepo, statusRepo: statusRepo,
		priorityRepo: priorityRepo, attachRepo: attachRepo, historyRepo: historyRepo,
		fileStorage: fileStorage, logger: logger,
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, data string, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, 0)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	creatorID := authContext.Actor.ID
	var createDTO dto.CreateOrderDTO
	if err = json.Unmarshal([]byte(data), &createDTO); err != nil {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный JSON в поле 'data'", err, nil)
	}
	s.logger.Debug("Получены данные для создания заявки (CreateOrder)", zap.Any("createDTO", createDTO))

	var finalOrderID uint64
	var executorFio string // Сохраняем ФИО, чтобы не делать лишний запрос

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		status, err := s.statusRepo.FindByCode(ctx, "OPEN")
		if err != nil {
			return err
		}

		priority, err := s.priorityRepo.FindByCode(ctx, "MEDIUM")
		if err != nil {
			return err
		}

		executor, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, createDTO.DepartmentID)
		if err != nil {
			return err
		}
		executorFio = executor.Fio // <--- Сохраняем

		orderEntity := &entities.Order{
			Name:         createDTO.Name,
			Address:      createDTO.Address,
			DepartmentID: createDTO.DepartmentID,
			OtdelID:      createDTO.OtdelID,
			BranchID:     createDTO.BranchID,
			OfficeID:     createDTO.OfficeID,
			EquipmentID:  createDTO.EquipmentID,
			StatusID:     status.ID,
			PriorityID:   priority.ID,
			CreatorID:    creatorID,
			ExecutorID:   executor.ID,
		}

		orderID, err := s.orderRepo.Create(ctx, tx, orderEntity)
		if err != nil {
			return err
		}
		finalOrderID = orderID

		historyCreate := &entities.OrderHistory{OrderID: orderID, UserID: creatorID, EventType: "CREATE"}
		if err := s.historyRepo.CreateInTx(ctx, tx, historyCreate, nil); err != nil {
			return err
		}

		historyDelegate := &entities.OrderHistory{OrderID: orderID, UserID: creatorID, EventType: "DELEGATION", NewValue: &executor.Fio}
		if err := s.historyRepo.CreateInTx(ctx, tx, historyDelegate, nil); err != nil {
			return err
		}

		if createDTO.Comment != nil && *createDTO.Comment != "" {
			historyComment := &entities.OrderHistory{
				OrderID: orderID, UserID: creatorID, EventType: "COMMENT", Comment: createDTO.Comment,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, historyComment, nil); err != nil {
				return fmt.Errorf("не удалось создать запись в истории (комментарий): %w", err)
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

	// <<<--- НАЧАЛО ИСПРАВЛЕНИЙ ---
	// Теперь мы вручную собираем все данные для ответа, как и в других функциях

	createdOrder, err := s.orderRepo.FindByID(ctx, finalOrderID)
	if err != nil {
		return nil, err
	}

	// Данные создателя - это текущий пользователь, мы их уже знаем
	creator := authContext.Actor

	// Данные исполнителя - мы их получили в транзакции
	executor := &entities.User{
		ID:  createdOrder.ExecutorID,
		Fio: executorFio,
	}

	// Вложения (их не будет, т.к. это новое обращение, но на всякий случай проверяем)
	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, finalOrderID, 50, 0)

	// Вызываем независимую функцию `buildOrderResponse`
	return buildOrderResponse(createdOrder, creator, executor, attachments), nil
	// <<<--- КОНЕЦ ИСПРАВЛЕНИЙ ---
}

// Реализация с правильной сигнатурой UpdateOrder
// Файл: internal/services/order_service.go

// UpdateOrder - ПОЛНАЯ ИСПРАВЛЕННАЯ ВЕРСИЯ
func (s *OrderService) UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	// --- 1. Подготовка и Авторизация (блок остается без изменений) ---
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		s.logger.Error("UpdateOrder: Ошибка построения контекста авторизации", zap.Uint64("orderID", orderID), zap.Error(err))
		return nil, err
	}
	if !authz.CanDo(authz.OrdersUpdate, *authContext) {
		s.logger.Warn("UpdateOrder: Отказано в доступе на обновление заявки (базовое право)", zap.Uint64("actorID", authContext.Actor.ID), zap.Uint64("orderID", orderID))
		return nil, apperrors.ErrForbidden
	}

	actor := authContext.Actor
	orderToUpdate := authContext.Target.(*entities.Order)

	currentStatus, err := s.statusRepo.FindStatus(ctx, orderToUpdate.StatusID)
	if err != nil {
		s.logger.Error("UpdateOrder: Не удалось получить текущий статус заявки", zap.Uint64("orderID", orderID), zap.Uint64("statusID", orderToUpdate.StatusID), zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}

	isCreator := actor.ID == orderToUpdate.CreatorID
	isExecutor := actor.ID == orderToUpdate.ExecutorID
	// ПРЕДУПРЕЖДЕНИЕ: Проверьте, что у вас есть 'RoleName' в структуре 'actor' и роль называется "User". Если роль руководителя называется иначе, измените здесь.
	isDepartmentHead := actor.DepartmentID == orderToUpdate.DepartmentID && (actor.RoleName == "User")
	isGlobalAdmin := authContext.HasPermission(authz.ScopeAll)
	isSuperuser := authContext.HasPermission(authz.Superuser)

	// --- 2. Выполнение операций в транзакции ---
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		s.logger.Debug("Получены данные для обновления заявки (UpdateOrder)", zap.Any("updateDTO", updateDTO))
		hasChanges := false

		// 2.1. Смена Департамента (только админ)
		if updateDTO.DepartmentID != nil && (isGlobalAdmin || isSuperuser) && *updateDTO.DepartmentID != orderToUpdate.DepartmentID {
			historyDeptChange := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "DEPARTMENT_CHANGE",
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, historyDeptChange, nil); err != nil {
				return fmt.Errorf("ошибка истории (смена департамента): %w", err)
			}
			newDepartmentID := *updateDTO.DepartmentID
			newExecutor, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, newDepartmentID)
			if err != nil {
				s.logger.Warn("UpdateOrder: Не найден руководитель в новом департаменте", zap.Uint64("newDepartmentID", newDepartmentID), zap.Error(err))
				return apperrors.NewHttpError(http.StatusNotFound, "В целевом департаменте не найден руководитель.", err, nil)
			}
			orderToUpdate.DepartmentID = newDepartmentID
			orderToUpdate.ExecutorID = newExecutor.ID
			hasChanges = true

			historyNewExecutor := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION", NewValue: &newExecutor.Fio,
			}
			if err = s.historyRepo.CreateInTx(ctx, tx, historyNewExecutor, nil); err != nil {
				return fmt.Errorf("ошибка истории (авто-делегирование): %w", err)
			}
		}

		// 2.2. Смена Исполнителя (Ручная делегация)
		if updateDTO.ExecutorID != nil && (isDepartmentHead || isGlobalAdmin || isSuperuser) && *updateDTO.ExecutorID != orderToUpdate.ExecutorID {
			newExec, err := s.userRepo.FindUserByID(ctx, *updateDTO.ExecutorID)
			if err != nil {
				s.logger.Warn("UpdateOrder: Не найден указанный исполнитель", zap.Uint64("newExecutorID", *updateDTO.ExecutorID), zap.Error(err))
				return apperrors.NewHttpError(http.StatusBadRequest, "Указанный исполнитель не найден.", err, nil)
			}
			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION", NewValue: &newExec.Fio}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (делегирование): %w", err)
			}
			orderToUpdate.ExecutorID = *updateDTO.ExecutorID
			hasChanges = true
		}

		// 2.3. Изменение Названия и Адреса
		// <<< ИСПРАВЛЕНО: Теперь администраторы тоже могут менять эти поля >>>
		if (currentStatus.Code == "OPEN" && isCreator) || isGlobalAdmin || isSuperuser {
			if updateDTO.Name != nil && *updateDTO.Name != orderToUpdate.Name {
				history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "NAME_CHANGE", NewValue: updateDTO.Name}
				if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
					return fmt.Errorf("ошибка истории (смена названия): %w", err)
				}
				orderToUpdate.Name = *updateDTO.Name
				hasChanges = true
			}
			if updateDTO.Address != nil && *updateDTO.Address != orderToUpdate.Address {
				history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "ADDRESS_CHANGE", NewValue: updateDTO.Address}
				if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
					return fmt.Errorf("ошибка истории (смена адреса): %w", err)
				}
				orderToUpdate.Address = *updateDTO.Address
				hasChanges = true
			}
		}

		// <<< ДОБАВЛЕН БЛОК: Логика для обновления ранее игнорируемых полей >>>
		// Права на эти изменения даны руководителю и администраторам
		if isDepartmentHead || isGlobalAdmin || isSuperuser {
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
		}

		// 2.4. Смена Статуса
		if updateDTO.StatusID != nil && *updateDTO.StatusID != orderToUpdate.StatusID {
			newStatus, err := s.statusRepo.FindStatus(ctx, *updateDTO.StatusID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Целевой статус не найден.", err, nil)
			}

			isReopening := currentStatus.Code == "CLOSED" && newStatus.Code == "OPEN"
			canChangeThisStatus := false
			if isSuperuser {
				canChangeThisStatus = true
			} else if isReopening {
				canChangeThisStatus = isGlobalAdmin || (isCreator && authContext.HasPermission(authz.OrdersReopen))
			} else {
				if currentStatus.Code == "CLOSED" {
					return apperrors.NewHttpError(http.StatusBadRequest, "Невозможно изменить закрытую заявку (кроме повторного открытия).", nil, nil)
				}
				canChangeThisStatus = isExecutor || isDepartmentHead || isGlobalAdmin
			}

			if !canChangeThisStatus {
				return apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав на изменение статуса этой заявки.", nil, nil)
			}

			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "STATUS_CHANGE", NewValue: &newStatus.Name}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (смена статуса): %w", err)
			}
			orderToUpdate.StatusID = *updateDTO.StatusID
			hasChanges = true
		}

		// 2.5. Смена Приоритета
		if updateDTO.PriorityID != nil && *updateDTO.PriorityID != orderToUpdate.PriorityID && (isDepartmentHead || isGlobalAdmin || isSuperuser) {
			priority, err := s.priorityRepo.FindPriority(ctx, *updateDTO.PriorityID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Целевой приоритет не найден.", err, nil)
			}
			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "PRIORITY_CHANGE", NewValue: &priority.Name}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (смена приоритета): %w", err)
			}
			orderToUpdate.PriorityID = *updateDTO.PriorityID
			hasChanges = true
		}

		// 2.6. Смена Длительности (Дедлайн)
		if updateDTO.Duration != nil && (isDepartmentHead || isGlobalAdmin || isSuperuser) {
			parsedTime, err := time.Parse(time.RFC3339, *updateDTO.Duration)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат даты в поле duration", err, nil)
			}
			timeForHistory := parsedTime.Format("02.01.2006 15:04")
			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DURATION_CHANGE", NewValue: &timeForHistory}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (смена срока): %w", err)
			}
			orderToUpdate.Duration = &parsedTime
			hasChanges = true
		}

		// 2.7. Добавление Комментария
		if updateDTO.Comment != nil && *updateDTO.Comment != "" {
			historyComment := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "COMMENT", Comment: updateDTO.Comment,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, historyComment, nil); err != nil {
				return fmt.Errorf("ошибка истории (добавление комментария): %w", err)
			}
		}

		// 2.8. Прикрепление файла
		if file != nil {
			if err = s.attachFileToOrderInTx(ctx, tx, file, orderID, actor.ID); err != nil {
				return err
			}
		}

		// 3. Сохранение всех изменений в базе данных
		if hasChanges {
			s.logger.Info("Обнаружены изменения, выполняется обновление заявки в БД", zap.Uint64("orderID", orderID))
			if err = s.orderRepo.Update(ctx, tx, orderToUpdate); err != nil {
				return err
			}
		} else {
			s.logger.Info("Изменений для заявки не обнаружено, обновление БД не требуется", zap.Uint64("orderID", orderID))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 4. Формирование ответа (блок остается без изменений)
	finalOrder, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		s.logger.Error("UpdateOrder: Не удалось найти заявку после обновления", zap.Uint64("orderID", orderID), zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}

	creator, _ := s.userRepo.FindUserByID(ctx, finalOrder.CreatorID)
	executor, _ := s.userRepo.FindUserByID(ctx, finalOrder.ExecutorID)
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

	// === НАЧАЛО НОВОЙ ОПТИМИЗИРОВАННОЙ ЛОГИКИ ===

	// Шаг 1: Получаем базовый список заявок (1-й SQL-запрос)
	orders, totalCount, err := s.orderRepo.GetOrders(ctx, filter, securityFilter, securityArgs)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
	}

	// Шаг 2: Собираем все ID, которые нам нужно будет запросить дополнительно
	creatorIDs := make([]uint64, 0)
	executorIDs := make([]uint64, 0)
	orderIDs := make([]uint64, 0, len(orders))
	// Используем карты, чтобы собрать только УНИКАЛЬНЫЕ ID пользователей
	userIDsMap := make(map[uint64]struct{})

	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
		if order.CreatorID > 0 {
			if _, ok := userIDsMap[order.CreatorID]; !ok {
				userIDsMap[order.CreatorID] = struct{}{}
				creatorIDs = append(creatorIDs, order.CreatorID)
			}
		}
		if order.ExecutorID > 0 {
			if _, ok := userIDsMap[order.ExecutorID]; !ok {
				userIDsMap[order.ExecutorID] = struct{}{}
				executorIDs = append(executorIDs, order.ExecutorID)
			}
		}
	}

	// Шаг 3: Выполняем "пакетные" запросы (еще 2 SQL-запроса)
	usersMap, err := s.userRepo.FindUsersByIDs(ctx, append(creatorIDs, executorIDs...))
	if err != nil {
		s.logger.Error("GetOrders: не удалось получить пользователей по IDs", zap.Error(err))
		usersMap = make(map[uint64]entities.User) // Инициализируем пустой картой, чтобы избежать паники
	}

	attachmentsMap, err := s.attachRepo.FindAttachmentsByOrderIDs(ctx, orderIDs)
	if err != nil {
		s.logger.Error("GetOrders: не удалось получить вложения по IDs", zap.Error(err))
		attachmentsMap = make(map[uint64][]entities.Attachment)
	}

	// Шаг 4: Собираем финальные DTO без единого запроса к БД
	dtos := make([]dto.OrderResponseDTO, 0, len(orders))
	for i := range orders {
		order := &orders[i]
		creator := usersMap[order.CreatorID]
		executor := usersMap[order.ExecutorID]
		attachments := attachmentsMap[order.ID]

		orderResponse := buildOrderResponse(order, &creator, &executor, attachments)
		dtos = append(dtos, *orderResponse)
	}

	return &dto.OrderListResponseDTO{List: dtos, TotalCount: totalCount}, nil
}

func (s *OrderService) FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error) {
	// <<<--- НАЧАЛО ОТЛАДОЧНОГО КОДА ---
	s.logger.Info("--- ЗАПУСК FindOrderByID ---", zap.Uint64("orderID", orderID))

	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		s.logger.Warn("--- ОШИБКА в orderRepo.FindByID ---", zap.Error(err))
		return nil, err
	}

	// Если код дошел до сюда, значит, заявка нашлась.
	s.logger.Info("--- Заявка успешно найдена в repo ---", zap.Any("order", order))

	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("--- ОШИБКА: не найден actor ---", zap.Error(err))
		return nil, apperrors.ErrUserNotFound
	}

	authContext := authz.Context{Actor: actor, Permissions: permissionsMap, Target: order}
	s.logger.Info("--- Контекст для authz.CanDo собран ---", zap.Any("authContext.Target", authContext.Target))

	// Проверяем права
	if !authz.CanDo(authz.OrdersView, authContext) {
		s.logger.Warn("--- ДОСТУП ЗАПРЕЩЕН со стороны authz.CanDo ---")
		return nil, apperrors.ErrForbidden
	}

	s.logger.Info("--- Права успешно проверены ---")
	// <<<--- КОНЕЦ ОТЛАДОЧНОГО КОДА ---

	// Собираем ответ (этот код остается прежним)
	creator, _ := s.userRepo.FindUserByID(ctx, order.CreatorID)
	executor, _ := s.userRepo.FindUserByID(ctx, order.ExecutorID)
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

	executorDTO := dto.ShortUserDTO{ID: order.ExecutorID}
	if executor != nil && executor.ID != 0 {
		executorDTO.Fio = executor.Fio
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
		// Чтобы в JSON всегда был `[]` вместо `null`, если вложений нет
		attachmentsDTO = make([]dto.AttachmentResponseDTO, 0)
	}

	var durationStr *string
	if order.Duration != nil {
		formatted := order.Duration.Format(time.RFC3339)
		durationStr = &formatted
	}

	return &dto.OrderResponseDTO{
		ID:           order.ID,
		Name:         order.Name,
		Address:      order.Address,
		Creator:      creatorDTO,
		Executor:     executorDTO,
		DepartmentID: order.DepartmentID,
		OtdelID:      order.OtdelID,
		BranchID:     order.BranchID,
		OfficeID:     order.OfficeID,
		EquipmentID:  order.EquipmentID,
		StatusID:     order.StatusID,
		PriorityID:   order.PriorityID,
		Attachments:  attachmentsDTO,
		Duration:     durationStr,
		CreatedAt:    order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    order.UpdatedAt.Format(time.RFC3339),
	}
}

func (s *OrderService) attachFileToOrderInTx(ctx context.Context, tx pgx.Tx, file *multipart.FileHeader, orderID, userID uint64) error {
	// Этот метод остается как есть, без изменений.
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
	attach := &entities.Attachment{OrderID: orderID, UserID: userID, FileName: file.Filename, FilePath: fullFilePath, FileType: file.Header.Get("Content-Type"), FileSize: file.Size}
	attachmentID, err := s.attachRepo.Create(ctx, tx, attach)
	if err != nil {
		return fmt.Errorf("не удалось создать вложение: %w", err)
	}
	attachHistory := &entities.OrderHistory{OrderID: orderID, UserID: userID, EventType: "ATTACHMENT_ADDED", NewValue: &file.Filename}
	return s.historyRepo.CreateInTx(ctx, tx, attachHistory, &attachmentID)
}

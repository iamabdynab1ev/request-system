package services

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"request-system/config"
	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/filestorage"
	"request-system/pkg/types"
	"request-system/pkg/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type OrderServiceInterface interface {
	GetOrders(ctx context.Context, filter types.Filter) (*dto.OrderListResponseDTO, error)
	FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error)
	CreateOrder(ctx context.Context, data string, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	DelegateOrder(ctx context.Context, orderID uint64, delegatePayload dto.DelegateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	DeleteOrder(ctx context.Context, orderID uint64) error
	UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
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
	txManager repositories.TxManagerInterface,
	orderRepo repositories.OrderRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface,
	attachRepo repositories.AttachmentRepositoryInterface,
	historyRepo repositories.OrderHistoryRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) OrderServiceInterface {
	return &OrderService{
		txManager: txManager, orderRepo: orderRepo, userRepo: userRepo,
		statusRepo: statusRepo, priorityRepo: priorityRepo, attachRepo: attachRepo,
		historyRepo: historyRepo, fileStorage: fileStorage, logger: logger,
	}
}

func (s *OrderService) UpdateOrder(ctx context.Context, orderID uint64, updateDTO dto.UpdateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// Базовая проверка права 'orders:update' + scope.
	if !authz.CanDo("orders:update", *authContext) {
		s.logger.Warn("Отказано в доступе на редактирование заявки", zap.Uint64("orderID", orderID), zap.Uint64("actorID", authContext.Actor.ID))
		return nil, apperrors.ErrForbidden
	}

	actor := authContext.Actor
	orderToUpdate := authContext.Target.(*entities.Order)

	// Правило 0: Проверка на финальный статус
	currentStatus, err := s.statusRepo.FindStatus(ctx, orderToUpdate.StatusID)
	if err != nil {
		return nil, err
	}

	if currentStatus.Code == "CLOSED" {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Невозможно изменить закрытую заявку. Она находится в финальном статусе.", nil)
	}

	// Определяем контекстные роли пользователя
	isCreator := actor.ID == orderToUpdate.CreatorID
	isExecutor := actor.ID == orderToUpdate.ExecutorID
	isDepartmentHead := actor.RoleName == "User" && actor.DepartmentID == orderToUpdate.DepartmentID
	isGlobalAdmin := actor.RoleName == "Admin" || actor.RoleName == "Super Admin"

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		hasChanges := false

		// Логика смены департамента (только для Админа/СуперАдмина)
		if updateDTO.DepartmentID != nil && isGlobalAdmin && *updateDTO.DepartmentID != orderToUpdate.DepartmentID {
			// --- ОБЪЯВЛЯЕМ oldDepartmentID (уже есть) ---
			oldDepartmentID := orderToUpdate.DepartmentID
			oldExecutorID := orderToUpdate.ExecutorID
			oldExecutorFio := ""
			if oldExecutorID > 0 {
				oldExecutorUser, err := s.userRepo.FindUserByID(ctx, oldExecutorID)
				if err == nil && oldExecutorUser != nil {
					oldExecutorFio = oldExecutorUser.Fio
				} else {
					oldExecutorFio = "неизвестен"
				}
			}

			newDeptID := *updateDTO.DepartmentID
			newHead, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, newDeptID)
			if err != nil {
				return fmt.Errorf("руководитель в новом департаменте не найден: %w", err)
			}

			orderToUpdate.DepartmentID = newDeptID
			orderToUpdate.ExecutorID = newHead.ID
			hasChanges = true

			// --- ИСПРАВЛЕНИЕ: Используем oldDepartmentID в historyComment ---
			historyComment := fmt.Sprintf("Заявка переведена в департамент #%d (была в #%d).", newDeptID, oldDepartmentID) // <--- ИСПОЛЬЗУЕМ ЗДЕСЬ
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "DEPARTMENT_CHANGE", Comment: &historyComment,
			}, nil)

			if oldExecutorID != newHead.ID {
				executorChangeText := fmt.Sprintf("Исполнитель изменен: %s (был: %s).", newHead.Fio, oldExecutorFio)
				s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
					OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION",
					NewValue: &newHead.Fio, Comment: &executorChangeText,
				}, nil)
			}
		}

		// Делегирование (Руководитель или Админ)
		if updateDTO.ExecutorID != nil && (isDepartmentHead || isGlobalAdmin) && *updateDTO.ExecutorID != orderToUpdate.ExecutorID {
			newExecutor, _ := s.userRepo.FindUserByID(ctx, *updateDTO.ExecutorID)
			historyComment := fmt.Sprintf("Назначен новый ответственный: %s", newExecutor.Fio)
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION", NewValue: &newExecutor.Fio, Comment: &historyComment,
			}, nil)
			orderToUpdate.ExecutorID = *updateDTO.ExecutorID
			hasChanges = true
		}

		// Название и Адрес (только Создатель, если статус OPEN)
		if currentStatus.Code == "OPEN" && isCreator {
			if updateDTO.Name != nil && *updateDTO.Name != orderToUpdate.Name {
				historyComment := fmt.Sprintf("Название заявки изменено на: %s", *updateDTO.Name)
				s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
					OrderID: orderID, UserID: actor.ID, EventType: "NAME_CHANGE", Comment: &historyComment,
				}, nil)
				orderToUpdate.Name = *updateDTO.Name
				hasChanges = true
			}
			if updateDTO.Address != nil && *updateDTO.Address != orderToUpdate.Address {
				historyComment := fmt.Sprintf("Адрес заявки изменен на: %s", *updateDTO.Address)
				s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
					OrderID: orderID, UserID: actor.ID, EventType: "ADDRESS_CHANGE", Comment: &historyComment,
				}, nil)
				orderToUpdate.Address = *updateDTO.Address
				hasChanges = true
			}
		}

		// Статус (Исполнитель, Руководитель, Админ)
		if updateDTO.StatusID != nil && *updateDTO.StatusID != orderToUpdate.StatusID && (isExecutor || isDepartmentHead || isGlobalAdmin) {
			newStatus, err := s.statusRepo.FindStatus(ctx, *updateDTO.StatusID)
			if err != nil {
				return err
			}

			historyComment := ""

			if newStatus.Code == "CLOSED" {
				if !(isCreator || isGlobalAdmin) {
					return apperrors.NewHttpError(http.StatusForbidden, "Только создатель или администратор может закрыть заявку.", nil)
				}
				historyComment = fmt.Sprintf("Статус изменен на: «%s» (Заявка закрыта).", newStatus.Name)
			} else {
				historyComment = fmt.Sprintf("Статус изменен на: «%s».", newStatus.Name)
			}

			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "STATUS_CHANGE", NewValue: &newStatus.Name, Comment: &historyComment,
			}, nil)

			orderToUpdate.StatusID = *updateDTO.StatusID
			hasChanges = true
		}

		// Приоритет (Руководитель или Админ)
		if updateDTO.PriorityID != nil && *updateDTO.PriorityID != orderToUpdate.PriorityID && (isDepartmentHead || isGlobalAdmin) {
			priority, _ := s.priorityRepo.FindPriority(ctx, *updateDTO.PriorityID)
			historyComment := fmt.Sprintf("Приоритет изменен на: %s", priority.Name)
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "PRIORITY_CHANGE", NewValue: &priority.Name, Comment: &historyComment,
			}, nil)
			orderToUpdate.PriorityID = *updateDTO.PriorityID
			hasChanges = true
		}

		// Срок (Руководитель или Админ)
		if updateDTO.Duration != nil && (isDepartmentHead || isGlobalAdmin) {
			shouldUpdateDuration := false
			if orderToUpdate.Duration == nil {
				shouldUpdateDuration = true
			} else if *updateDTO.Duration != *orderToUpdate.Duration { // Правильное сравнение *string с *string
				shouldUpdateDuration = true
			}

			if shouldUpdateDuration {
				historyComment := fmt.Sprintf("Срок выполнения изменен на: %s", *updateDTO.Duration)
				s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
					OrderID: orderID, UserID: actor.ID, EventType: "DURATION_CHANGE", NewValue: updateDTO.Duration, Comment: &historyComment,
				}, nil)
				orderToUpdate.Duration = updateDTO.Duration
				hasChanges = true
			}
		}

		// Комментарий (Любой, кто имеет доступ к обновлению)
		if updateDTO.Comment != nil && *updateDTO.Comment != "" {
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "COMMENT", Comment: updateDTO.Comment,
			}, nil)
		}

		// Прикрепление файла (Любой, кто имеет доступ к обновлению)
		if file != nil {
			if err = s.attachFileToOrderInTx(ctx, tx, file, orderID, actor.ID); err != nil {
				return err
			}
		}

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

	finalOrder, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		s.logger.Error("Не удалось найти заявку после обновления", zap.Uint64("orderID", orderID), zap.Error(err))
		return nil, err
	}
	return s.buildOrderResponse(ctx, finalOrder)
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

	if !(permissionsMap["superuser"] || permissionsMap["scope:all"]) {
		if permissionsMap["scope:department"] {
			filter.Filter["department_id"] = actor.DepartmentID
		} else if permissionsMap["scope:own"] {
			filter.Filter["actor_id_for_own_scope"] = actor.ID
		} else {
			return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
		}
	}

	orders, totalCount, err := s.orderRepo.GetOrders(ctx, filter, actor)
	if err != nil {
		return nil, err
	}

	if len(orders) == 0 {
		return &dto.OrderListResponseDTO{List: []dto.OrderResponseDTO{}, TotalCount: 0}, nil
	}

	dtos := make([]dto.OrderResponseDTO, 0, len(orders))
	for _, order := range orders {
		orderResponse, err := s.buildOrderResponse(ctx, &order)
		if err != nil {
			s.logger.Error("Ошибка сборки DTO для заявки", zap.Uint64("orderID", order.ID), zap.Error(err))
			continue
		}
		dtos = append(dtos, *orderResponse)
	}

	return &dto.OrderListResponseDTO{
		List:       dtos,
		TotalCount: totalCount,
	}, nil
}

func (s *OrderService) FindOrderByID(ctx context.Context, orderID uint64) (*dto.OrderResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo("orders:view", *authContext) {
		s.logger.Warn("Отказано в доступе при просмотре заявки", zap.Uint64("orderID", orderID), zap.Uint64("actorID", authContext.Actor.ID))
		return nil, apperrors.ErrForbidden
	}

	return s.buildOrderResponse(ctx, authContext.Target.(*entities.Order))
}

func (s *OrderService) CreateOrder(ctx context.Context, data string, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, 0)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo("orders:create", *authContext) {
		return nil, apperrors.ErrForbidden
	}

	creatorID := authContext.Actor.ID
	var createDTO dto.CreateOrderDTO
	if err = json.Unmarshal([]byte(data), &createDTO); err != nil {
		return nil, apperrors.ErrBadRequest
	}

	var finalOrderID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		status, err := s.statusRepo.FindByCode(ctx, "OPEN")
		if err != nil {
			return fmt.Errorf("не найден статус 'OPEN': %w", err)
		}
		priority, err := s.priorityRepo.FindByCode(ctx, "MEDIUM")
		if err != nil {
			return fmt.Errorf("не найден приоритет 'MEDIUM': %w", err)
		}
		executor, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, createDTO.DepartmentID)
		if err != nil {
			return err
		}

		orderEntity := &entities.Order{
			Name: createDTO.Name, Address: createDTO.Address, DepartmentID: createDTO.DepartmentID,
			OtdelID: createDTO.OtdelID, BranchID: createDTO.BranchID, OfficeID: createDTO.OfficeID,
			EquipmentID: createDTO.EquipmentID,
			StatusID:    uint64(status.ID), PriorityID: uint64(priority.ID),
			CreatorID: creatorID, ExecutorID: executor.ID,
		}

		orderID, err := s.orderRepo.Create(ctx, tx, orderEntity)
		if err != nil {
			return fmt.Errorf("не удалось создать заявку: %w", err)
		}

		finalOrderID = orderID

		createHistory := &entities.OrderHistory{
			OrderID: orderID, UserID: creatorID, EventType: "CREATE", Comment: &orderEntity.Name,
		}
		if err := s.historyRepo.CreateInTx(ctx, tx, createHistory, nil); err != nil {
			return err
		}

		delegationComment := fmt.Sprintf("Назначен ответственный: %s", executor.Fio)
		delegateHistory := &entities.OrderHistory{
			OrderID: orderID, UserID: creatorID, EventType: "DELEGATION",
			NewValue: &executor.Fio, Comment: &delegationComment,
		}
		if err := s.historyRepo.CreateInTx(ctx, tx, delegateHistory, nil); err != nil {
			return err
		}

		if createDTO.Comment != nil && *createDTO.Comment != "" {
			commentHistory := &entities.OrderHistory{
				OrderID: orderID, UserID: creatorID, EventType: "COMMENT", Comment: createDTO.Comment,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, commentHistory, nil); err != nil {
				return err
			}
		}

		if file != nil {
			if err := s.attachFileToOrderInTx(ctx, tx, file, orderID, creatorID); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("ошибка в транзакции при создании заявки: %w", err)
	}

	createdOrder, err := s.orderRepo.FindByID(ctx, finalOrderID)
	if err != nil {
		s.logger.Error("Не удалось найти заявку после создания", zap.Uint64("orderID", finalOrderID), zap.Error(err))
		return nil, err
	}

	return s.buildOrderResponse(ctx, createdOrder)
}

func (s *OrderService) DelegateOrder(ctx context.Context, orderID uint64, delegatePayload dto.DelegateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, orderID)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo("orders:delegate", *authContext) {
		s.logger.Warn("Отказано в доступе на делегирование заявки", zap.Uint64("orderID", orderID), zap.Uint64("actorID", authContext.Actor.ID))
		return nil, apperrors.ErrForbidden
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		orderToUpdate := authContext.Target.(*entities.Order)
		actorID := authContext.Actor.ID
		isSuperAdmin := authContext.Permissions["superuser"]
		hasChanges := false

		if delegatePayload.Duration != nil {
			if orderToUpdate.Duration == nil || *orderToUpdate.Duration != *delegatePayload.Duration {
				historyComment := fmt.Sprintf("Срок выполнения изменен на: %s", *delegatePayload.Duration)
				history := &entities.OrderHistory{
					OrderID: orderID, UserID: actorID, EventType: "DURATION_CHANGE",
					NewValue: delegatePayload.Duration, Comment: &historyComment,
				}
				if err := s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
					return err
				}
				orderToUpdate.Duration = delegatePayload.Duration
				hasChanges = true
			}
		}

		if delegatePayload.ExecutorID != nil {
			if orderToUpdate.ExecutorID != *delegatePayload.ExecutorID {
				newExecutor, err := s.userRepo.FindUserByID(ctx, *delegatePayload.ExecutorID)
				if err != nil {
					return apperrors.ErrUserNotFound
				}
				if newExecutor.DepartmentID != orderToUpdate.DepartmentID && !isSuperAdmin {
					return apperrors.ErrForbidden
				}

				historyComment := fmt.Sprintf("Назначен новый ответственный: %s", newExecutor.Fio)
				delegationHistory := &entities.OrderHistory{
					OrderID: orderID, UserID: actorID, EventType: "DELEGATION",
					NewValue: &newExecutor.Fio, Comment: &historyComment,
				}
				if err := s.historyRepo.CreateInTx(ctx, tx, delegationHistory, nil); err != nil {
					return err
				}
				orderToUpdate.ExecutorID = *delegatePayload.ExecutorID
				hasChanges = true
			}
		}

		if delegatePayload.PriorityID != nil && orderToUpdate.PriorityID != *delegatePayload.PriorityID {
			priority, err := s.priorityRepo.FindPriority(ctx, *delegatePayload.PriorityID)
			if err != nil {
				return err
			}
			historyComment := fmt.Sprintf("Приоритет изменен на: %s", priority.Name)
			history := &entities.OrderHistory{
				OrderID: orderID, UserID: actorID, EventType: "PRIORITY_CHANGE",
				NewValue: &priority.Name, Comment: &historyComment,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return err
			}
			orderToUpdate.PriorityID = *delegatePayload.PriorityID
			hasChanges = true
		}

		if delegatePayload.StatusID != nil && orderToUpdate.StatusID != *delegatePayload.StatusID {
			newStatus, err := s.statusRepo.FindStatus(ctx, *delegatePayload.StatusID)
			if err != nil {
				return err
			}
			statusHistory := &entities.OrderHistory{
				OrderID: orderID, UserID: actorID, EventType: "STATUS_CHANGE", NewValue: &newStatus.Name,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, statusHistory, nil); err != nil {
				return err
			}
			orderToUpdate.StatusID = *delegatePayload.StatusID
			hasChanges = true
		}

		if delegatePayload.Comment != nil && *delegatePayload.Comment != "" {
			commentHistory := &entities.OrderHistory{
				OrderID: orderID, UserID: actorID, EventType: "COMMENT", Comment: delegatePayload.Comment,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, commentHistory, nil); err != nil {
				return err
			}
		}

		if file != nil {
			if err := s.attachFileToOrderInTx(ctx, tx, file, orderID, actorID); err != nil {
				return err
			}
		}

		if hasChanges {
			if err := s.orderRepo.Update(ctx, tx, orderToUpdate); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("ошибка в транзакции при делегировании: %w", err)
	}

	updatedOrder, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		s.logger.Error("Не удалось найти заявку после делегирования", zap.Uint64("orderID", orderID), zap.Error(err))
		return nil, err
	}

	return s.buildOrderResponse(ctx, updatedOrder)
}

// internal/services/order_service.go

// ... (после всех остальных методов, либо там, где у тебя была заглушка)

func (s *OrderService) DeleteOrder(ctx context.Context, orderID uint64) error {
	// 1. Собираем контекст для авторизации
	authContext, err := s.buildAuthzContext(ctx, orderID) // Для DeleteOrder целевая заявка нужна
	if err != nil {
		// Если заявка не найдена (ErrNotFound), buildAuthzContext вернет nil, apperrors.ErrNotFound
		// и эта ошибка будет корректно обработана ниже.
		return err
	}

	// 2. Проверяем права: `orders:delete` или `superuser`.
	if !authz.CanDo("orders:delete", *authContext) {
		s.logger.Warn("Отказано в доступе на удаление заявки",
			zap.Uint64("orderID", orderID),
			zap.Uint64("actorID", authContext.Actor.ID),
		)
		return apperrors.ErrForbidden
	}

	s.logger.Info("Заявка помечена на удаление Администратором",
		zap.Uint64("orderID", orderID),
		zap.Uint64("adminID", authContext.Actor.ID),
	)
	return s.orderRepo.DeleteOrder(ctx, orderID)
}

// Приватный хелпер для сборки контекста авторизации
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

	var targetOrder *entities.Order
	if orderID > 0 {
		targetOrder, err = s.orderRepo.FindByID(ctx, orderID)
		if err != nil {
			return nil, err
		}
	}

	return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: targetOrder}, nil
}

// Приватный хелпер для сборки DTO
func (s *OrderService) buildOrderResponse(ctx context.Context, order *entities.Order) (*dto.OrderResponseDTO, error) {
	if order == nil {
		return nil, apperrors.ErrNotFound
	}

	creator, _ := s.userRepo.FindUserByID(ctx, uint64(order.CreatorID))
	var executor *entities.User
	if order.ExecutorID != 0 {
		executor, _ = s.userRepo.FindUserByID(ctx, order.ExecutorID)
	}
	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, order.ID, 50, 0) // Увеличим лимит вложений для детального просмотра

	creatorDTO := dto.ShortUserDTO{ID: order.CreatorID}
	if creator != nil {
		creatorDTO.Fio = creator.Fio
	}

	executorDTO := dto.ShortUserDTO{}
	if executor != nil {
		executorDTO.ID = executor.ID
		executorDTO.Fio = executor.Fio
	}

	var attachmentsDTO []dto.AttachmentResponseDTO
	for _, att := range attachments {
		attachmentsDTO = append(attachmentsDTO, dto.AttachmentResponseDTO{
			ID: att.ID, FileName: att.FileName, FileSize: att.FileSize, FileType: att.FileType, URL: att.FilePath,
		})
	}

	return &dto.OrderResponseDTO{
		ID: order.ID, Name: order.Name, Address: order.Address,
		Creator: creatorDTO, Executor: executorDTO,
		DepartmentID: order.DepartmentID, StatusID: order.StatusID, PriorityID: order.PriorityID,
		Attachments: attachmentsDTO, Duration: order.Duration,
		CreatedAt: order.CreatedAt.Format(time.RFC3339), UpdatedAt: order.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// Приватный хелпер для прикрепления файла, чтобы не дублировать код.
func (s *OrderService) attachFileToOrderInTx(ctx context.Context, tx pgx.Tx, file *multipart.FileHeader, orderID, userID uint64) error {
	src, err := file.Open()
	if err != nil {
		s.logger.Error("Не удалось открыть файл для сохранения", zap.Error(err), zap.Uint64("orderID", orderID))
		return apperrors.ErrInternalServer
	}
	defer src.Close()

	const uploadContext = "order_document"
	if err := utils.ValidateFile(file, src, uploadContext); err != nil {
		return fmt.Errorf("файл не прошел валидацию: %w", err)
	}

	rules, _ := config.UploadContexts[uploadContext]
	relativePath, err := s.fileStorage.Save(src, file.Filename, rules.PathPrefix)
	if err != nil {
		return fmt.Errorf("не удалось сохранить файл: %w", err)
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
		return fmt.Errorf("не удалось создать вложение: %w", err)
	}

	attachHistory := &entities.OrderHistory{
		OrderID:   orderID,
		UserID:    userID,
		EventType: "ATTACHMENT_ADDED",
		Comment:   &file.Filename,
	}
	return s.historyRepo.CreateInTx(ctx, tx, attachHistory, &attachmentID)
}

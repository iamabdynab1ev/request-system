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

// ... (интерфейс, структура, конструктор - без изменений)
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

// ... (GetOrders, FindOrderByID, CreateOrder - без изменений)
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

	dtos := make([]dto.OrderResponseDTO, 0, len(orders))
	for _, order := range orders {
		orderResponse, err := s.buildOrderResponse(ctx, &order)
		if err != nil {
			continue
		}
		dtos = append(dtos, *orderResponse)
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
	return s.buildOrderResponse(ctx, authContext.Target.(*entities.Order))
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
		return nil, apperrors.ErrBadRequest
	}

	var finalOrderID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		status, _ := s.statusRepo.FindByCode(ctx, "OPEN")
		priority, _ := s.priorityRepo.FindByCode(ctx, "MEDIUM")
		executor, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, createDTO.DepartmentID)
		if err != nil {
			return err
		}

		orderEntity := &entities.Order{
			Name: createDTO.Name, Address: createDTO.Address, DepartmentID: createDTO.DepartmentID,
			OtdelID: createDTO.OtdelID, BranchID: createDTO.BranchID, OfficeID: createDTO.OfficeID, EquipmentID: createDTO.EquipmentID,
			StatusID: uint64(status.ID), PriorityID: uint64(priority.ID), CreatorID: creatorID, ExecutorID: executor.ID,
		}

		orderID, err := s.orderRepo.Create(ctx, tx, orderEntity)
		if err != nil {
			return fmt.Errorf("не удалось создать заявку: %w", err)
		}
		finalOrderID = orderID
		historyCreate := &entities.OrderHistory{
			OrderID: orderID, UserID: creatorID, EventType: "CREATE", Comment: &orderEntity.Name,
		}
		if err := s.historyRepo.CreateInTx(ctx, tx, historyCreate, nil); err != nil {
			return fmt.Errorf("не удалось создать запись в истории (создание): %w", err)
		}

		delegationComment := fmt.Sprintf("Назначен ответственный: %s", executor.Fio)
		historyDelegate := &entities.OrderHistory{
			OrderID: orderID, UserID: creatorID, EventType: "DELEGATION", NewValue: &executor.Fio, Comment: &delegationComment,
		}
		if err := s.historyRepo.CreateInTx(ctx, tx, historyDelegate, nil); err != nil {
			return fmt.Errorf("не удалось создать запись в истории (делегирование): %w", err)
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
	createdOrder, err := s.orderRepo.FindByID(ctx, finalOrderID)
	if err != nil {
		return nil, err
	}
	return s.buildOrderResponse(ctx, createdOrder)
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

	currentStatus, err := s.statusRepo.FindStatus(ctx, orderToUpdate.StatusID)
	if err != nil {
		return nil, err
	}
	if currentStatus.Code == "CLOSED" {
		return nil, apperrors.NewHttpError(
			http.StatusBadRequest,
			"Невозможно изменить закрытую заявку.",
			nil,
			nil,
		)
	}

	isCreator := actor.ID == orderToUpdate.CreatorID
	isExecutor := actor.ID == orderToUpdate.ExecutorID
	isDepartmentHead := actor.DepartmentID == orderToUpdate.DepartmentID && (actor.RoleName == "User")
	isGlobalAdmin := authContext.Permissions[authz.ScopeAll] || authContext.Permissions[authz.Superuser]

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		hasChanges := false

		// 1. Смена Департамента (только админ)
		if updateDTO.DepartmentID != nil && isGlobalAdmin && *updateDTO.DepartmentID != orderToUpdate.DepartmentID {
			commentDeptChange := "Заявка переведена в другой департамент."
			historyDeptChange := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "DEPARTMENT_CHANGE", Comment: &commentDeptChange,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, historyDeptChange, nil); err != nil {
				return fmt.Errorf("ошибка истории (смена департамента): %w", err)
			}
			newDepartmentID := *updateDTO.DepartmentID
			newExecutor, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, newDepartmentID)
			if err != nil {
				return apperrors.NewHttpError(http.StatusNotFound, "В целевом департаменте не найден руководитель.", err, nil)
			}
			orderToUpdate.DepartmentID = newDepartmentID
			orderToUpdate.ExecutorID = newExecutor.ID
			hasChanges = true
			commentNewExecutor := fmt.Sprintf("Назначен новый ответственный: %s", newExecutor.Fio)
			historyNewExecutor := &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION", NewValue: &newExecutor.Fio, Comment: &commentNewExecutor,
			}
			if err = s.historyRepo.CreateInTx(ctx, tx, historyNewExecutor, nil); err != nil {
				return fmt.Errorf("ошибка истории (авто-делегирование): %w", err)
			}
		}

		// 2. Смена Исполнителя
		if updateDTO.ExecutorID != nil && (isDepartmentHead || isGlobalAdmin) && *updateDTO.ExecutorID != orderToUpdate.ExecutorID {
			newExec, _ := s.userRepo.FindUserByID(ctx, *updateDTO.ExecutorID)
			comment := fmt.Sprintf("Назначен новый ответственный: %s", newExec.Fio)
			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION", NewValue: &newExec.Fio, Comment: &comment}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (делегирование): %w", err)
			}
			orderToUpdate.ExecutorID = *updateDTO.ExecutorID
			hasChanges = true
		}

		// 3. Изменение Названия и Адреса (только создатель открытой заявки)
		if currentStatus.Code == "OPEN" && isCreator {
			if updateDTO.Name != nil && *updateDTO.Name != orderToUpdate.Name {
				comment := "Название заявки изменено."
				history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "NAME_CHANGE", NewValue: updateDTO.Name, Comment: &comment}
				if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
					return fmt.Errorf("ошибка истории (смена названия): %w", err)
				}
				orderToUpdate.Name = *updateDTO.Name
				hasChanges = true
			}
			if updateDTO.Address != nil && *updateDTO.Address != orderToUpdate.Address {
				comment := "Адрес заявки изменен."
				history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "ADDRESS_CHANGE", NewValue: updateDTO.Address, Comment: &comment}
				if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
					return fmt.Errorf("ошибка истории (смена адреса): %w", err)
				}
				orderToUpdate.Address = *updateDTO.Address
				hasChanges = true
			}
		}

		// 4. Смена Статуса
		if updateDTO.StatusID != nil && *updateDTO.StatusID != orderToUpdate.StatusID && (isExecutor || isDepartmentHead || isGlobalAdmin) {
			newStatus, _ := s.statusRepo.FindStatus(ctx, *updateDTO.StatusID)
			comment := fmt.Sprintf("Статус изменен на: «%s»", newStatus.Name)
			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "STATUS_CHANGE", NewValue: &newStatus.Name, Comment: &comment}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (смена статуса): %w", err)
			}
			orderToUpdate.StatusID = *updateDTO.StatusID
			hasChanges = true
		}

		// 5. Смена Приоритета
		if updateDTO.PriorityID != nil && *updateDTO.PriorityID != orderToUpdate.PriorityID && (isDepartmentHead || isGlobalAdmin) {
			priority, _ := s.priorityRepo.FindPriority(ctx, *updateDTO.PriorityID)
			comment := fmt.Sprintf("Приоритет изменен на: %s", priority.Name)
			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "PRIORITY_CHANGE", NewValue: &priority.Name, Comment: &comment}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (смена приоритета): %w", err)
			}
			orderToUpdate.PriorityID = *updateDTO.PriorityID
			hasChanges = true
		}

		// 6. Смена Длительности
		if updateDTO.Duration != nil && (isDepartmentHead || isGlobalAdmin) {
			// 1. ПАРСИМ СТРОКУ С ДАТОЙ, которая пришла в поле "duration"
			parsedTime, err := time.Parse(time.RFC3339, *updateDTO.Duration)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат даты в поле duration", err, nil)
			}

			// 2. Форматируем дату для красивой записи в историю
			timeForHistory := parsedTime.Format("02.01.2006 15:04")
			comment := fmt.Sprintf("Срок выполнения изменен на: %s", timeForHistory)

			// 3. Создаем запись в истории (обрати внимание, в NewValue мы тоже кладем красивую строку)
			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DURATION_CHANGE", NewValue: &timeForHistory, Comment: &comment}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (смена срока): %w", err)
			}

			// 4. В сущность `orderToUpdate` мы кладем уже объект time.Time, а не строку
			orderToUpdate.Duration = &parsedTime
			hasChanges = true
		}
		// 7. Добавление Комментария
		if updateDTO.Comment != nil && *updateDTO.Comment != "" {
			history := &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "COMMENT", Comment: updateDTO.Comment}
			if err = s.historyRepo.CreateInTx(ctx, tx, history, nil); err != nil {
				return fmt.Errorf("ошибка истории (добавление комментария): %w", err)
			}
		}

		// 8. Прикрепление файла
		if file != nil {
			if err = s.attachFileToOrderInTx(ctx, tx, file, orderID, actor.ID); err != nil {
				return err
			}
		}

		// 9. Сохранение изменений
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

	finalOrder, _ := s.orderRepo.FindByID(ctx, orderID)
	return s.buildOrderResponse(ctx, finalOrder)
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
		targetOrder, err = s.orderRepo.FindByID(ctx, orderID)
		if err != nil {
			return nil, err
		}
	}

	return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: targetOrder}, nil
}

func (s *OrderService) buildOrderResponse(ctx context.Context, order *entities.Order) (*dto.OrderResponseDTO, error) {
	if order == nil {
		return nil, apperrors.ErrNotFound
	}

	creator, _ := s.userRepo.FindUserByID(ctx, order.CreatorID)
	executor, _ := s.userRepo.FindUserByID(ctx, order.ExecutorID)
	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, order.ID, 50, 0)

	creatorDTO := dto.ShortUserDTO{ID: order.CreatorID}
	if creator != nil {
		creatorDTO.Fio = creator.Fio
	}

	executorDTO := dto.ShortUserDTO{ID: order.ExecutorID}
	if executor != nil {
		executorDTO.Fio = executor.Fio
	}

	var attachmentsDTO []dto.AttachmentResponseDTO
	for _, att := range attachments {
		attachmentsDTO = append(attachmentsDTO, dto.AttachmentResponseDTO{
			ID: att.ID, FileName: att.FileName, FileSize: att.FileSize, FileType: att.FileType, URL: att.FilePath,
		})
	}

	var durationStr *string
	if order.Duration != nil {
		formatted := order.Duration.Format(time.RFC3339)
		durationStr = &formatted
	}

	return &dto.OrderResponseDTO{
		ID: order.ID, Name: order.Name, Address: order.Address,
		Creator: creatorDTO, Executor: executorDTO, DepartmentID: order.DepartmentID,
		StatusID: order.StatusID, PriorityID: order.PriorityID, Attachments: attachmentsDTO,

		Duration: durationStr,

		CreatedAt: order.CreatedAt.Format(time.RFC3339),
		UpdatedAt: order.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *OrderService) attachFileToOrderInTx(ctx context.Context, tx pgx.Tx, file *multipart.FileHeader, orderID, userID uint64) error {
	src, err := file.Open()
	if err != nil {
		s.logger.Error("Не удалось открыть файл", zap.Error(err), zap.Uint64("orderID", orderID))
		return apperrors.ErrInternalServer
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
		OrderID: orderID, UserID: userID, FileName: file.Filename, FilePath: fullFilePath,
		FileType: file.Header.Get("Content-Type"), FileSize: file.Size,
	}
	attachmentID, err := s.attachRepo.Create(ctx, tx, attach)
	if err != nil {
		return fmt.Errorf("не удалось создать вложение: %w", err)
	}
	attachHistory := &entities.OrderHistory{
		OrderID: orderID, UserID: userID, EventType: "ATTACHMENT_ADDED", Comment: &file.Filename,
	}
	return s.historyRepo.CreateInTx(ctx, tx, attachHistory, &attachmentID)
}

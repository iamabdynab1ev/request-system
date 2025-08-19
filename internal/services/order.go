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
			securityFilter = fmt.Sprintf("department_id = $%d", 1)
			securityArgs = append(securityArgs, actor.DepartmentID)
		} else if permissionsMap[authz.ScopeOwn] {
			securityFilter = fmt.Sprintf("(user_id = $%d OR executor_id = $%d)", 1, 2)
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

		s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{OrderID: orderID, UserID: creatorID, EventType: "CREATE", Comment: &orderEntity.Name}, nil)
		delegationComment := fmt.Sprintf("Назначен ответственный: %s", executor.Fio)
		s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{OrderID: orderID, UserID: creatorID, EventType: "DELEGATION", NewValue: &executor.Fio, Comment: &delegationComment}, nil)
		if createDTO.Comment != nil && *createDTO.Comment != "" {
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{OrderID: orderID, UserID: creatorID, EventType: "COMMENT", Comment: createDTO.Comment}, nil)
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

// internal/services/order_service.go

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
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Невозможно изменить закрытую заявку.", nil)
	}

	isCreator := actor.ID == orderToUpdate.CreatorID
	isExecutor := actor.ID == orderToUpdate.ExecutorID
	isDepartmentHead := actor.DepartmentID == orderToUpdate.DepartmentID && (actor.RoleName == "User")
	isGlobalAdmin := authContext.Permissions[authz.ScopeAll]

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		hasChanges := false

		if updateDTO.DepartmentID != nil && isGlobalAdmin && *updateDTO.DepartmentID != orderToUpdate.DepartmentID {
			oldDeptID, oldExecID := orderToUpdate.DepartmentID, orderToUpdate.ExecutorID
			oldExecFio := "неизвестен"
			if oldExec, err := s.userRepo.FindUserByID(ctx, oldExecID); err == nil && oldExec != nil {
				oldExecFio = oldExec.Fio
			}

			newDeptID := *updateDTO.DepartmentID
			newHead, err := s.userRepo.FindHeadByDepartmentInTx(ctx, tx, newDeptID)
			if err != nil {
				return err
			}

			orderToUpdate.DepartmentID, orderToUpdate.ExecutorID = newDeptID, newHead.ID
			hasChanges = true

			deptComment := fmt.Sprintf("Заявка переведена в департамент #%d (была в #%d).", newDeptID, oldDeptID)
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DEPARTMENT_CHANGE", Comment: &deptComment}, nil)

			if oldExecID != newHead.ID {
				execComment := fmt.Sprintf("Исполнитель изменен: %s (был: %s).", newHead.Fio, oldExecFio)
				s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION", NewValue: &newHead.Fio, Comment: &execComment}, nil)
			}
		}

		if updateDTO.ExecutorID != nil && (isDepartmentHead || isGlobalAdmin) && *updateDTO.ExecutorID != orderToUpdate.ExecutorID {
			newExec, _ := s.userRepo.FindUserByID(ctx, *updateDTO.ExecutorID)
			execComment := fmt.Sprintf("Назначен новый ответственный: %s", newExec.Fio)
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{OrderID: orderID, UserID: actor.ID, EventType: "DELEGATION", NewValue: &newExec.Fio, Comment: &execComment}, nil)
			orderToUpdate.ExecutorID, hasChanges = *updateDTO.ExecutorID, true
		}

		if currentStatus.Code == "OPEN" && isCreator {
			if updateDTO.Name != nil && *updateDTO.Name != orderToUpdate.Name {
				historyComment := fmt.Sprintf("Название заявки изменено на: «%s»", *updateDTO.Name)
				s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
					OrderID: orderID, UserID: actor.ID, EventType: "NAME_CHANGE",
					NewValue: updateDTO.Name, Comment: &historyComment,
				}, nil)
				orderToUpdate.Name, hasChanges = *updateDTO.Name, true
			}
			if updateDTO.Address != nil && *updateDTO.Address != orderToUpdate.Address {
				historyComment := fmt.Sprintf("Адрес заявки изменен на: «%s»", *updateDTO.Address)
				s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
					OrderID: orderID, UserID: actor.ID, EventType: "ADDRESS_CHANGE",
					NewValue: updateDTO.Address, Comment: &historyComment,
				}, nil)
				orderToUpdate.Address, hasChanges = *updateDTO.Address, true
			}
		}

		if updateDTO.StatusID != nil && *updateDTO.StatusID != orderToUpdate.StatusID && (isExecutor || isDepartmentHead || isGlobalAdmin) {
			newStatus, _ := s.statusRepo.FindStatus(ctx, *updateDTO.StatusID)
			if newStatus.Code == "CLOSED" && !(isCreator || isGlobalAdmin) {
				return apperrors.NewHttpError(http.StatusForbidden, "Только создатель или администратор может закрыть заявку.", nil)
			}

			historyComment := fmt.Sprintf("Статус изменен на: «%s»", newStatus.Name)
			if newStatus.Code == "CLOSED" {
				historyComment = fmt.Sprintf("Статус изменен на: «%s» (Заявка закрыта).", newStatus.Name)
			}
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "STATUS_CHANGE",
				NewValue: &newStatus.Name, Comment: &historyComment,
			}, nil)
			orderToUpdate.StatusID, hasChanges = *updateDTO.StatusID, true
		}

		if updateDTO.PriorityID != nil && *updateDTO.PriorityID != orderToUpdate.PriorityID && (isDepartmentHead || isGlobalAdmin) {
			priority, _ := s.priorityRepo.FindPriority(ctx, *updateDTO.PriorityID)
			historyComment := fmt.Sprintf("Приоритет изменен на: %s", priority.Name)
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "PRIORITY_CHANGE",
				NewValue: &priority.Name, Comment: &historyComment,
			}, nil)
			orderToUpdate.PriorityID, hasChanges = *updateDTO.PriorityID, true
		}

		if updateDTO.Duration != nil && (isDepartmentHead || isGlobalAdmin) {
			if orderToUpdate.Duration == nil || *updateDTO.Duration != *orderToUpdate.Duration {
				historyComment := fmt.Sprintf("Срок выполнения изменен на: %s", *updateDTO.Duration)
				s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
					OrderID: orderID, UserID: actor.ID, EventType: "DURATION_CHANGE",
					NewValue: updateDTO.Duration, Comment: &historyComment,
				}, nil)
				orderToUpdate.Duration, hasChanges = updateDTO.Duration, true
			}
		}

		if updateDTO.Comment != nil && *updateDTO.Comment != "" {
			s.historyRepo.CreateInTx(ctx, tx, &entities.OrderHistory{
				OrderID: orderID, UserID: actor.ID, EventType: "COMMENT", Comment: updateDTO.Comment,
			}, nil)
		}

		if file != nil {
			if err := s.attachFileToOrderInTx(ctx, tx, file, orderID, actor.ID); err != nil {
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

	return &dto.OrderResponseDTO{
		ID: order.ID, Name: order.Name, Address: order.Address,
		Creator: creatorDTO, Executor: executorDTO, DepartmentID: order.DepartmentID,
		StatusID: order.StatusID, PriorityID: order.PriorityID, Attachments: attachmentsDTO,
		Duration: order.Duration, CreatedAt: order.CreatedAt.Format(time.RFC3339),
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

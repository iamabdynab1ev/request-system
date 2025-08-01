package services

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
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
	GetOrders(ctx context.Context, filter types.Filter, actorID uint64) ([]dto.OrderResponseDTO, uint64, error)
	CreateOrder(ctx context.Context, data string, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
	DelegateOrder(ctx context.Context, orderID uint64, delegatePayload dto.DelegateOrderDTO, file *multipart.FileHeader) (*dto.OrderResponseDTO, error)
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

func (s *OrderService) GetOrders(ctx context.Context, filter types.Filter, actorID uint64) ([]dto.OrderResponseDTO, uint64, error) {
	
	actor, err := s.userRepo.FindUserByID(ctx, actorID)
	if err != nil {
		s.logger.Error("GetOrders: не удалось найти пользователя по actorID", zap.Uint64("actorID", actorID), zap.Error(err))
		return nil, 0, apperrors.ErrUserNotFound
	}
	orders, totalCount, err := s.orderRepo.GetOrders(ctx, filter, actor)
	if err != nil {
		return nil, 0, err
	}

	if len(orders) == 0 {
		return []dto.OrderResponseDTO{}, totalCount, nil
	}

	dtos := make([]dto.OrderResponseDTO, 0, len(orders))
	for _, order := range orders {
		orderResponse, err := s.buildOrderResponse(ctx, order.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("ошибка при построении DTO для заявки %d: %w", order.ID, err)
		}
		dtos = append(dtos, *orderResponse)
	}

	return dtos, totalCount, nil
}

func (s *OrderService) CreateOrder(ctx context.Context, data string, file *multipart.FileHeader) (*dto.OrderResponseDTO, error) {
	creatorID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

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
			Name:         createDTO.Name,
			Address:      createDTO.Address,
			DepartmentID: createDTO.DepartmentID,
			StatusID:     uint64(status.ID),
			PriorityID:   uint64(priority.ID),
			CreatorID:    uint64(creatorID),
			ExecutorID:   executor.ID,
		}

		orderID, err := s.orderRepo.Create(ctx, tx, orderEntity)
		if err != nil {
			return fmt.Errorf("не удалось создать заявку: %w", err)
		}
		finalOrderID = orderID
		createHistory := &entities.OrderHistory{
			OrderID:   orderID,
			UserID:    uint64(creatorID),
			EventType: "CREATE",
			Comment:   &orderEntity.Name,
		}
		if err := s.historyRepo.CreateInTx(ctx, tx, createHistory, nil); err != nil {
			return err
		}
		delegationComment := fmt.Sprintf("Назначен ответственный: %s", executor.Fio)
		delegateHistory := &entities.OrderHistory{
			OrderID:   orderID,
			UserID:    uint64(creatorID),
			EventType: "DELEGATION",
			NewValue:  &executor.Fio,
			Comment:   &delegationComment,
		}
		if err := s.historyRepo.CreateInTx(ctx, tx, delegateHistory, nil); err != nil {
			return err
		}

		if createDTO.Comment != nil && *createDTO.Comment != "" {
			commentHistory := &entities.OrderHistory{
				OrderID:   orderID,
				UserID:    uint64(creatorID),
				EventType: "COMMENT",
				Comment:   createDTO.Comment,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, commentHistory, nil); err != nil {
				return err
			}
		}
		if file != nil {
			filePath, err := s.fileStorage.Save(file)
			if err != nil {
				return fmt.Errorf("не удалось сохранить файл: %w", err)
			}
			attach := &entities.Attachment{
				OrderID:  orderID,
				UserID:   uint64(creatorID),
				FileName: file.Filename,
				FilePath: filePath,
				FileType: file.Header.Get("Content-Type"),
				FileSize: file.Size,
			}
			_, err = s.attachRepo.Create(ctx, tx, attach)
			if err != nil {
				return fmt.Errorf("не удалось создать вложение: %w", err)
			}
			attachHistory := &entities.OrderHistory{
				OrderID:   orderID,
				UserID:    uint64(creatorID),
				EventType: "ATTACHMENT_ADDED",
				Comment:   &file.Filename,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, attachHistory, nil); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("ошибка при создании заявки: %w", err)
	}
	return s.buildOrderResponse(ctx, finalOrderID)
}

func (s *OrderService) DelegateOrder(
	ctx context.Context,
	orderID uint64,
	delegatePayload dto.DelegateOrderDTO,
	file *multipart.FileHeader,
) (*dto.OrderResponseDTO, error) {
	actorID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		order, err := s.orderRepo.FindByID(ctx, orderID)
		if err != nil {
			return err
		}
		actor, err := s.userRepo.FindUserByID(ctx, actorID)
		if err != nil {
			s.logger.Error("Не удалось найти пользователя-актора по ID", zap.Uint64("actorID", actorID), zap.Error(err))
			return apperrors.ErrUserNotFound
		}

		const SuperAdminRoleName string = "Super Admin"
		const AdminRoleName string = "Admin"
		const UserRoleName string = "User"

		isCurrentExecutor := (order.ExecutorID == actor.ID)
		isSuperAdmin := (actor.RoleName == SuperAdminRoleName)
		isAdmin := (actor.RoleName == AdminRoleName)
		isUserInSameDepartment := (actor.RoleName == UserRoleName && actor.DepartmentID == order.DepartmentID)

		canDelegate := isCurrentExecutor || isSuperAdmin || isAdmin || isUserInSameDepartment

		if !canDelegate {
			s.logger.Warn("Отказано в доступе на делегирование заказа. Пользователь не удовлетворяет ни одному из правил.",
				zap.Uint64("orderID", order.ID),
				zap.Uint64("actorID", actor.ID),
				zap.String("actorRoleName", actor.RoleName),
				zap.Uint64("orderDepartmentID", order.DepartmentID),
				zap.Uint64("actorDepartmentID", actor.DepartmentID),
				zap.Bool("isCurrentExecutor", isCurrentExecutor),
				zap.Bool("isSuperAdmin", isSuperAdmin),
				zap.Bool("isAdmin", isAdmin),
				zap.Bool("isUserInSameDepartment", isUserInSameDepartment),
			)
			return apperrors.ErrForbidden
		}

		hasChanges := false

		if delegatePayload.ExecutorID != nil {
			newID := *delegatePayload.ExecutorID
			if order.ExecutorID != newID {
				newExec, err := s.userRepo.FindUserByID(ctx, newID)
				if err != nil {
					return apperrors.ErrUserNotFound
				}
				if newExec.DepartmentID != order.DepartmentID && !isSuperAdmin && !isAdmin {
					return apperrors.ErrForbidden
				}
				delegationHistory := &entities.OrderHistory{
					OrderID:   orderID,
					UserID:    actorID,
					EventType: "DELEGATION",
					NewValue:  &newExec.Fio,
				}
				if err := s.historyRepo.CreateInTx(ctx, tx, delegationHistory, nil); err != nil {
					return err
				}
				order.ExecutorID = newID
				hasChanges = true
			}
		}

		if delegatePayload.StatusID != nil && *delegatePayload.StatusID != order.StatusID {
			newStatus, err := s.statusRepo.FindStatus(ctx, *delegatePayload.StatusID)
			if err != nil {
				return err
			}
			statusHistory := &entities.OrderHistory{
				OrderID:   orderID,
				UserID:    actorID,
				EventType: "STATUS_CHANGE",
				NewValue:  &newStatus.Name,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, statusHistory, nil); err != nil {
				return err
			}
			order.StatusID = *delegatePayload.StatusID
			hasChanges = true
		}

		if delegatePayload.Comment != nil && *delegatePayload.Comment != "" {
			commentHistory := &entities.OrderHistory{
				OrderID:   orderID,
				UserID:    actorID,
				EventType: "COMMENT",
				Comment:   delegatePayload.Comment,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, commentHistory, nil); err != nil {
				return err
			}
		}

		if hasChanges {
			if err := s.orderRepo.Update(ctx, tx, order); err != nil {
				return err
			}
		}

		if file != nil {
			filePath, err := s.fileStorage.Save(file)
			if err != nil {
				return err
			}
			attach := &entities.Attachment{
				OrderID:  orderID,
				UserID:   actorID,
				FileName: file.Filename,
				FilePath: filePath,
				FileType: file.Header.Get("Content-Type"),
				FileSize: file.Size,
			}
			attachmentID, err := s.attachRepo.Create(ctx, tx, attach)
			if err != nil {
				return err
			}
			attachHistory := &entities.OrderHistory{
				OrderID:   orderID,
				UserID:    actorID,
				EventType: "ATTACHMENT_ADDED",
				Comment:   &file.Filename,
			}
			if err := s.historyRepo.CreateInTx(ctx, tx, attachHistory, &attachmentID); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return s.buildOrderResponse(ctx, orderID)
}

func (s *OrderService) buildOrderResponse(
	ctx context.Context,
	orderID uint64,
) (*dto.OrderResponseDTO, error) {
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	creator, _ := s.userRepo.FindUserByID(ctx, uint64(order.CreatorID))
	var executor *entities.User
	if order.ExecutorID != 0 {
		executor, _ = s.userRepo.FindUserByID(ctx, order.ExecutorID)
	}
	attachments, _ := s.attachRepo.FindAllByOrderID(ctx, orderID, 5, 0)
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
			ID:       att.ID,
			FileName: att.FileName,
			FileSize: att.FileSize,
			FileType: att.FileType,
			URL:      "/static/" + att.FilePath,
		})
	}
	return &dto.OrderResponseDTO{
		ID:           order.ID,
		Name:         order.Name,
		Address:      order.Address,
		Creator:      creatorDTO,
		Executor:     executorDTO,
		DepartmentID: order.DepartmentID,
		StatusID:     order.StatusID,
		PriorityID:   order.PriorityID,
		Attachments:  attachmentsDTO,
		CreatedAt:    order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    order.UpdatedAt.Format(time.RFC3339),
	}, nil
}

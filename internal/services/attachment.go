package services

import (
	"context"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/filestorage"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

// AttachmentServiceInterface определяет контракт для управления вложениями.
type AttachmentServiceInterface interface {
	GetAttachmentsByOrderID(ctx context.Context, orderID uint64) ([]dto.AttachmentResponseDTO, error)
	DeleteAttachment(ctx context.Context, attachmentID uint64) error
}

type AttachmentService struct {
	repo        repositories.AttachmentRepositoryInterface
	orderRepo   repositories.OrderRepositoryInterface
	userRepo    repositories.UserRepositoryInterface
	historyRepo repositories.OrderHistoryRepositoryInterface
	fileStorage filestorage.FileStorageInterface
	logger      *zap.Logger
}

func NewAttachmentService(
	repo repositories.AttachmentRepositoryInterface,
	orderRepo repositories.OrderRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	historyRepo repositories.OrderHistoryRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) AttachmentServiceInterface {
	return &AttachmentService{
		repo:        repo,
		orderRepo:   orderRepo,
		userRepo:    userRepo,
		historyRepo: historyRepo,
		fileStorage: fileStorage,
		logger:      logger,
	}
}

// GetAttachmentsByOrderID получает список всех вложений для указанной заявки.
func (s *AttachmentService) GetAttachmentsByOrderID(ctx context.Context, orderID uint64) ([]dto.AttachmentResponseDTO, error) {
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	authCtx, err := s.buildOrderAuthzContext(ctx, order)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.OrdersView, *authCtx) {
		s.logger.Warn("доступ к вложениям заявки запрещен", zap.Uint64("orderID", orderID), zap.Uint64("userID", authCtx.Actor.ID))
		return nil, apperrors.ErrForbidden
	}

	attachments, err := s.repo.FindAllByOrderID(ctx, orderID, 100, 0)
	if err != nil {
		s.logger.Error("не удалось получить вложения для заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return nil, err
	}

	attachmentsDTO := make([]dto.AttachmentResponseDTO, 0, len(attachments))
	for _, attachment := range attachments {
		attachmentsDTO = append(attachmentsDTO, dto.AttachmentResponseDTO{
			ID:       attachment.ID,
			FileName: attachment.FileName,
			URL:      "/uploads/" + attachment.FilePath,
		})
	}

	return attachmentsDTO, nil
}

func (s *AttachmentService) DeleteAttachment(ctx context.Context, attachmentID uint64) error {
	targetAttachment, err := s.repo.FindByID(ctx, attachmentID)
	if err != nil {
		s.logger.Warn("попытка удаления несуществующего вложения", zap.Uint64("attachmentID", attachmentID), zap.Error(err))
		return err
	}

	order, err := s.orderRepo.FindByID(ctx, targetAttachment.OrderID)
	if err != nil {
		return err
	}

	authCtx, err := s.buildOrderAuthzContext(ctx, order)
	if err != nil {
		return err
	}
	if !s.canManageOrderAttachments(*authCtx) {
		s.logger.Warn(
			"удаление вложения запрещено",
			zap.Uint64("attachmentID", attachmentID),
			zap.Uint64("orderID", targetAttachment.OrderID),
			zap.Uint64("userID", authCtx.Actor.ID),
		)
		return apperrors.ErrForbidden
	}

	attachment := targetAttachment

	err = s.repo.DeleteAttachment(ctx, attachmentID)
	if err != nil {
		s.logger.Error("не удалось удалить запись о вложении из бд", zap.Uint64("attachmentID", attachmentID), zap.Error(err))
		return err
	}
	s.logger.Info("запись о вложении успешно удалена из бд", zap.Uint64("attachmentID", attachmentID))

	fileURL := "/uploads/" + attachment.FilePath
	err = s.fileStorage.Delete(fileURL)
	if err != nil {
		s.logger.Warn("не удалось удалить физический файл вложения",
			zap.Uint64("attachmentID", attachmentID),
			zap.String("path", fileURL),
			zap.Error(err))
	} else {
		s.logger.Info("физический файл вложения успешно удален", zap.String("path", fileURL))
	}

	return nil
}

func (s *AttachmentService) canManageOrderAttachments(ctx authz.Context) bool {
	permissions := []string{
		authz.OrdersUpdate,
		authz.OrdersUpdateInOtdelScope,
		authz.OrdersUpdateInOfficeScope,
		authz.OrdersUpdateInBranchScope,
		authz.OrdersUpdateInDepartmentScope,
	}

	for _, permission := range permissions {
		if authz.CanDo(permission, ctx) {
			return true
		}
	}

	return false
}

func (s *AttachmentService) buildOrderAuthzContext(ctx context.Context, order *entities.Order) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, apperrors.ErrUnauthorized
	}

	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	isParticipant := order.CreatorID == userID || (order.ExecutorID != nil && *order.ExecutorID == userID)
	if !isParticipant {
		if wasParticipant, err := s.historyRepo.IsUserParticipant(ctx, order.ID, userID); err == nil {
			isParticipant = wasParticipant
		}
	}

	return &authz.Context{
		Actor:         actor,
		Permissions:   permissionsMap,
		Target:        order,
		IsParticipant: isParticipant,
	}, nil
}

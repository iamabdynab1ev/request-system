// Corrected: internal/services/attachment_service.go (no changes, assumes DTO fixed)
package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/repositories"
	"request-system/pkg/filestorage"

	"go.uber.org/zap"
)

// AttachmentServiceInterface определяет контракт для управления вложениями.
type AttachmentServiceInterface interface {
	GetAttachmentsByOrderID(ctx context.Context, orderID uint64) ([]dto.AttachmentResponseDTO, error)
	DeleteAttachment(ctx context.Context, attachmentID uint64) error
}

type AttachmentService struct {
	repo        repositories.AttachmentRepositoryInterface
	fileStorage filestorage.FileStorageInterface
	logger      *zap.Logger
}

func NewAttachmentService(
	repo repositories.AttachmentRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) AttachmentServiceInterface {
	return &AttachmentService{
		repo:        repo,
		fileStorage: fileStorage,
		logger:      logger,
	}
}

// GetAttachmentsByOrderID получает список всех вложений для указанной заявки.
func (s *AttachmentService) GetAttachmentsByOrderID(ctx context.Context, orderID uint64) ([]dto.AttachmentResponseDTO, error) {
	attachments, err := s.repo.FindAllByOrderID(ctx, orderID, 100, 0)
	if err != nil {
		s.logger.Error("не удалось получить вложения для заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return nil, err
	}

	var attachmentsDTO []dto.AttachmentResponseDTO
	for _, a := range attachments {
		dto := dto.AttachmentResponseDTO{
			ID:       a.ID,
			FileName: a.FileName,
			URL:      "/uploads/" + a.FilePath,
		}
		attachmentsDTO = append(attachmentsDTO, dto)
	}

	return attachmentsDTO, nil
}

func (s *AttachmentService) DeleteAttachment(ctx context.Context, attachmentID uint64) error {
	attachment, err := s.repo.FindByID(ctx, attachmentID)
	if err != nil {
		s.logger.Warn("попытка удаления несуществующего вложения", zap.Uint64("attachmentID", attachmentID), zap.Error(err))
		return err
	}

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

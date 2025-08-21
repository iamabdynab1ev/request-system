// Файл: internal/services/attachment_service.go
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
			FileSize: a.FileSize,
			FileType: a.FileType,
			// Используем единообразный префикс /uploads/
			URL: "/uploads/" + a.FilePath,
		}
		attachmentsDTO = append(attachmentsDTO, dto)
	}

	return attachmentsDTO, nil
}

// DeleteAttachment удаляет вложение как из базы данных, так и с физического носителя.
// ИСПРАВЛЕННАЯ ВЕРСИЯ
func (s *AttachmentService) DeleteAttachment(ctx context.Context, attachmentID uint64) error {
	// 1. Находим вложение в БД, чтобы получить путь к файлу.
	//    Предполагаем, что у вас в репозитории есть метод FindByID.
	attachment, err := s.repo.FindByID(ctx, attachmentID)
	if err != nil {
		s.logger.Warn("попытка удаления несуществующего вложения", zap.Uint64("attachmentID", attachmentID), zap.Error(err))
		return err
	}

	// 2. Удаляем запись из базы данных, ИСПОЛЬЗУЯ ВАШ СУЩЕСТВУЮЩИЙ МЕТОД
	err = s.repo.DeleteAttachment(ctx, attachmentID)
	if err != nil {
		s.logger.Error("не удалось удалить запись о вложении из бд", zap.Uint64("attachmentID", attachmentID), zap.Error(err))
		return err
	}
	s.logger.Info("запись о вложении успешно удалена из бд", zap.Uint64("attachmentID", attachmentID))

	// 3. После успешного удаления из БД, удаляем физический файл.
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

	return nil // Все прошло успешно
}

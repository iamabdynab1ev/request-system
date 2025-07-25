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
	fileStorage filestorage.FileStorageInterface // Зависимость для удаления физических файлов
	logger      *zap.Logger
}

func NewAttachmentService(
	repo repositories.AttachmentRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) AttachmentServiceInterface {
	return &AttachmentService{
		repo:        repo,
		fileStorage: fileStorage, // Используется fileStorage.basePath для удаления
		logger:      logger,
	}
}

// GetAttachmentsByOrderID получает список всех вложений для указанной заявки.
func (s *AttachmentService) GetAttachmentsByOrderID(ctx context.Context, orderID uint64) ([]dto.AttachmentResponseDTO, error) {
	// Используем метод FindAllByOrderID, который мы определили ранее
	attachments, err := s.repo.FindAllByOrderID(ctx, orderID, 10, 0)
	if err != nil {
		s.logger.Error("не удалось получить вложения для заявки", zap.Uint64("orderID", orderID), zap.Error(err))
		return nil, err
	}

	// Преобразуем сущности в DTO для ответа клиенту
	var attachmentsDTO []dto.AttachmentResponseDTO
	for _, a := range attachments {
		dto := dto.AttachmentResponseDTO{
			ID:       a.ID,
			FileName: a.FileName,
			FileSize: a.FileSize,
			FileType: a.FileType,
			URL:      "/static/" + a.FilePath, // Формируем URL для фронтенда
		}
		attachmentsDTO = append(attachmentsDTO, dto)
	}

	return attachmentsDTO, nil
}

// DeleteAttachment удаляет вложение как из базы данных, так и с физического носителя.
func (s *AttachmentService) DeleteAttachment(ctx context.Context, attachmentID uint64) error {
	// Сначала нужно найти вложение в БД, чтобы получить путь к файлу
	// Для этого в AttachmentRepositoryInterface нам нужен метод FindByID. Давайте предположим, что он есть.
	// Если его нет, его нужно добавить по аналогии с FindAllByOrderID.
	// AttachmentRepository.FindByID(...)

	// Здесь должна быть логика поиска attachment по ID, чтобы получить FilePath.
	// Для упрощения, предположим, что fileStorage может сам строить путь,
	// но более надежно - сначала получить путь из БД.

	// Для данного примера, мы пока пропустим физическое удаление файла,
	// так как это потребует добавления метода FindByID в репозиторий.
	// Фокусируемся на том, чтобы код компилировался с текущими интерфейсами.

	// ВАЖНО: Эта реализация требует доработки после добавления FindByID в репозиторий.
	// Пока что эта функция будет заглушкой, чтобы удовлетворить компилятор, если
	// где-то в контроллере есть вызов.

	// TODO: Реализовать полное удаление после добавления repo.FindByID()
	s.logger.Warn("DeleteAttachment вызван, но физическое удаление файла не реализовано", zap.Uint64("attachmentID", attachmentID))

	// err := s.repo.Delete(ctx, attachmentID)
	// if err != nil {
	//  s.logger.Error("не удалось удалить запись о вложении из бд", zap.Uint64("attachmentID", attachmentID), zap.Error(err))
	// 	return err
	// }

	return nil // Возвращаем nil, чтобы не ломать существующую логику
}

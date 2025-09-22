// Файл: internal/services/priority_service.go
package services

import (
	"context"
	"strings" // Добавляем импорт для работы со строками

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

// PriorityServiceInterface - ОБНОВЛЕННЫЙ ИНТЕРФЕЙС.
// Вся информация о файлах (multipart.FileHeader) удалена.
type PriorityServiceInterface interface {
	GetPriorities(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.PriorityDTO], error)
	FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error)
	CreatePriority(ctx context.Context, createDTO dto.CreatePriorityDTO) (*dto.PriorityDTO, error)
	UpdatePriority(ctx context.Context, id uint64, updateDTO dto.UpdatePriorityDTO) (*dto.PriorityDTO, error)
	DeletePriority(ctx context.Context, id uint64) error
}

// PriorityService - ОБНОВЛЕННАЯ СТРУКТУРА.
// Убрано поле fileStorage, так как оно больше не используется.
type PriorityService struct {
	repo     repositories.PriorityRepositoryInterface
	userRepo repositories.UserRepositoryInterface
	logger   *zap.Logger
}

// NewPriorityService - ОБНОВЛЕННЫЙ КОНСТРУКТОР.
// Убран параметр fileStorage.
func NewPriorityService(
	repo repositories.PriorityRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) PriorityServiceInterface {
	return &PriorityService{repo: repo, userRepo: userRepo, logger: logger}
}

// buildAuthzContext - остается без изменений
func (s *PriorityService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
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
	return &authz.Context{Actor: actor, Permissions: permissionsMap}, nil
}

// GetPriorities и FindPriority - остаются без изменений
func (s *PriorityService) GetPriorities(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.PriorityDTO], error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.PrioritiesView, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	priorities, total, err := s.repo.GetPriorities(ctx, limit, offset, search)
	if err != nil {
		return nil, err
	}

	var currentPage uint64 = 1
	if limit > 0 {
		currentPage = (offset / limit) + 1
	}

	return &dto.PaginatedResponse[dto.PriorityDTO]{
		List:       priorities,
		Pagination: dto.PaginationObject{TotalCount: total, Page: currentPage, Limit: limit},
	}, nil
}

func (s *PriorityService) FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.PrioritiesView, *authContext) {
		return nil, apperrors.ErrForbidden
	}
	return s.repo.FindPriority(ctx, id)
}

// CreatePriority - ИСПРАВЛЕННЫЙ И УПРОЩЕННЫЙ МЕТОД
func (s *PriorityService) CreatePriority(ctx context.Context, createDTO dto.CreatePriorityDTO) (*dto.PriorityDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.PrioritiesCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	// Добавлена логика генерации 'code', если он не был предоставлен клиентом.
	if createDTO.Code == "" && createDTO.Name != "" {
		// "Высокий приоритет" -> "ВЫСОКИЙ_ПРИОРИТЕТ"
		// Для русского языка может понадобиться транслитерация, но это простой рабочий пример.
		createDTO.Code = strings.ToUpper(strings.ReplaceAll(createDTO.Name, " ", "_"))
		s.logger.Debug("Поле 'code' не было предоставлено, сгенерировано автоматически", zap.String("generated_code", createDTO.Code))
	}

	// Вся логика работы с файлами и иконками удалена.
	// Просто вызываем метод репозитория.
	return s.repo.CreatePriority(ctx, createDTO)
}

// UpdatePriority - ИСПРАВЛЕННЫЙ И УПРОЩЕННЫЙ МЕТОД
func (s *PriorityService) UpdatePriority(ctx context.Context, id uint64, updateDTO dto.UpdatePriorityDTO) (*dto.PriorityDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.PrioritiesUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	// Вся логика работы с файлами и иконками удалена.
	// Можно добавить проверку существования записи перед обновлением для большей надежности.
	if _, err := s.repo.FindPriority(ctx, id); err != nil {
		s.logger.Warn("Попытка обновить несуществующий приоритет", zap.Uint64("id", id), zap.Error(err))
		return nil, err // err здесь будет ErrNotFound
	}

	return s.repo.UpdatePriority(ctx, id, updateDTO)
}

// DeletePriority - ИСПРАВЛЕННЫЙ И УПРОЩЕННЫЙ МЕТОД
func (s *PriorityService) DeletePriority(ctx context.Context, id uint64) error {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.PrioritiesDelete, *authContext) {
		return apperrors.ErrForbidden
	}

	// Вся логика удаления файлов иконок удалена.
	return s.repo.DeletePriority(ctx, id)
}

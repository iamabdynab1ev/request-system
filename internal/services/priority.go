// Файл: internal/services/priority_service.go
package services

import (
	"context"
	"mime/multipart"
	"net/http"

	"request-system/config"
	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/filestorage"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

type PriorityServiceInterface interface {
	GetPriorities(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.PriorityDTO], error)
	FindPriority(ctx context.Context, id uint64) (*dto.PriorityDTO, error)
	CreatePriority(ctx context.Context, createDTO dto.CreatePriorityDTO, iconSmallHeader *multipart.FileHeader, iconBigHeader *multipart.FileHeader) (*dto.PriorityDTO, error)
	UpdatePriority(ctx context.Context, id uint64, updateDTO dto.UpdatePriorityDTO, iconSmallHeader *multipart.FileHeader, iconBigHeader *multipart.FileHeader) (*dto.PriorityDTO, error)
	DeletePriority(ctx context.Context, id uint64) error
}

type PriorityService struct {
	repo        repositories.PriorityRepositoryInterface
	userRepo    repositories.UserRepositoryInterface
	fileStorage filestorage.FileStorageInterface
	logger      *zap.Logger
}

func NewPriorityService(
	repo repositories.PriorityRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) PriorityServiceInterface {
	return &PriorityService{repo: repo, userRepo: userRepo, fileStorage: fileStorage, logger: logger}
}

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

func (s *PriorityService) CreatePriority(ctx context.Context, createDTO dto.CreatePriorityDTO, iconSmallHeader *multipart.FileHeader, iconBigHeader *multipart.FileHeader) (*dto.PriorityDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.PrioritiesCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	var smallIconPath, bigIconPath string
	const urlPrefix = "/uploads/"

	if iconSmallHeader != nil {
		file, err := iconSmallHeader.Open()
		if err != nil {
			s.logger.Error("Не удалось открыть small icon", zap.Error(err))
			return nil, apperrors.ErrInternalServer
		}
		defer file.Close()
		// ИЗМЕНЕНИЕ ЗДЕСЬ
		if err = utils.ValidateFile(iconSmallHeader, file, "icon_small"); err != nil {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Маленькая иконка: "+err.Error(), err)
		}
		// И ИЗМЕНЕНИЕ ЗДЕСЬ
		rules, _ := config.UploadContexts["icon_small"]
		path, err := s.fileStorage.Save(file, iconSmallHeader.Filename, rules.PathPrefix)
		if err != nil {
			s.logger.Error("Не удалось сохранить small icon", zap.Error(err))
			return nil, apperrors.ErrInternalServer
		}
		smallIconPath = urlPrefix + path
	}

	if iconBigHeader != nil {
		file, err := iconBigHeader.Open()
		if err != nil {
			s.logger.Error("Не удалось открыть big icon", zap.Error(err))
			return nil, apperrors.ErrInternalServer
		}
		defer file.Close()
		// ИЗМЕНЕНИЕ ЗДЕСЬ
		if err := utils.ValidateFile(iconBigHeader, file, "icon_big"); err != nil {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "Большая иконка: "+err.Error(), err)
		}
		// И ИЗМЕНЕНИЕ ЗДЕСЬ
		rules, _ := config.UploadContexts["icon_big"]
		path, err := s.fileStorage.Save(file, iconBigHeader.Filename, rules.PathPrefix)
		if err != nil {
			s.logger.Error("Не удалось сохранить big icon", zap.Error(err))
			return nil, apperrors.ErrInternalServer
		}
		bigIconPath = urlPrefix + path
	}

	return s.repo.CreatePriority(ctx, createDTO, smallIconPath, bigIconPath)
}

func (s *PriorityService) UpdatePriority(ctx context.Context, id uint64, updateDTO dto.UpdatePriorityDTO, iconSmallHeader, iconBigHeader *multipart.FileHeader) (*dto.PriorityDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.PrioritiesUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	currentPriority, err := s.repo.FindPriority(ctx, id)
	if err != nil {
		return nil, err
	}

	var smallIconPath, bigIconPath *string
	const urlPrefix = "/uploads/"

	if iconSmallHeader != nil {
		file, _ := iconSmallHeader.Open()
		defer file.Close()
		// ИЗМЕНЕНИЕ ЗДЕСЬ
		if err := utils.ValidateFile(iconSmallHeader, file, "icon_small"); err != nil {
			return nil, err
		}
		// И ИЗМЕНЕНИЕ ЗДЕСЬ
		rules, _ := config.UploadContexts["icon_small"]
		path, err := s.fileStorage.Save(file, iconSmallHeader.Filename, rules.PathPrefix)
		if err != nil {
			return nil, err
		}
		fullPath := urlPrefix + path
		smallIconPath = &fullPath
	}

	if iconBigHeader != nil {
		file, _ := iconBigHeader.Open()
		defer file.Close()
		// ИЗМЕНЕНИЕ ЗДЕСЬ
		if err := utils.ValidateFile(iconBigHeader, file, "icon_big"); err != nil {
			return nil, err
		}
		// И ИЗМЕНЕНИЕ ЗДЕСЬ
		rules, _ := config.UploadContexts["icon_big"]
		path, err := s.fileStorage.Save(file, iconBigHeader.Filename, rules.PathPrefix)
		if err != nil {
			return nil, err
		}
		fullPath := urlPrefix + path
		bigIconPath = &fullPath
	}

	updatedPriority, err := s.repo.UpdatePriority(ctx, id, updateDTO, smallIconPath, bigIconPath)
	if err != nil {
		if smallIconPath != nil {
			_ = s.fileStorage.Delete(*smallIconPath)
		}
		if bigIconPath != nil {
			_ = s.fileStorage.Delete(*bigIconPath)
		}
		return nil, err
	}

	if smallIconPath != nil && currentPriority.IconSmall != "" {
		_ = s.fileStorage.Delete(currentPriority.IconSmall)
	}
	if bigIconPath != nil && currentPriority.IconBig != "" {
		_ = s.fileStorage.Delete(currentPriority.IconBig)
	}

	return updatedPriority, nil
}

func (s *PriorityService) DeletePriority(ctx context.Context, id uint64) error {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.PrioritiesDelete, *authContext) {
		return apperrors.ErrForbidden
	}

	priorityToDelete, err := s.repo.FindPriority(ctx, id)
	if err != nil {
		return err
	}

	err = s.repo.DeletePriority(ctx, id)
	if err != nil {
		return err
	}

	if priorityToDelete.IconSmall != "" {
		if err := s.fileStorage.Delete(priorityToDelete.IconSmall); err != nil {
			s.logger.Warn("Не удалось удалить маленькую иконку приоритета", zap.String("path", priorityToDelete.IconSmall), zap.Error(err))
		}
	}
	if priorityToDelete.IconBig != "" {
		if err := s.fileStorage.Delete(priorityToDelete.IconBig); err != nil {
			s.logger.Warn("Не удалось удалить большую иконку приоритета", zap.String("path", priorityToDelete.IconBig), zap.Error(err))
		}
	}

	return nil
}

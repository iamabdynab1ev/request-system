// Файл: internal/services/status_service.go
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

type StatusServiceInterface interface {
	GetStatuses(ctx context.Context, limit uint64, offset uint64, search string) (*dto.PaginatedResponse[dto.StatusDTO], error)
	FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error)
	FindByCode(ctx context.Context, code string) (*dto.StatusDTO, error)
	CreateStatus(ctx context.Context, createDTO dto.CreateStatusDTO, iconSmallHeader *multipart.FileHeader, iconBigHeader *multipart.FileHeader) (*dto.StatusDTO, error)
	UpdateStatus(ctx context.Context, id uint64, updateDTO dto.UpdateStatusDTO, iconSmallHeader *multipart.FileHeader, iconBigHeader *multipart.FileHeader) (*dto.StatusDTO, error)
	DeleteStatus(ctx context.Context, id uint64) error
}

type StatusService struct {
	repo        repositories.StatusRepositoryInterface
	userRepo    repositories.UserRepositoryInterface
	fileStorage filestorage.FileStorageInterface
	logger      *zap.Logger
}

func NewStatusService(
	repo repositories.StatusRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) StatusServiceInterface {
	return &StatusService{repo: repo, userRepo: userRepo, fileStorage: fileStorage, logger: logger}
}

func (s *StatusService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
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
	return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: nil}, nil
}

func (s *StatusService) GetStatuses(ctx context.Context, limit, offset uint64, search string) (*dto.PaginatedResponse[dto.StatusDTO], error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.StatusesView, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	statuses, total, err := s.repo.GetStatuses(ctx, limit, offset, search)
	if err != nil {
		return nil, err
	}

	var currentPage uint64 = 1
	if limit > 0 {
		currentPage = (offset / limit) + 1
	}
	return &dto.PaginatedResponse[dto.StatusDTO]{
		List:       statuses,
		Pagination: dto.PaginationObject{TotalCount: total, Page: currentPage, Limit: limit},
	}, nil
}

func (s *StatusService) FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.StatusesView, *authContext) {
		return nil, apperrors.ErrForbidden
	}
	return s.repo.FindStatus(ctx, id)
}

func (s *StatusService) FindByCode(ctx context.Context, code string) (*dto.StatusDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.StatusesView, *authContext) {
		return nil, apperrors.ErrForbidden
	}
	return s.repo.FindByCode(ctx, code)
}

func (s *StatusService) CreateStatus(
	ctx context.Context,
	createDTO dto.CreateStatusDTO,
	iconSmallHeader *multipart.FileHeader,
	iconBigHeader *multipart.FileHeader,
) (*dto.StatusDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}

	if !authz.CanDo(authz.StatusesCreate, *authContext) {
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

	return s.repo.CreateStatus(ctx, createDTO, smallIconPath, bigIconPath)
}

func (s *StatusService) UpdateStatus(ctx context.Context, id uint64, updateDTO dto.UpdateStatusDTO, iconSmallHeader, iconBigHeader *multipart.FileHeader) (*dto.StatusDTO, error) {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.StatusesUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
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
	return s.repo.UpdateStatus(ctx, id, updateDTO, smallIconPath, bigIconPath)
}

func (s *StatusService) DeleteStatus(ctx context.Context, id uint64) error {
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.StatusesDelete, *authContext) {
		return apperrors.ErrForbidden
	}
	return s.repo.DeleteStatus(ctx, id)
}

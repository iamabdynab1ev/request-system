// Файл: internal/services/status_service.go
package services

import (
	"context"
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

	"go.uber.org/zap"
)

type StatusServiceInterface interface {
	GetStatuses(ctx context.Context, filter types.Filter) (*dto.PaginatedResponse[dto.StatusDTO], error)
	FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error)
	FindIDByCode(ctx context.Context, code string) (uint64, error)
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

func statusEntityToDTO(entity *entities.Status) *dto.StatusDTO {
	if entity == nil {
		return nil
	}

	var codeStr string

	if entity.Code != nil {
		codeStr = *entity.Code
	}

	return &dto.StatusDTO{
		ID:   uint64(entity.ID),
		Name: entity.Name,
		Code: codeStr,
		Type: entity.Type,
	}
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

func (s *StatusService) GetStatuses(ctx context.Context, filter types.Filter) (*dto.PaginatedResponse[dto.StatusDTO], error) {
	authCtx, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.StatusesView, *authCtx) {
		return nil, apperrors.ErrForbidden
	}

	statuses, total, err := s.repo.GetStatuses(ctx, filter)
	if err != nil {
		return nil, err
	}

	return &dto.PaginatedResponse[dto.StatusDTO]{
		List: statuses,
		Pagination: dto.PaginationObject{
			TotalCount: total,
			Page:       uint64(filter.Page),
			Limit:      uint64(filter.Limit),
		},
	}, nil
}

func (s *StatusService) FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error) {
	// <<<--- ИСПРАВЛЕНИЕ: Вызываем метод, возвращающий сущность, и конвертируем в DTO ---
	entity, err := s.repo.FindStatus(ctx, id)
	if err != nil {
		return nil, err
	}
	return statusEntityToDTO(entity), nil
}

func (s *StatusService) FindIDByCode(ctx context.Context, code string) (uint64, error) {
	return s.repo.FindIDByCode(ctx, code)
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

		if err = utils.ValidateFile(iconSmallHeader, file, "icon_small"); err != nil {
			return nil, apperrors.NewHttpError(
				http.StatusBadRequest,
				"Маленькая иконка: "+err.Error(),
				err,
				nil,
			)
		}

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

		if err := utils.ValidateFile(iconBigHeader, file, "icon_big"); err != nil {
			return nil, apperrors.NewHttpError(
				http.StatusBadRequest,
				"Большая иконка: "+err.Error(),
				err,
				nil,
			)
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

package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"request-system/config"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	"request-system/pkg/filestorage"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

type StatusService struct {
	statusRepository repositories.StatusRepositoryInterface
	fileStorage      filestorage.FileStorageInterface
	logger           *zap.Logger
}

func NewStatusService(
	statusRepository repositories.StatusRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) *StatusService {
	return &StatusService{
		statusRepository: statusRepository,
		fileStorage:      fileStorage,
		logger:           logger,
	}
}

func (s *StatusService) GetStatuses(ctx context.Context, limit uint64, offset uint64) ([]dto.StatusDTO, uint64, error) {
	return s.statusRepository.GetStatuses(ctx, limit, offset)
}

func (s *StatusService) FindStatus(ctx context.Context, id uint64) (*dto.StatusDTO, error) {
	return s.statusRepository.FindStatus(ctx, id)
}

func (s *StatusService) FindByCode(ctx context.Context, code string) (*dto.StatusDTO, error) {
	return s.statusRepository.FindByCode(ctx, code)
}

func (s *StatusService) CreateStatus(
	ctx context.Context,
	dto dto.CreateStatusDTO,
	smallIconHeader *multipart.FileHeader,
	bigIconHeader *multipart.FileHeader,
	smallIconFile io.ReadSeeker,
	bigIconFile io.ReadSeeker,
) (*dto.StatusDTO, error) {

	if err := utils.ValidateFile(smallIconHeader, smallIconFile, "status_icon_small"); err != nil {
		return nil, fmt.Errorf("маленькая иконка (16x16): %w", err)
	}
	if err := utils.ValidateFile(bigIconHeader, bigIconFile, "status_icon_big"); err != nil {
		return nil, fmt.Errorf("большая иконка (24x24): %w", err)
	}

	rulesSmall, _ := config.UploadContexts["status_icon_small"]
	smallPath, err := s.fileStorage.Save(smallIconFile, smallIconHeader.Filename, rulesSmall.PathPrefix)
	if err != nil {
		return nil, fmt.Errorf("ошибка сохранения маленькой иконки: %w", err)
	}

	rulesBig, _ := config.UploadContexts["status_icon_big"]
	bigPath, err := s.fileStorage.Save(bigIconFile, bigIconHeader.Filename, rulesBig.PathPrefix)
	if err != nil {
		return nil, fmt.Errorf("ошибка сохранения большой иконки: %w", err)
	}

	const urlPrefix = "/uploads/"

	return s.statusRepository.CreateStatus(ctx, dto, urlPrefix+smallPath, urlPrefix+bigPath)
}
func (s *StatusService) UpdateStatus(
	ctx context.Context,
	id uint64,
	dto dto.UpdateStatusDTO,
	smallIconHeader *multipart.FileHeader,
	bigIconHeader *multipart.FileHeader,
) (*dto.StatusDTO, error) {

	var smallPath, bigPath *string
	const urlPrefix = "/uploads/"

	if smallIconHeader != nil {
		file, err := smallIconHeader.Open()
		if err != nil {
			return nil, err
		}
		defer file.Close()

		if err = utils.ValidateFile(smallIconHeader, file, "status_icon_small"); err != nil {
			return nil, fmt.Errorf("маленькая иконка: %w", err)
		}

		rules, _ := config.UploadContexts["status_icon_small"]
		path, err := s.fileStorage.Save(file, smallIconHeader.Filename, rules.PathPrefix)
		if err != nil {
			return nil, err
		}

		fullPath := urlPrefix + path
		smallPath = &fullPath
	}

	if bigIconHeader != nil {
		file, err := bigIconHeader.Open()
		if err != nil {
			return nil, err
		}
		defer file.Close()

		if err = utils.ValidateFile(bigIconHeader, file, "status_icon_big"); err != nil {
			return nil, fmt.Errorf("большая иконка: %w", err)
		}

		rules, _ := config.UploadContexts["status_icon_big"]
		path, err := s.fileStorage.Save(file, bigIconHeader.Filename, rules.PathPrefix)
		if err != nil {
			return nil, err
		}

		fullPath := urlPrefix + path
		bigPath = &fullPath
	}

	return s.statusRepository.UpdateStatus(ctx, id, dto, smallPath, bigPath)
}

func (s *StatusService) DeleteStatus(ctx context.Context, id uint64) error {
	return s.statusRepository.DeleteStatus(ctx, id)
}

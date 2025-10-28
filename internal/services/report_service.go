package services

import (
	"context"

	"request-system/internal/authz"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
)

type ReportServiceInterface interface {
	GetReport(ctx context.Context, filter entities.ReportFilter) ([]entities.ReportItem, uint64, error)
}

type reportService struct {
	reportRepo repositories.ReportRepositoryInterface
	userRepo   repositories.UserRepositoryInterface
}

func NewReportService(reportRepo repositories.ReportRepositoryInterface, userRepo repositories.UserRepositoryInterface) ReportServiceInterface {
	return &reportService{
		reportRepo: reportRepo,
		userRepo:   userRepo,
	}
}

func (s *reportService) GetReport(ctx context.Context, filter entities.ReportFilter) ([]entities.ReportItem, uint64, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, 0, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, 0, err
	}
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, 0, apperrors.ErrUserNotFound
	}

	authContext := authz.Context{Actor: actor, Permissions: permissionsMap}
	if !authz.CanDo(authz.ReportView, authContext) {
		return nil, 0, apperrors.ErrForbidden
	}

	return s.reportRepo.GetReport(ctx, filter)
}

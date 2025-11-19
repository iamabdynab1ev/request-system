package services

import (
	"context"
	"database/sql"
	"time"

	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
)

type ReportServiceInterface interface {
	GetReportForExcel(ctx context.Context, filter entities.ReportFilter) ([]entities.ReportItem, uint64, error)
	GetReportDTOs(ctx context.Context, filter entities.ReportFilter) ([]dto.ReportItemDTO, uint64, error)
}

type reportService struct {
	reportRepo repositories.ReportRepositoryInterface
	userRepo   repositories.UserRepositoryInterface
	logger     *zap.Logger
}

func NewReportService(
	reportRepo repositories.ReportRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) ReportServiceInterface {
	return &reportService{
		reportRepo: reportRepo,
		userRepo:   userRepo,
		logger:     logger,
	}
}

// Новый общий метод для получения данных с учетом прав
func (s *reportService) getAndAuthorizeReport(ctx context.Context, filter entities.ReportFilter) ([]entities.ReportItem, uint64, error) {
	// 1. Получаем всю информацию о текущем пользователе (акторе) и его правах
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, 0, err // Ошибка получения ID пользователя
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, 0, err // Ошибка получения карты прав
	}
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, 0, apperrors.ErrUserNotFound
	}

	// 2. Проверяем базовое право на просмотр отчета
	authContext := authz.Context{Actor: actor, Permissions: permissionsMap}
	if !authz.CanDo(authz.ReportView, authContext) {
		s.logger.Warn("Попытка доступа к отчету без права report:view", zap.Uint64("userID", userID))
		return nil, 0, apperrors.ErrForbidden
	}

	// 3. Обогащаем фильтр информацией об акторе и его правах
	filter.Actor = actor
	filter.PermissionsMap = permissionsMap

	// 4. Вызываем репозиторий с полным контекстом для фильтрации
	return s.reportRepo.GetReport(ctx, filter)
}

func (s *reportService) GetReportForExcel(ctx context.Context, filter entities.ReportFilter) ([]entities.ReportItem, uint64, error) {
	// Просто вызываем наш новый общий метод
	return s.getAndAuthorizeReport(ctx, filter)
}

func (s *reportService) GetReportDTOs(ctx context.Context, filter entities.ReportFilter) ([]dto.ReportItemDTO, uint64, error) {
	// Получаем данные через наш новый авторизованный метод
	items, total, err := s.getAndAuthorizeReport(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]dto.ReportItemDTO, len(items))
	for i, item := range items {
		formatNullTime := func(t sql.NullTime) string {
			if t.Valid {
				return t.Time.Format(time.RFC3339)
			}
			return ""
		}
		nullStr := func(s sql.NullString) string {
			if s.Valid {
				return s.String
			}
			return ""
		}

		dtos[i] = dto.ReportItemDTO{
			OrderID:          item.OrderID,
			CreatorFio:       nullStr(item.CreatorFio),
			CreatedAt:        item.CreatedAt.Format(time.RFC3339),
			OrderTypeName:    nullStr(item.OrderTypeName),
			PriorityName:     nullStr(item.PriorityName),
			StatusName:       nullStr(item.StatusName),
			OrderName:        nullStr(item.OrderName),
			ResponsibleFio:   nullStr(item.ResponsibleFio),
			DelegatedAt:      formatNullTime(item.DelegatedAt),
			ExecutorFio:      nullStr(item.ExecutorFio),
			CompletedAt:      formatNullTime(item.CompletedAt),
			ResolutionTime:   nullStr(item.ResolutionTimeStr),
			SLAStatus:        nullStr(item.SLAStatus),
			SourceDepartment: nullStr(item.SourceDepartment),
			Comment:          nullStr(item.Comment),
		}
	}

	return dtos, total, nil
}

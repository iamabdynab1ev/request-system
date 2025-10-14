package services

import (
	"context"

	"request-system/internal/authz"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
)

// Интерфейсы и конструктор не меняются
type ReportServiceInterface interface {
	GetHistoryReport(ctx context.Context, filter entities.ReportFilter) ([]entities.HistoryReportItem, uint64, error)
}
type reportService struct {
	reportRepo repositories.ReportRepository
	userRepo   repositories.UserRepositoryInterface
}

func NewReportService(reportRepo repositories.ReportRepository, userRepo repositories.UserRepositoryInterface) ReportServiceInterface {
	return &reportService{
		reportRepo: reportRepo,
		userRepo:   userRepo,
	}
}

// Основная функция теперь ОЧЕНЬ ЧИСТАЯ
func (s *reportService) GetHistoryReport(ctx context.Context, filter entities.ReportFilter) ([]entities.HistoryReportItem, uint64, error) {
	// 1. Создаем контекст авторизации с помощью нашей новой вспомогательной функции.
	authContext, err := s.buildAuthzContext(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 2. Проверяем право доступа. Код стал лаконичным и понятным.
	if !authz.CanDo(authz.ReportView, *authContext) {
		return nil, 0, apperrors.ErrForbidden
	}

	// 3. Если все проверки пройдены, вызываем репозиторий.
	return s.reportRepo.GetHistoryReport(ctx, filter)
}

// Вспомогательная функция, написанная по образу и подобию твоей `buildAuthzContext` из OrderService.
func (s *reportService) buildAuthzContext(ctx context.Context) (*authz.Context, error) {
	// Шаг A: Извлекаем ID и права из контекста Go.
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	// Шаг B: Получаем полную модель "актора" (пользователя) из репозитория.
	actor, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	// Шаг C: Собираем и возвращаем контекст для системы авторизации.
	authContext := &authz.Context{
		Actor:       actor,
		Permissions: permissionsMap,
		Target:      nil, // Для отчетов Target не нужен
	}

	return authContext, nil
}

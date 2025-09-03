// internal/services/order_history_service.go

package services

import (
	"context"
	"fmt"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

type OrderHistoryServiceInterface interface {
	GetTimelineByOrderID(ctx context.Context, orderID uint64) ([]dto.TimelineEventDTO, error)
}

type OrderHistoryService struct {
	repo      repositories.OrderHistoryRepositoryInterface
	userRepo  repositories.UserRepositoryInterface
	orderRepo repositories.OrderRepositoryInterface
	logger    *zap.Logger
}

func NewOrderHistoryService(
	repo repositories.OrderHistoryRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	orderRepo repositories.OrderRepositoryInterface,
	logger *zap.Logger,
) OrderHistoryServiceInterface {
	return &OrderHistoryService{
		repo:      repo,
		userRepo:  userRepo,
		orderRepo: orderRepo,
		logger:    logger,
	}
}

func (s *OrderHistoryService) GetTimelineByOrderID(ctx context.Context, orderID uint64) ([]dto.TimelineEventDTO, error) {
	// --- Блок авторизации и получения данных (без изменений) ---
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
	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	authContext := authz.Context{Actor: actor, Permissions: permissionsMap, Target: order}
	if !authz.CanDo(authz.OrdersView, authContext) {
		return nil, apperrors.ErrForbidden
	}
	rawEvents, err := s.repo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if len(rawEvents) == 0 {
		return []dto.TimelineEventDTO{}, nil
	}

	// --- Конец блока без изменений ---

	var timeline []dto.TimelineEventDTO

	// --- ВОЗВРАЩАЕМ ЛОГИКУ ГРУППИРОВКИ, НО ПРАВИЛЬНУЮ ---
	i := 0
	for i < len(rawEvents) {
		currentEvent := rawEvents[i]

		// Начинаем новую "группу" событий
		eventDTO := dto.TimelineEventDTO{
			Actor: dto.ShortUserDTO{
				ID:  currentEvent.UserID,
				Fio: utils.NullStringToString(currentEvent.ActorFio),
			},
			CreatedAt: currentEvent.CreatedAt.Format("02.01.2006 / 15:04"),
			Lines:     []string{},
		}

		// Внутренний цикл для сбора всех событий, произошедших в одну секунду от одного юзера
		j := i
		for j < len(rawEvents) &&
			rawEvents[j].UserID == currentEvent.UserID &&
			rawEvents[j].CreatedAt.Unix() == currentEvent.CreatedAt.Unix() {

			event := rawEvents[j]
			var line string

			// Если тип события - КОММЕНТАРИЙ, мы его текст кладем в поле Comment группы
			if event.EventType == "COMMENT" {
				if event.Comment != nil && *event.Comment != "" {
					eventDTO.Comment = event.Comment
				}
			} else {
				// Для всех остальных событий мы генерируем `line` и добавляем в `lines`
				switch event.EventType {
				case "CREATE":
					line = fmt.Sprintf("Создана заявка: «%s»", order.Name)
				case "STATUS_CHANGE":
					if event.NewValue != nil {
						line = fmt.Sprintf("Статус изменен на: «%s»", *event.NewValue)
					}
				case "DELEGATION":
					if event.NewValue != nil {
						line = fmt.Sprintf("Назначен исполнитель: %s", *event.NewValue)
					}
				case "ATTACHMENT_ADDED":
					if event.NewValue != nil {
						line = fmt.Sprintf(`Прикреплен файл: %s`, *event.NewValue)
					}
				//... и так далее для всех остальных типов
				case "DEPARTMENT_CHANGE":
					if event.Comment != nil {
						line = *event.Comment
					}
				case "DURATION_CHANGE":
					if event.NewValue != nil {
						line = fmt.Sprintf("Срок выполнения: %s", *event.NewValue)
					}
				case "PRIORITY_CHANGE":
					if event.NewValue != nil {
						line = fmt.Sprintf("Приоритет изменен на: %s", *event.NewValue)
					}
				case "NAME_CHANGE":
					if event.NewValue != nil {
						line = fmt.Sprintf("Название заявки изменено на: «%s»", *event.NewValue)
					}
				case "ADDRESS_CHANGE":
					if event.NewValue != nil {
						line = fmt.Sprintf("Адрес заявки изменен на: «%s»", *event.NewValue)
					}
				}
				if line != "" {
					eventDTO.Lines = append(eventDTO.Lines, line)
				}
			}
			j++
		}

		// Добавляем собранную группу в итоговый список
		timeline = append(timeline, eventDTO)
		i = j // Передвигаем внешний счетчик на конец обработанной группы
	}

	return timeline, nil
}

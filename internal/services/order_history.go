package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	// Добавляем strings

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
		s.logger.Warn("Отказано в доступе при просмотре истории заявки",
			zap.Uint64("orderID", orderID),
			zap.Uint64("actorID", actor.ID),
		)
		return nil, apperrors.ErrForbidden
	}

	rawEvents, err := s.repo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if len(rawEvents) == 0 {
		return []dto.TimelineEventDTO{}, nil
	}

	sort.SliceStable(rawEvents, func(i, j int) bool {
		if rawEvents[i].CreatedAt.Equal(rawEvents[j].CreatedAt) {
			return rawEvents[i].ID < rawEvents[j].ID
		}
		return rawEvents[i].CreatedAt.Before(rawEvents[j].CreatedAt)
	})

	var timeline []dto.TimelineEventDTO
	i := 0
	for i < len(rawEvents) {
		currentEvent := rawEvents[i]
		eventDTO := dto.TimelineEventDTO{
			Actor: dto.ShortUserDTO{
				ID:  currentEvent.UserID,
				Fio: utils.NullStringToString(currentEvent.ActorFio),
			},
			CreatedAt: currentEvent.CreatedAt.Format("02.01.2006 / 15:04"),
			Lines:     []string{},
		}

		// Назначаем иконку по умолчанию
		if currentEvent.EventType != "" {
			eventDTO.Icon = strings.ToLower(currentEvent.EventType)
		}

		j := i
		for j < len(rawEvents) &&
			rawEvents[j].UserID == currentEvent.UserID &&
			rawEvents[j].CreatedAt.Unix() == currentEvent.CreatedAt.Unix() {

			event := rawEvents[j]
			var line string

			switch event.EventType {
			case "CREATE":
				eventDTO.Icon = "status_open"
				if event.Comment != nil {
					line = fmt.Sprintf("Создал(а) заявку: «%s»", *event.Comment)
				}
			case "STATUS_CHANGE":
				eventDTO.Icon = "status_inprogress"
				if event.NewValue != nil {
					line = fmt.Sprintf("Статус изменен на: «%s»", *event.NewValue)
				}
			case "DELEGATION":
				eventDTO.Icon = "status_inprogress"
				if event.NewValue != nil {
					line = fmt.Sprintf("Назначены исполнитель: %s", *event.NewValue)
				}
				if event.Comment != nil && strings.Contains(*event.Comment, "после перевода заявки") {
					line += fmt.Sprintf(" (%s)", *event.Comment)
				}
			case "DEPARTMENT_CHANGE":
				eventDTO.Icon = "status_transfer"
				if event.Comment != nil {
					line = *event.Comment
				}
			case "DURATION_CHANGE":
				eventDTO.Icon = "timer"
				if event.NewValue != nil {
					line = fmt.Sprintf("Срок выполнения изменен на: %s", *event.NewValue)
				}
			case "PRIORITY_CHANGE":
				eventDTO.Icon = "priority"
				if event.NewValue != nil {
					line = fmt.Sprintf("Приоритет изменен на: %s", *event.NewValue)
				}
			case "NAME_CHANGE":
				eventDTO.Icon = "title"
				if event.NewValue != nil {
					line = fmt.Sprintf("Название заявки изменено на: «%s»", *event.NewValue)
				}
			case "ADDRESS_CHANGE":
				eventDTO.Icon = "address"
				if event.NewValue != nil {
					line = fmt.Sprintf("Адрес заявки изменен на: «%s»", *event.NewValue)
				}

			case "COMMENT":
				eventDTO.Icon = "comment"
				if event.Comment != nil {
					line = *event.Comment
				}
			case "ATTACHMENT_ADDED":
				eventDTO.Icon = "attachment"
				if event.Comment != nil {
					line = fmt.Sprintf(`Прикреплен файл: %s`, *event.Comment)
				}
			}

			if line != "" {
				eventDTO.Lines = append(eventDTO.Lines, line)
			}
			j++
		}

		if len(eventDTO.Lines) > 0 {
			timeline = append(timeline, eventDTO)
		}
		i = j
	}

	return timeline, nil
}

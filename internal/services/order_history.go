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

		j := i
		for j < len(rawEvents) && rawEvents[j].UserID == currentEvent.UserID && rawEvents[j].CreatedAt.Unix() == currentEvent.CreatedAt.Unix() {
			event := rawEvents[j]
			var line string

			// <<<--- НАЧАЛО: ИСПРАВЛЕНИЯ ДЛЯ SQL.NULLSTRING ---
			if event.EventType == "COMMENT" {
				if event.Comment.Valid && event.Comment.String != "" {
					comment := event.Comment.String
					eventDTO.Comment = &comment
				}
			} else {
				switch event.EventType {
				case "CREATE":
					line = fmt.Sprintf("Создана заявка: «%s»", order.Name)
				case "STATUS_CHANGE":
					if event.NewValue.Valid {
						line = fmt.Sprintf("Статус изменен на: «%s»", event.NewValue.String)
					}
				case "DELEGATION":
					if event.NewValue.Valid {
						line = fmt.Sprintf("Назначен исполнитель: %s", event.NewValue.String)
					}
				case "ATTACHMENT_ADDED":
					if event.NewValue.Valid {
						line = fmt.Sprintf(`Прикреплен файл: %s`, event.NewValue.String)
					}

				case "DEPARTMENT_CHANGE":
					if event.Comment.Valid {
						line = event.Comment.String
					}
				case "DURATION_CHANGE":
					if event.NewValue.Valid {
						line = fmt.Sprintf("Срок выполнения: %s", event.NewValue.String)
					}
				case "PRIORITY_CHANGE":
					if event.NewValue.Valid {
						line = fmt.Sprintf("Приоритет изменен на: %s", event.NewValue.String)
					}
				case "NAME_CHANGE":
					if event.NewValue.Valid {
						line = fmt.Sprintf("Название заявки изменено на: «%s»", event.NewValue.String)
					}
				case "ADDRESS_CHANGE":
					if event.NewValue.Valid {
						line = fmt.Sprintf("Адрес заявки изменен на: «%s»", event.NewValue.String)
					}
				}
				if line != "" {
					eventDTO.Lines = append(eventDTO.Lines, line)
				}
			}
			// <<<--- КОНЕЦ ИСПРАВЛЕНИЙ ---
			if event.Attachment != nil {
				eventDTO.Attachment = &dto.AttachmentResponseDTO{
					ID:       event.Attachment.ID,
					FileName: event.Attachment.FileName,
					FileSize: event.Attachment.FileSize,
					FileType: event.Attachment.FileType,
					URL:      event.Attachment.FilePath,
				}
			}
			j++
		}

		timeline = append(timeline, eventDTO)
		i = j
	}

	return timeline, nil
}

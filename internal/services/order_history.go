package services

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/repositories"
	"sort"

	"go.uber.org/zap"
)

type OrderHistoryServiceInterface interface {
	GetTimelineByOrderID(ctx context.Context, orderID uint64) ([]dto.TimelineEventDTO, error)
}

type OrderHistoryService struct {
	repo   repositories.OrderHistoryRepositoryInterface
	logger *zap.Logger
}

func NewOrderHistoryService(repo repositories.OrderHistoryRepositoryInterface, logger *zap.Logger) OrderHistoryServiceInterface {
	return &OrderHistoryService{repo: repo, logger: logger}
}

func (s *OrderHistoryService) GetTimelineByOrderID(ctx context.Context, orderID uint64) ([]dto.TimelineEventDTO, error) {
	rawEvents, err := s.repo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if len(rawEvents) == 0 {
		return []dto.TimelineEventDTO{}, nil
	}

	// Сортировка событий по времени и ID для стабильного порядка
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

		// Создаем базовый объект для группы событий
		eventDTO := dto.TimelineEventDTO{
			Actor: dto.ShortUserDTO{
				ID:  currentEvent.UserID,
				Fio: currentEvent.ActorFio.String,
			},
			CreatedAt: currentEvent.CreatedAt.Format("02.01.2006 / 15:04"),
			Lines:     []string{},
		}

		// Группируем все события, произошедшие в одну секунду одним и тем же пользователем
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
				if eventDTO.Icon == "" {
					eventDTO.Icon = "status_change"
				}
				// ИСПРАВЛЕНО: Используем поле NewStatusName, которое содержит название статуса, а не его ID.
				// Добавлена проверка, что название статуса было успешно получено из БД.
				if event.NewStatusName.Valid {
					line = fmt.Sprintf("Изменен статус заявки на «%s»", event.NewStatusName.String)
				} else if event.NewValue != nil {
					// Запасной вариант, если название статуса по какой-то причине не нашлось
					line = fmt.Sprintf("Изменен статус заявки на ID: %s", *event.NewValue)
				}
			case "DELEGATION":
				if eventDTO.Icon == "" {
					eventDTO.Icon = "status_inprogress"
				}
				if event.NewValue != nil {
					line = fmt.Sprintf("Назначен(а) исполнитель: %s", *event.NewValue)
				}
			case "COMMENT":
				if eventDTO.Icon == "" {
					eventDTO.Icon = "comment"
				}
				if event.Comment != nil {
					line = *event.Comment
				}
			case "ATTACHMENT_ADDED":
				if eventDTO.Icon == "" {
					eventDTO.Icon = "attachment"
				}
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

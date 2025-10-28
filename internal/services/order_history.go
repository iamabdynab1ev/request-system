package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/repositories"
	"request-system/pkg/utils"
)

type OrderHistoryServiceInterface interface {
	GetTimelineByOrderID(ctx context.Context, orderID uint64, limitStr, offsetStr string) ([]dto.TimelineEventDTO, error)
}

type OrderHistoryService struct {
	repo         repositories.OrderHistoryRepositoryInterface
	orderService OrderServiceInterface
	userRepo     repositories.UserRepositoryInterface
	logger       *zap.Logger
}

func NewOrderHistoryService(
	repo repositories.OrderHistoryRepositoryInterface,
	orderService OrderServiceInterface,
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) OrderHistoryServiceInterface {
	return &OrderHistoryService{
		repo:         repo,
		orderService: orderService,
		userRepo:     userRepo,
		logger:       logger,
	}
}

func (s *OrderHistoryService) GetTimelineByOrderID(ctx context.Context, orderID uint64, limitStr, offsetStr string) ([]dto.TimelineEventDTO, error) {
	_, err := s.orderService.FindOrderByID(ctx, orderID)
	if err != nil {
		s.logger.Error("Доступ к истории запрещен", zap.Uint64("orderID", orderID), zap.Error(err))
		return nil, err
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	historyEvents, err := s.repo.FindByOrderID(ctx, orderID, uint64(limit), uint64(offset))
	if err != nil || len(historyEvents) == 0 {
		return []dto.TimelineEventDTO{}, err
	}

	var timeline []dto.TimelineEventDTO

	currentBlock := &dto.TimelineEventDTO{
		Lines:     []string{},
		Actor:     getActorFromEvent(historyEvents[0], s, ctx, historyEvents),
		CreatedAt: historyEvents[0].CreatedAt.Format("02.01.2006 / 15:04"),
	}
	addEventToBlock(ctx, currentBlock, historyEvents[0], s)

	for i := 1; i < len(historyEvents); i++ {
		event := historyEvents[i]
		prevEvent := historyEvents[i-1]

		if event.UserID == prevEvent.UserID && event.CreatedAt.Sub(prevEvent.CreatedAt) < 5*time.Second {
			addEventToBlock(ctx, currentBlock, event, s)
		} else {
			timeline = append(timeline, *currentBlock)
			currentBlock = &dto.TimelineEventDTO{
				Lines:     []string{},
				Actor:     getActorFromEvent(event, s, ctx, historyEvents),
				CreatedAt: event.CreatedAt.Format("02.01.2006 / 15:04"),
			}
			addEventToBlock(ctx, currentBlock, event, s)
		}
	}

	timeline = append(timeline, *currentBlock)
	s.logger.Info("Таймлайн сформирован", zap.Int("blocks", len(timeline)))
	return timeline, nil
}

func getActorFromEvent(event repositories.OrderHistoryItem, s *OrderHistoryService, ctx context.Context, allEvents []repositories.OrderHistoryItem) dto.ShortUserDTO {
	user, err := s.userRepo.FindUserByID(ctx, event.UserID)
	var fio string
	if err == nil && user != nil {
		fio = user.Fio
	} else {
		fio = "Неизвестный пользователь"
	}

	var role string
	var creatorID uint64
	for _, e := range allEvents {
		if e.EventType == "CREATE" {
			creatorID = e.UserID
			break
		}
	}

	if event.UserID == creatorID {
		role = "creator"
	} else {

		isDelegator := false
		for _, e := range allEvents {
			if e.UserID == event.UserID && e.EventType == "DELEGATION" {
				isDelegator = true
				break
			}
		}
		if isDelegator {
			role = "delegator"
		} else {
			role = "executor"
		}
	}

	return dto.ShortUserDTO{ID: event.UserID, Fio: fio, Role: role}
}

func addEventToBlock(ctx context.Context, block *dto.TimelineEventDTO, event repositories.OrderHistoryItem, s *OrderHistoryService) {
	fieldMap := map[string]string{"OTDEL_CHANGE": "Отдел", "NAME_CHANGE": "Имя заявки", "ADDRESS_CHANGE": "Адрес"}

	if event.EventType == "COMMENT" && event.Comment.Valid {
		block.Comment = &event.Comment.String
	}

	if event.NewValue.Valid || event.EventType == "CREATE" || event.EventType == "COMMENT" {
		var line string
		newValue := utils.NullStringToString(event.NewValue)

		switch event.EventType {
		case "CREATE":
			line = fmt.Sprintf("Создана заявка: «%s»", newValue)
		case "DELEGATION":
			if event.Comment.Valid {
				line = event.Comment.String
			}
		case "ATTACHMENT_ADD":
			line = fmt.Sprintf("Прикреплен файл: %s", newValue)

			if event.Attachment != nil {
				block.Attachment = &dto.AttachmentResponseDTO{
					ID:       event.Attachment.ID,
					FileName: event.Attachment.FileName,
					URL:      "/uploads/" + event.Attachment.FilePath,
				}
			}
		case "STATUS_CHANGE":
			newID, _ := strconv.ParseUint(newValue, 10, 64)
			if newStatus, err := s.orderService.GetStatusByID(ctx, newID); err == nil {
				line = fmt.Sprintf("Установлен статус: «%s»", newStatus.Name)
			}
		case "PRIORITY_CHANGE":
			newID, _ := strconv.ParseUint(newValue, 10, 64)
			if newPriority, err := s.orderService.GetPriorityByID(ctx, newID); err == nil {
				line = fmt.Sprintf("Установлен приоритет: «%s»", newPriority.Name)
			}

		case "DURATION_CHANGE":
			parsedTime, err := time.Parse(time.RFC3339, newValue)
			if err == nil {
				line = fmt.Sprintf("Установлен срок выполнения: %s", parsedTime.Format("02.01.2006 15:04"))
			} else {
				line = fmt.Sprintf("Срок выполнения изменен на: %s", newValue)
			}

		case "COMMENT":
			line = fmt.Sprintf("Добавлен комментарий: %s", newValue)
		default:
			if humanField, ok := fieldMap[event.EventType]; ok {
				oldValue := utils.NullStringToString(event.OldValue)
				line = fmt.Sprintf("Изменено поле '%s': «%s» -> «%s»", humanField, oldValue, newValue)
			}
		}

		if line != "" {
			block.Lines = append(block.Lines, line)
		}
	}
}

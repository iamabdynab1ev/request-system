package services

import (
	"context"
	"fmt"
	"time"

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
	// 1. Проверка прав
	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, _ := s.userRepo.FindUserByID(ctx, userID)

	order, err := s.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	authContext := authz.Context{Actor: actor, Permissions: permissionsMap, Target: order}
	if !authz.CanDo(authz.OrdersView, authContext) {
		return nil, apperrors.ErrForbidden
	}

	// 2. Получаем все события через OrderHistoryRepository
	historyEvents, err := s.repo.FindByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if len(historyEvents) == 0 {
		return []dto.TimelineEventDTO{}, nil
	}

	// --- 3. ИСПРАВЛЕННАЯ ГРУППИРОВКА ПО ВРЕМЕНИ ---

	var timeline []dto.TimelineEventDTO

	currentBlock := dto.TimelineEventDTO{
		Lines:     []string{},
		Actor:     dto.ShortUserDTO{ID: historyEvents[0].UserID, Fio: utils.NullStringToString(historyEvents[0].ActorFio)},
		CreatedAt: historyEvents[0].CreatedAt.Format("02.01.2006 / 15:04"),
	}
	addEventToBlock(&currentBlock, historyEvents[0]) // Добавляем первое событие

	for i := 1; i < len(historyEvents); i++ {
		event := historyEvents[i]
		prevEvent := historyEvents[i-1]

		// --- НОВОЕ, УПРОЩЕННОЕ УСЛОВИЕ ГРУППИРОВКИ ---
		// Группируем ВСЕ события (включая комментарии), если они произошли
		// почти одновременно (< 1 секунды) и у них один автор.
		if event.UserID == prevEvent.UserID && event.CreatedAt.Sub(prevEvent.CreatedAt) < time.Second {
			// Добавляем событие в ТЕКУЩИЙ блок
			addEventToBlock(&currentBlock, event)
		} else {
			// ИНАЧЕ, это новое событие, создаем новый блок
			timeline = append(timeline, currentBlock) // Завершаем старый блок

			currentBlock = dto.TimelineEventDTO{ // Создаем новый
				Lines:     []string{},
				Actor:     dto.ShortUserDTO{ID: event.UserID, Fio: utils.NullStringToString(event.ActorFio)},
				CreatedAt: event.CreatedAt.Format("02.01.2006 / 15:04"),
			}
			addEventToBlock(&currentBlock, event)
		}
	}

	timeline = append(timeline, currentBlock) // Добавляем самый последний блок

	return timeline, nil
}

// ПОЛНОСТЬЮ ЗАМЕНИ СВОЙ addEventToBlock НА ЭТОТ
func addEventToBlock(block *dto.TimelineEventDTO, event repositories.OrderHistoryItem) {
	if event.EventType == "COMMENT" && event.Comment.Valid {
		comment := event.Comment.String
		block.Comment = &comment
	} else if event.NewValue.Valid {
		var line string
		switch event.EventType {
		case "CREATE":
			line = fmt.Sprintf("Создана заявка: «%s»", event.NewValue.String)
		case "DELEGATION":
			// !!! ИСПРАВЛЕНО: Теперь строка уже содержит "Назначено на:", поэтому просто используем ее
			line = event.NewValue.String
		case "STATUS_CHANGE":
			line = fmt.Sprintf("Статус изменен на: «%s»", event.NewValue.String)
		case "PRIORITY_CHANGE":
			line = fmt.Sprintf("Установлен приоритет: «%s»", event.NewValue.String)
		case "ATTACHMENT_ADDED":
			line = fmt.Sprintf("Прикреплен файл: %s", event.NewValue.String)
		case "DURATION_CHANGE":
			line = fmt.Sprintf("Установлено время выполнения до: %s", event.NewValue.String)
		}
		if line != "" {
			block.Lines = append(block.Lines, line)
		}
	}

	// Логика добавления вложения (остается без изменений)
	if event.Attachment != nil {
		block.Attachment = &dto.AttachmentResponseDTO{
			ID:       event.Attachment.ID,
			FileName: event.Attachment.FileName,
			FileSize: event.Attachment.FileSize,
			FileType: event.Attachment.FileType,
			URL:      event.Attachment.FilePath,
		}
	}
}

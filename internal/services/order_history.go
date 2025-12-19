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
	repo              repositories.OrderHistoryRepositoryInterface
	userRepo          repositories.UserRepositoryInterface
	departmentService DepartmentServiceInterface
	otdelService      OtdelServiceInterface
	statusRepo        repositories.StatusRepositoryInterface
	priorityRepo      repositories.PriorityRepositoryInterface
	logger            *zap.Logger
}

func NewOrderHistoryService(
	repo repositories.OrderHistoryRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	departmentService DepartmentServiceInterface,
	otdelService OtdelServiceInterface,
	statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface,
	logger *zap.Logger,
) OrderHistoryServiceInterface {
	return &OrderHistoryService{
		repo:              repo,
		userRepo:          userRepo,
		departmentService: departmentService,
		otdelService:      otdelService,
		statusRepo:        statusRepo,
		priorityRepo:      priorityRepo,
		logger:            logger,
	}
}

func (s *OrderHistoryService) GetTimelineByOrderID(ctx context.Context, orderID uint64, limitStr, offsetStr string) ([]dto.TimelineEventDTO, error) {
	// ПРОВЕРКА ПРАВ ДОСТУПА УДАЛЕНА ОТСЮДА.

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

	meta := buildHistoryMetadata(historyEvents)

	var timeline []dto.TimelineEventDTO
	currentBlock := createTimelineBlock(historyEvents[0], s, ctx, meta)
	addEventToBlock(ctx, currentBlock, historyEvents[0], s)

	for i := 1; i < len(historyEvents); i++ {
		event := historyEvents[i]
		prevEvent := historyEvents[i-1]

		isSameTransaction := event.TxID != nil && prevEvent.TxID != nil && event.TxID.String() == prevEvent.TxID.String()

		if isSameTransaction {
			addEventToBlock(ctx, currentBlock, event, s)
		} else {
			timeline = append(timeline, *currentBlock)
			currentBlock = createTimelineBlock(event, s, ctx, meta)
			addEventToBlock(ctx, currentBlock, event, s)
		}
	}

	timeline = append(timeline, *currentBlock)
	s.logger.Info("Таймлайн сформирован", zap.Int("blocks", len(timeline)), zap.Uint64("orderID", orderID))
	return timeline, nil
}

type historyMetadata struct {
	creatorID    uint64
	delegatorIDs map[uint64]struct{}
}

func buildHistoryMetadata(events []repositories.OrderHistoryItem) historyMetadata {
	meta := historyMetadata{delegatorIDs: make(map[uint64]struct{})}
	for _, e := range events {
		if e.EventType == "CREATE" {
			meta.creatorID = e.UserID
		}
		if e.EventType == "DELEGATION" {
			meta.delegatorIDs[e.UserID] = struct{}{}
		}
	}
	return meta
}

// createTimelineBlock - хелпер для создания чистого блока таймлайна.
func createTimelineBlock(event repositories.OrderHistoryItem, s *OrderHistoryService, ctx context.Context, meta historyMetadata) *dto.TimelineEventDTO {
	return &dto.TimelineEventDTO{
		Lines:     []string{},
		Actor:     getActorFromEvent(event, s, ctx, meta),
		CreatedAt: event.CreatedAt.Format("02.01.2006 / 15:04"),
	}
}

// getActorFromEvent - теперь работает за O(1) благодаря заранее собранным мета-данным.
func getActorFromEvent(event repositories.OrderHistoryItem, s *OrderHistoryService, ctx context.Context, meta historyMetadata) dto.ShortUserDTO {
	user, err := s.userRepo.FindUserByID(ctx, event.UserID)
	fio := "Неизвестный пользователь"
	if err == nil && user != nil {
		fio = user.Fio
	}

	var role string
	if event.UserID == meta.creatorID {
		role = "creator"
	} else {
		if _, isDelegator := meta.delegatorIDs[event.UserID]; isDelegator {
			role = "delegator"
		} else {
			role = "executor"
		}
	}

	return dto.ShortUserDTO{ID: event.UserID, Fio: fio, Role: role}
}

func addEventToBlock(ctx context.Context, block *dto.TimelineEventDTO, event repositories.OrderHistoryItem, s *OrderHistoryService) {
	if event.EventType == "COMMENT" && event.Comment.Valid {
		commentText := event.Comment.String
		block.Comment = &commentText
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
		case "DEPARTMENT_CHANGE":
			id, err := strconv.ParseUint(newValue, 10, 64)
			if err == nil {
				if dept, err := s.departmentService.FindDepartment(ctx, id); err == nil {
					line = fmt.Sprintf("Заявка переведена в департамент: «%s»", dept.Name)
				} else {
					s.logger.Warn("Не удалось найти департамент по ID из истории", zap.Uint64("deptID", id))
					line = fmt.Sprintf("Изменено поле 'Департамент': ID на %s", newValue)
				}
			}
		case "OTDEL_CHANGE":
			id, err := strconv.ParseUint(newValue, 10, 64)
			if err == nil {
				if otdel, err := s.otdelService.FindOtdel(ctx, id); err == nil {
					line = fmt.Sprintf("Заявка переведена в отдел: «%s»", otdel.Name)
				} else {
					s.logger.Warn("Не удалось найти отдел по ID из истории", zap.Uint64("otdelID", id))
					line = fmt.Sprintf("Изменено поле 'Отдел': ID на %s", newValue)
				}
			}
		case "STATUS_CHANGE":
			newID, _ := strconv.ParseUint(newValue, 10, 64)
			if newStatus, err := s.statusRepo.FindStatus(ctx, newID); err == nil {
				line = fmt.Sprintf("Установлен статус: «%s»", newStatus.Name)
			}
		case "PRIORITY_CHANGE":
			newID, _ := strconv.ParseUint(newValue, 10, 64)
			if newPriority, err := s.priorityRepo.FindByID(ctx, newID); err == nil {
				line = fmt.Sprintf("Установлен приоритет: «%s»", newPriority.Name)
			}
		case "DURATION_CHANGE":
			parsedTime, err := time.Parse(time.RFC3339, newValue)
			if err == nil {
				line = fmt.Sprintf("Установлен срок выполнения до: %s", parsedTime.Format("02.01.2006 15:04"))
			} else {
				line = fmt.Sprintf("Срок выполнения изменен на: %s", newValue)
			}
		case "COMMENT":

		case "NAME_CHANGE":
			oldValue := utils.NullStringToString(event.OldValue)
			line = fmt.Sprintf("Изменено название заявки: «%s» на «%s»", oldValue, newValue)

		case "ADDRESS_CHANGE":
			oldValue := utils.NullStringToString(event.OldValue)
			line = fmt.Sprintf("Изменен адрес: «%s» на «%s»", oldValue, newValue)
		case "EQUIPMENT_CHANGE":
			line = fmt.Sprintf("Изменено оборудование: ID на %s", newValue)

		case "EQUIPMENT_TYPE_CHANGE":
			line = fmt.Sprintf("Изменен тип оборудования: ID на %s", newValue)

		case "ORDER_TYPE_CHANGE":
			line = fmt.Sprintf("Изменен тип заявки: ID на %s", newValue)

		default:
			fieldMap := map[string]string{}
			if humanField, ok := fieldMap[event.EventType]; ok {
				oldValue := utils.NullStringToString(event.OldValue)
				line = fmt.Sprintf("Изменено поле '%s': «%s» на «%s»", humanField, oldValue, newValue)
			}
		}

		if line != "" {
			block.Lines = append(block.Lines, line)
		}
	}
}

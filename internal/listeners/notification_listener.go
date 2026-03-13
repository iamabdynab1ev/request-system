package listeners

import (
	"context"
	"fmt"
	"sort"
	"strconv" 
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"request-system/internal/entities"
	"request-system/internal/events"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/eventbus"
	"request-system/pkg/telegram"
	"request-system/pkg/websocket"
)


type eventGroupKey struct {
	OrderID uint64
	TxID    string
}

type eventGroup struct {
	events []events.OrderHistoryCreatedEvent
	timer  *time.Timer
	
}



type NotificationListener struct {
	notificationService   services.NotificationServiceInterface
	wsNotificationService services.WebSocketNotificationServiceInterface
	userRepo              repositories.UserRepositoryInterface
	statusRepo            repositories.StatusRepositoryInterface
	priorityRepo          repositories.PriorityRepositoryInterface
	frontendCfg           config.FrontendConfig
	serverCfg             config.ServerConfig
	logger                *zap.Logger
	groups                map[eventGroupKey]*eventGroup
	groupsMu              sync.Mutex
}

func NewNotificationListener(
	notificationService services.NotificationServiceInterface,
	wsNotificationService services.WebSocketNotificationServiceInterface,
	userRepo repositories.UserRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.PriorityRepositoryInterface,
	frontendCfg config.FrontendConfig,
	serverCfg config.ServerConfig,
	logger *zap.Logger,
) *NotificationListener {
	return &NotificationListener{
		notificationService:   notificationService,
		wsNotificationService: wsNotificationService,
		userRepo:              userRepo,
		statusRepo:            statusRepo,
		priorityRepo:          priorityRepo,
		frontendCfg:           frontendCfg,
		serverCfg:             serverCfg,
		logger:                logger,
		groups:                make(map[eventGroupKey]*eventGroup),
	}
}

func (l *NotificationListener) Register(bus *eventbus.Bus) {
	bus.Subscribe("order.history.created", l.handleOrderHistoryCreated)
	l.logger.Info("NotificationListener (с группировкой) подписан на событие 'order.history.created'")
}


func (l *NotificationListener) handleOrderHistoryCreated(ctx context.Context, event eventbus.Event) error {
	e, ok := event.(events.OrderHistoryCreatedEvent)
	if !ok || e.HistoryItem.TxID == nil {
		return nil
	}

	key := eventGroupKey{
		OrderID: e.HistoryItem.OrderID,
		TxID:    e.HistoryItem.TxID.String(),
	}

	l.groupsMu.Lock()
	defer l.groupsMu.Unlock()

	group, exists := l.groups[key]
	if !exists {
		group = &eventGroup{}
		l.groups[key] = group
		group.timer = time.AfterFunc(2*time.Second, func() {
			l.sendGroupedNotification(context.Background(), key)
		})
	}

	group.events = append(group.events, e)
	l.logger.Info("Событие добавлено в группу", zap.Any("key", key), zap.Int("totalInGroup", len(group.events)))

	return nil
}


func (l *NotificationListener) sendGroupedNotification(ctx context.Context, key eventGroupKey) {
    l.groupsMu.Lock()
    group, exists := l.groups[key]
    if !exists {
        l.groupsMu.Unlock()
        return
    }
    delete(l.groups, key)
    l.groupsMu.Unlock()

    if len(group.events) == 0 {
        return
    }
    sort.Slice(group.events, func(i, j int) bool {
        return group.events[i].HistoryItem.CreatedAt.Before(group.events[j].HistoryItem.CreatedAt)
    })

    recipients, err := l.determineRecipients(ctx, group.events)
	if err != nil {
		l.logger.Error("Не удалось определить получателей для отправки", zap.Error(err), zap.Any("key", key))
		return
	}


	for _, user := range recipients {
		message := l.formatGroupedMessage(ctx, group.events, &user)
		if message == "" {
			continue
		}

		if !user.TelegramChatID.Valid || user.TelegramChatID.Int64 == 0 {
			continue
		}

		if err := l.notificationService.SendFormattedMessage(ctx, user.TelegramChatID.Int64, message); err != nil {
			l.logger.Error("Не удалось отправить сгруппированное уведомление", zap.Uint64("userID", user.ID), zap.Error(err))
		}
		payload, err := l.formatWebSocketPayload(ctx, group.events, &user)
		if err != nil {
			l.logger.Error("Не удалось сформировать WebSocket payload", zap.Uint64("userID", user.ID), zap.Error(err))
			continue 
		}
		if payload != nil {
			err := l.wsNotificationService.SendNotification(user.ID, payload, "notification")
			if err != nil {
				l.logger.Error("Не удалось отправить WebSocket-уведомление", zap.Uint64("userID", user.ID), zap.Error(err))
			}
		}
	}
}


func (l *NotificationListener) determineRecipients(ctx context.Context, groupEvents []events.OrderHistoryCreatedEvent) ([]entities.User, error) {
	if len(groupEvents) == 0 {
		return nil, nil
	}

	firstEvent := groupEvents[0]
	order, ok := firstEvent.Order.(*entities.Order)
	if !ok {
		return nil, fmt.Errorf("сущность Order не была передана в событии")
	}
	actor, _ := firstEvent.Actor.(*entities.User)

	userIDs := make(map[uint64]struct{})

	userIDs[order.CreatorID] = struct{}{}
	if order.ExecutorID != nil {
		userIDs[*order.ExecutorID] = struct{}{}
	}
	for _, e := range groupEvents {
    if e.HistoryItem.UserID > 0 {
        userIDs[e.HistoryItem.UserID] = struct{}{}
    }
    if e.HistoryItem.EventType == "DELEGATION" && e.HistoryItem.OldValue.Valid {
        oldExecutorID, err := strconv.ParseUint(e.HistoryItem.OldValue.String, 10, 64)
        if err == nil && oldExecutorID > 0 {
            userIDs[oldExecutorID] = struct{}{}
        }
    }
}


if actor != nil {
		delete(userIDs, actor.ID)
	}

	if len(userIDs) == 0 {
		return nil, nil
	}

	ids := make([]uint64, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}

	usersMap, err := l.userRepo.FindUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	recipients := make([]entities.User, 0, len(usersMap))
	for _, user := range usersMap {
		recipients = append(recipients, user)
	}

	return recipients, nil
}

func (l *NotificationListener) formatGroupedMessage(ctx context.Context, events []events.OrderHistoryCreatedEvent, recipient *entities.User) string {
	if len(events) == 0 || recipient == nil {
		return ""
	}

	escape := telegram.EscapeTextForMarkdownV2
	firstEvent := events[0]
	actor, _ := firstEvent.Actor.(*entities.User)
	order, _ := firstEvent.Order.(*entities.Order)

	if actor == nil || order == nil {
		return ""
	}

	actorName := escape(actor.Fio)
	orderName := escape(order.Name)
	orderLink := fmt.Sprintf("[Посмотреть мои заявки](%s/order?participant=me)", l.frontendCfg.BaseURL)

	var sb strings.Builder
	var mainAction string
	details := make(map[string]string)
	var comment, attachmentText string

	for _, e := range events {
		item := e.HistoryItem
		switch item.EventType {
		case "CREATE":
			mainAction = fmt.Sprintf("✅ %s создал\\(а\\) новую заявку №%d\n*%s*", actorName, order.ID, orderName)
		case "STATUS_CHANGE":
			if statusID, err := strconv.ParseUint(item.NewValue.String, 10, 64); err == nil {
				if status, _ := l.statusRepo.FindStatus(ctx, statusID); status != nil {
					details["Статус"] = escape(status.Name)
				}
			}
		case "PRIORITY_CHANGE":
			if prioID, err := strconv.ParseUint(item.NewValue.String, 10, 64); err == nil {
				if prio, _ := l.priorityRepo.FindByID(ctx, prioID); prio != nil {
					details["Приоритет"] = escape(prio.Name)
				}
			}
		case "DELEGATION":
			if execID, err := strconv.ParseUint(item.NewValue.String, 10, 64); err == nil {
				if newExecutor, _ := l.userRepo.FindUserByID(ctx, execID); newExecutor != nil {
					if newExecutor.ID == recipient.ID {
						details["Назначено"] = "Вам"
					} else {
						details["Назначено"] = escape(newExecutor.Fio)
					}
				}
			}
		case "COMMENT":
			if item.Comment.Valid {
				comment = item.Comment.String
			}
		case "DURATION_CHANGE":
			if parsedTime, err := time.Parse(time.RFC3339, item.NewValue.String); err == nil {
				details["Срок"] = escape(parsedTime.Format("02.01.2006 15:04"))
			}
		case "ATTACHMENT_ADD":
			if item.Attachment != nil {
				fileURL := l.serverCfg.BaseURL + "/uploads/" + item.Attachment.FilePath
				attachmentText = fmt.Sprintf("📎 Прикреплен файл: [%s](%s)", escape(item.Attachment.FileName), fileURL)
			}
		}
	}

	if mainAction == "" {
		mainAction = fmt.Sprintf("🔄 %s обновил\\(а\\) заявку №%d\n*%s*", actorName, order.ID, orderName)
	}

	sb.WriteString(mainAction + "\n\n")

	if len(details) > 0 {
		var detailLines []string
		orderOfKeys := []string{"Статус", "Приоритет", "Назначено", "Срок"}
		labelMap := map[string]string{
			"Статус":    "Статус",
			"Приоритет": "Приоритет",
			"Назначено": "Исполнитель",
			"Срок":      "Срок выполнения",
		}

		for _, key := range orderOfKeys {
			if val, ok := details[key]; ok {
				line := labelMap[key] + ": *" + val + "*"
				detailLines = append(detailLines, line)
			}
		}
		sb.WriteString(strings.Join(detailLines, "\n") + "\n\n")
	}

	if attachmentText != "" {
		sb.WriteString(attachmentText + "\n\n")
	}

	if comment != "" {
		sb.WriteString(fmt.Sprintf("`%s`\n\n", escape(comment)))
	}

	sb.WriteString(orderLink)

	return sb.String()
}



func (l *NotificationListener) formatWebSocketPayload(ctx context.Context, events []events.OrderHistoryCreatedEvent, recipient *entities.User) (*websocket.NotificationPayload, error) {
	if len(events) == 0 || recipient == nil {
		return nil, nil
	}

	firstEvent := events[0]
	actor, ok := firstEvent.Actor.(*entities.User)
	if !ok || actor == nil {
		return nil, fmt.Errorf("сущность Actor не была передана в событии")
	}

	order, ok := firstEvent.Order.(*entities.Order)
	if !ok || order == nil {
		return nil, fmt.Errorf("сущность Order не была передана в событии")
	}

	mainMessage := fmt.Sprintf("<strong>%s</strong> обновил(а) заявку <strong>%s №%d</strong>", actor.Fio, order.Name, order.ID)
	if len(events) == 1 && events[0].HistoryItem.EventType == "CREATE" {
		mainMessage = fmt.Sprintf("<strong>%s</strong> создал(а) новую заявку <strong>%s №%d</strong>", actor.Fio, order.Name, order.ID)
	}

	var changes []websocket.ChangeInfo
	var attachmentLink *string

	for _, e := range events {
		item := e.HistoryItem
		switch item.EventType {
		case "STATUS_CHANGE":
			if statusID, err := strconv.ParseUint(item.NewValue.String, 10, 64); err == nil {
				if status, _ := l.statusRepo.FindStatus(ctx, statusID); status != nil {
					changes = append(changes, websocket.ChangeInfo{Type: "STATUS_CHANGE", Text: fmt.Sprintf("Статус: <strong>%s</strong>", status.Name)})
				}
			}
		case "PRIORITY_CHANGE":
			if prioID, err := strconv.ParseUint(item.NewValue.String, 10, 64); err == nil {
				if prio, _ := l.priorityRepo.FindByID(ctx, prioID); prio != nil {
					changes = append(changes, websocket.ChangeInfo{Type: "PRIORITY_CHANGE", Text: fmt.Sprintf("Приоритет: <strong>%s</strong>", prio.Name)})
				}
			}
		case "COMMENT":
			if item.Comment.Valid {
				changes = append(changes, websocket.ChangeInfo{Type: "COMMENT", Text: fmt.Sprintf("Комментарий: \"%s\"", item.Comment.String)})
			}
		case "DELEGATION":
			if execID, err := strconv.ParseUint(item.NewValue.String, 10, 64); err == nil {
				if newExecutor, _ := l.userRepo.FindUserByID(ctx, execID); newExecutor != nil {
					text := fmt.Sprintf("Исполнитель: <strong>%s</strong>", newExecutor.Fio)
					if newExecutor.ID == recipient.ID {
						text = "Заявка назначена на <strong>Вас</strong>"
					}
					changes = append(changes, websocket.ChangeInfo{Type: "DELEGATION", Text: text})
				}
			}
		case "DURATION_CHANGE":
			parsedTime, err := time.Parse(time.RFC3339, item.NewValue.String)
			if err == nil {
				changes = append(changes, websocket.ChangeInfo{Type: "DURATION_CHANGE", Text: fmt.Sprintf("Срок выполнения: <strong>%s</strong>", parsedTime.Format("02.01.2006 15:04"))})
			}
		case "ATTACHMENT_ADD":
			if item.Attachment != nil {
				link := "/uploads/" + item.Attachment.FilePath
				attachmentLink = &link
				changes = append(changes, websocket.ChangeInfo{Type: "ATTACHMENT_ADD", Text: fmt.Sprintf("Прикреплен файл: %s", item.Attachment.FileName)})
			}
		}
	}

	payload := &websocket.NotificationPayload{
		EventID: uuid.New().String(),
		Type:    "ORDER_UPDATED",
		IsRead:  false,
		Actor:   websocket.ActorInfo{Name: actor.Fio, AvatarURL: actor.PhotoURL},
		Message: mainMessage,
		Changes: changes,
		Links: websocket.LinkInfo{
			Primary:    fmt.Sprintf("/orders/%d", order.ID),
			Attachment: attachmentLink,
		},
		CreatedAt: firstEvent.HistoryItem.CreatedAt,
	}

	return payload, nil
}

package listeners

import (
	"context"
	"fmt"
	"sort"
	"strconv" // <<-- 1. ДОБАВЛЕН ЭТОТ ИМПОРТ
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

// ===== СТРУКТУРЫ ДЛЯ ГРУППИРОВКИ УВЕДОМЛЕНИЙ =====
type eventGroupKey struct {
	OrderID uint64
	TxID    string
}

type eventGroup struct {
	events []events.OrderHistoryCreatedEvent
	timer  *time.Timer
	// поле recipients удалено, так как получатели определяются в момент отправки
}

// ===========================================

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

// handleOrderHistoryCreated - обработчик, который собирает события в группы.
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
		// Запускаем таймер, который вызовет отправку через 2 секунды.
		// Передаем контекст, чтобы избежать гонки данных при логировании и запросах.
		group.timer = time.AfterFunc(2*time.Second, func() {
			l.sendGroupedNotification(context.Background(), key)
		})
	}

	group.events = append(group.events, e)
	l.logger.Info("Событие добавлено в группу", zap.Any("key", key), zap.Int("totalInGroup", len(group.events)))

	return nil
}

// sendGroupedNotification - отправляет сгруппированное уведомление.
// <<-- 2. ИСПРАВЛЕНА СИГНАТУРА: добавлен context
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

	// Определяем получателей прямо перед отправкой, когда все события уже собраны.
	recipients, err := l.determineRecipients(ctx, group.events)
	if err != nil {
		l.logger.Error("Не удалось определить получателей для отправки", zap.Error(err), zap.Any("key", key))
		return
	}

	// Для каждого получателя формируем свое, персонализированное сообщение
	for _, user := range recipients {
		message := l.formatGroupedMessage(ctx, group.events, &user)
		if message == "" {
			continue
		}

		if !user.TelegramChatID.Valid || user.TelegramChatID.Int64 == 0 {
			continue
		}

		// Используем тот же 'ctx', который пришел в функцию
		if err := l.notificationService.SendFormattedMessage(ctx, user.TelegramChatID.Int64, message); err != nil {
			l.logger.Error("Не удалось отправить сгруппированное уведомление", zap.Uint64("userID", user.ID), zap.Error(err))
		}
		payload, err := l.formatWebSocketPayload(ctx, group.events, &user)
		if err != nil {
			l.logger.Error("Не удалось сформировать WebSocket payload", zap.Uint64("userID", user.ID), zap.Error(err))
			continue // Пропускаем отправку, если payload не удалось собрать
		}
		if payload != nil {
			// Отправляем с типом "notification", чтобы фронтенд знал, что это
			err := l.wsNotificationService.SendNotification(user.ID, payload, "notification")
			if err != nil {
				l.logger.Error("Не удалось отправить WebSocket-уведомление", zap.Uint64("userID", user.ID), zap.Error(err))
			}
		}
	}
}

// determineRecipients - решает, кому нужно отправить уведомление.
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

	// 1. Добавляем создателя и текущего исполнителя заявки
	userIDs[order.CreatorID] = struct{}{}
	if order.ExecutorID != nil {
		userIDs[*order.ExecutorID] = struct{}{}
	}

	// 2. Добавляем всех, кто когда-либо участвовал в истории заявки
	// (старые исполнители, авторы комментариев и т.д.)
	for _, e := range groupEvents {
		// Добавляем автора события в истории
		if e.HistoryItem.UserID > 0 {
			userIDs[e.HistoryItem.UserID] = struct{}{}
		}
		// Если это было переназначение, добавляем и старого исполнителя
		if e.HistoryItem.EventType == "DELEGATION" && e.HistoryItem.OldValue.Valid {
			oldExecutorID, _ := strconv.ParseUint(e.HistoryItem.OldValue.String, 10, 64)
			userIDs[oldExecutorID] = struct{}{}
		}
	}

	// 4. Удаляем того, кто сам совершил действие, чтобы не спамить ему
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

	sort.Slice(events, func(i, j int) bool { return events[i].HistoryItem.CreatedAt.Before(events[j].HistoryItem.CreatedAt) })

	escape := telegram.EscapeTextForMarkdownV2
	// --- ИСПРАВЛЕНИЕ: Берем первый элемент из слайса, а не весь слайс ---
	firstEvent := events[0]
	actor, _ := firstEvent.Actor.(*entities.User)
	order, _ := firstEvent.Order.(*entities.Order)

	if actor == nil || order == nil {
		return ""
	}

	actorName := escape(actor.Fio)
	orderName := escape(order.Name)
	orderLink := fmt.Sprintf("[Посмотреть мои заявки](%s/order?participant=me)", l.frontendCfg.BaseURL)
	// orderLink := fmt.Sprintf("[Посмотреть заявки](%s/order)", l.frontendCfg.BaseURL)

	var sb strings.Builder
	var mainAction string
	details := make(map[string]string)
	var comment, attachmentText string

	// --- ШАГ 1: Собираем всю информацию из событий (правильная логика для Telegram) ---
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

func (l *NotificationListener) formatSingleMessage(ctx context.Context, e events.OrderHistoryCreatedEvent, recipient *entities.User) string {
	item := e.HistoryItem
	actor, _ := e.Actor.(*entities.User)
	order, _ := e.Order.(*entities.Order)

	if actor == nil || order == nil {
		return ""
	}

	escape := telegram.EscapeTextForMarkdownV2
	orderLink := fmt.Sprintf("\n\n[Посмотреть мои заявки](%s/order?participant=me)", l.frontendCfg.BaseURL)

	switch item.EventType {
	case "CREATE":
		return fmt.Sprintf("✅ *%s* создал\\(а\\) новую заявку *№%d*\n`%s`%s", escape(actor.Fio), order.ID, escape(order.Name), orderLink)
	case "COMMENT":
		if item.Comment.Valid && strings.TrimSpace(item.Comment.String) != "" {
			return fmt.Sprintf("💬 *%s* оставил\\(а\\) комментарий к заявке №%d:\n`%s`%s", escape(actor.Fio), order.ID, escape(item.Comment.String), orderLink)
		}
	case "STATUS_CHANGE":
		if statusID, err := strconv.ParseUint(item.NewValue.String, 10, 64); err == nil {
			newStatus, _ := l.statusRepo.FindStatus(ctx, statusID)
			if newStatus != nil {
				return fmt.Sprintf("📝 *%s* изменил\\(а\\) статус заявки №%d на *%s*%s", escape(actor.Fio), order.ID, escape(newStatus.Name), orderLink)
			}
		}
	case "PRIORITY_CHANGE":
		if prioID, err := strconv.ParseUint(item.NewValue.String, 10, 64); err == nil {
			newsPriority, _ := l.priorityRepo.FindByID(ctx, prioID)
			if newsPriority != nil {
				return fmt.Sprintf("📝 *%s* изменил\\(а\\) приоритет заявки №%d на *%s*%s", escape(actor.Fio), order.ID, escape(newsPriority.Name), orderLink)
			}
		}
	}

	return ""
}

func (l *NotificationListener) formatWebSocketPayload(ctx context.Context, events []events.OrderHistoryCreatedEvent, recipient *entities.User) (*websocket.NotificationPayload, error) {
	if len(events) == 0 || recipient == nil {
		return nil, nil
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].HistoryItem.CreatedAt.Before(events[j].HistoryItem.CreatedAt)
	})

	firstEvent := events[0]
	actor, ok := firstEvent.Actor.(*entities.User)
	if !ok || actor == nil {
		return nil, fmt.Errorf("сущность Actor не была передана в событии")
	}

	order, ok := firstEvent.Order.(*entities.Order)
	if !ok || order == nil {
		return nil, fmt.Errorf("сущность Order не была передана в событии")
	}

	// Формируем основное сообщение
	mainMessage := fmt.Sprintf("<strong>%s</strong> обновил(а) заявку <strong>%s №%d</strong>", actor.Fio, order.Name, order.ID)
	if len(events) == 1 && events[0].HistoryItem.EventType == "CREATE" {
		mainMessage = fmt.Sprintf("<strong>%s</strong> создал(а) новую заявку <strong>%s №%d</strong>", actor.Fio, order.Name, order.ID)
	}

	// Собираем детали изменений
	var changes []websocket.ChangeInfo
	var attachmentLink *string

	// --- НАЧАЛО ГЛАВНЫХ ИЗМЕНЕНИЙ ---
	for _, e := range events {
		item := e.HistoryItem
		switch item.EventType {
		case "STATUS_CHANGE":
			if statusID, err := strconv.ParseUint(item.NewValue.String, 10, 64); err == nil {
				if status, _ := l.statusRepo.FindStatus(ctx, statusID); status != nil {
					changes = append(changes, websocket.ChangeInfo{Type: "STATUS_CHANGE", Text: fmt.Sprintf("Статус: <strong>%s</strong>", status.Name)})
				}
			}
			// <<< ИЗМЕНЕНИЕ: ДОБАВЛЯЕМ PRIORITY_CHANGE >>>
		case "PRIORITY_CHANGE":
			if prioID, err := strconv.ParseUint(item.NewValue.String, 10, 64); err == nil {
				if prio, _ := l.priorityRepo.FindByID(ctx, prioID); prio != nil {
					changes = append(changes, websocket.ChangeInfo{Type: "PRIORITY_CHANGE", Text: fmt.Sprintf("Приоритет: <strong>%s</strong>", prio.Name)})
				}
			}
		case "COMMENT":
			if item.Comment.Valid {
				// <<< ИЗМЕНЕНИЕ: ТЕПЕРЬ МЫ ПЕРЕДАЕМ САМ ТЕКСТ КОММЕНТАРИЯ >>>
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
		// <<< ИЗМЕНЕНИЕ: ДОБАВЛЯЕМ DURATION_CHANGE >>>
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

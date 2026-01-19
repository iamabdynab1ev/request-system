// internal/controllers/telegram/actions.go
package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/pkg/telegram"
	"request-system/pkg/types"
)

func (c *TelegramController) handleSelectOrderAction(ctx context.Context, chatID int64, mid int, orderID uint64) error {
	user, _, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}
	order, err := c.orderService.FindOrderByIDForTelegram(ctx, user.ID, orderID)
	if err != nil {
		_ = c.tgService.AnswerCallbackQuery(ctx, "", "❌ Заявка не найдена")
		return nil
	}
	state := dto.NewTelegramState(orderID, mid)
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	return c.sendEditMenu(ctx, chatID, mid, order)
}

func (c *TelegramController) handleEditStatusStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}
	user, err := c.userService.FindUserByTelegramChatID(ctx, chatID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	order, err := c.orderService.FindOrderByIDForTelegram(ctx, user.ID, state.OrderID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	currentStatus, err := c.statusRepo.FindStatus(ctx, order.StatusID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	allowedStatuses := c.getAllowedStatuses(ctx, currentStatus, order.StatusID)
	if len(allowedStatuses) == 0 {
		_ = c.tgService.AnswerCallbackQuery(ctx, "", "Нет доступных статусов")
		return nil
	}
	state.Mode = "awaiting_new_status"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	var keyboard [][]telegram.InlineKeyboardButton
	currentRow := []telegram.InlineKeyboardButton{}
	for _, status := range allowedStatuses {
		cb := fmt.Sprintf(`{"action":"set_status","status_id":%d}`, status.ID)
		currentRow = append(currentRow, telegram.InlineKeyboardButton{
			Text: status.Name,
			CallbackData: cb,
		})
		if len(currentRow) == 2 {
			keyboard = append(keyboard, currentRow)
			currentRow = []telegram.InlineKeyboardButton{}
		}
	}
	if len(currentRow) > 0 {
		keyboard = append(keyboard, currentRow)
	}
	keyboard = append(keyboard, []telegram.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: fmt.Sprintf(`{"action":"select_order","order_id":%d}`, state.OrderID)},
	})
	return c.tgService.EditMessageText(ctx, chatID, messageID,
		"Выберите новый статус:", telegram.WithKeyboard(keyboard))
}

func (c *TelegramController) handleEditDurationStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}
	state.Mode = "awaiting_duration"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	quickDurations := []struct {
		Label string
		Duration time.Duration
	}{
		{"Через 3 часа", 3 * time.Hour},
		{"Завтра", 24 * time.Hour},
		{"Через 3 дня", 72 * time.Hour},
		{"Через неделю", 7 * 24 * time.Hour},
	}
	var keyboard [][]telegram.InlineKeyboardButton
	row := []telegram.InlineKeyboardButton{}
	now := time.Now().In(c.loc)
	for _, qd := range quickDurations {
		futureTime := now.Add(qd.Duration).Round(30 * time.Minute)
		callbackValue := futureTime.Format("02.01.2006 15:04")
		buttonText := fmt.Sprintf("%s (%s)", qd.Label, futureTime.Format("02.01 15:04"))
		row = append(row, telegram.InlineKeyboardButton{
			Text: buttonText,
			CallbackData: fmt.Sprintf(`{"action":"set_duration","value":"%s"}`, callbackValue),
		})
		if len(row) == 2 {
			keyboard = append(keyboard, row)
			row = []telegram.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		keyboard = append(keyboard, row)
	}
	keyboard = append(keyboard, []telegram.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: fmt.Sprintf(`{"action":"select_order","order_id":%d}`, state.OrderID)},
	})
	text := "Выберите срок или отправьте его текстом в формате `ДД.ММ.ГГГГ ЧЧ:ММ`"
	return c.tgService.EditMessageText(ctx, chatID, messageID, text,
		telegram.WithKeyboard(keyboard), telegram.WithMarkdownV2())
}

func (c *TelegramController) handleSetDuration(ctx context.Context, chatID int64, text string) error {
	if len(text) > 20 {
		return c.tgService.SendMessageEx(ctx, chatID, "❌ Неверный формат даты\\.", telegram.WithMarkdownV2())
	}
	var value interface{}
	var parsedTime time.Time
	var err error
	if strings.ToLower(text) == "clear" {
		value = nil
	} else {
		formats := []string{"2006-01-02 15:04", "02.01.2006 15:04", "02.01.2006"}
		for _, format := range formats {
			parsedTime, err = time.ParseInLocation(format, text, c.loc)
			if err == nil {
				break
			}
		}
		if err != nil {
			return c.tgService.SendMessageEx(ctx, chatID,
				"❌ Неверный формат\\. Используйте `ДД\\.ММ\\.ГГГГ ЧЧ:ММ`\\.",
				telegram.WithMarkdownV2())
		}
		if parsedTime.Before(time.Now()) {
			return c.tgService.SendMessageEx(ctx, chatID,
				"❌ Дата не может быть в прошлом\\.",
				telegram.WithMarkdownV2())
		}
		maxDate := time.Now().AddDate(0, 0, maxDateInFutureDays)
		if parsedTime.After(maxDate) {
			return c.tgService.SendMessageEx(ctx, chatID,
				"❌ Дата слишком далеко в будущем \\(макс\\. 1 год\\)\\.",
				telegram.WithMarkdownV2())
		}
		value = parsedTime
	}
	return c.handleSetSomething(ctx, chatID, "duration", value, "✅ Срок обновлен!")
}

func (c *TelegramController) handleEditCommentStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}
	state.Mode = "awaiting_comment"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	text := "💬 *Введите комментарий:*\n\n_Макс\\. 500 символов_"
	keyboard := [][]telegram.InlineKeyboardButton{
		{{Text: "◀️ Назад", CallbackData: fmt.Sprintf(`{"action":"select_order","order_id":%d}`, state.OrderID)}},
	}
	return c.tgService.EditMessageText(ctx, chatID, messageID, text,
		telegram.WithKeyboard(keyboard), telegram.WithMarkdownV2())
}

func (c *TelegramController) handleSetComment(ctx context.Context, chatID int64, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return c.tgService.SendMessageEx(ctx, chatID, "❌ Комментарий не может быть пустым\\.", telegram.WithMarkdownV2())
	}
	if len(text) > maxCommentLength {
		return c.tgService.SendMessageEx(ctx, chatID,
			fmt.Sprintf("❌ Комментарий слишком длинный \\(макс\\. %d символов\\)\\.", maxCommentLength),
			telegram.WithMarkdownV2())
	}
	return c.handleSetSomething(ctx, chatID, "comment", text, "✅ Комментарий добавлен!")
}
func (c *TelegramController) handleDelegateStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}

	// 1. Получаем ВАС (кто нажимает кнопку), чтобы подготовить контекст
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	// 2. Получаем ЗАЯВКУ, чтобы узнать текущего Исполнителя
	order, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, state.OrderID)
	if err != nil {
		return c.tgService.EditMessageText(ctx, chatID, messageID, "❌ Ошибка: заявка не найдена\\.", telegram.WithMarkdownV2())
	}

	// 3. Логика определения отдела для фильтрации
	var targetDepID *uint64
	var targetBranchID *uint64
	
	// Флаг для заголовка (чей отдел мы показываем)
	listTitle := "👤 *Коллеги в отделе исполнителя:*"

	if order.ExecutorID != nil {
		// Если заявка уже на ком-то висит -> берем ЕГО отдел
		executor, err := c.userRepo.FindUserByID(userCtx, *order.ExecutorID)
		if err == nil {
			targetDepID = executor.DepartmentID
			targetBranchID = executor.BranchID
		}
	} 

	// Если исполнителя нет (или не удалось определить отдел) -> берем ВАШ отдел
	if targetDepID == nil && targetBranchID == nil {
		targetDepID = user.DepartmentID
		targetBranchID = user.BranchID
		listTitle = "👤 *Выберите исполнителя:* " // Если исполнителя не было, просто заголовок
	}

	// 4. Настраиваем фильтр
	filter := types.Filter{Filter: make(map[string]interface{}), WithPagination: false}

	if targetDepID != nil {
		filter.Filter["department_id"] = *targetDepID
	} else if targetBranchID != nil {
		filter.Filter["branch_id"] = *targetBranchID
	} else {
		// Если совсем ничего не определилось (нет отдела) — покажем поиск
		listTitle = "👤 *Поиск сотрудника:*"
	}

	// 5. Загружаем список людей по этому фильтру
	users, _, err := c.userRepo.GetUsers(userCtx, filter)

	text := listTitle
	var keyboard [][]telegram.InlineKeyboardButton

	showSearch := false
	if err != nil || len(users) == 0 {
		showSearch = true
	}

	addedCount := 0
	if !showSearch {
		for _, u := range users {
			// Исключаем:
			// 1. Текущего исполнителя заявки (зачем делегировать ему же?)
			// 2. ВАС самих (зачем делегировать себе через меню "Делегировать"? Для этого есть кнопка "Взять в работу", но если надо, можно убрать это условие)
			
			if order.ExecutorID != nil && u.ID == *order.ExecutorID {
				continue
			}

			if addedCount >= 10 {
				showSearch = true // Слишком много людей, остановимся
				break
			}

			cb := fmt.Sprintf(`{"action":"set_executor","user_id":%d}`, u.ID)
			keyboard = append(keyboard, []telegram.InlineKeyboardButton{
				{Text: u.Fio, CallbackData: cb},
			})
			addedCount++
		}
	}

	if addedCount == 0 {
		text = "Сотрудники в этом отделе не найдены\\.\n\n" +
			"Введите ФИО сотрудника для глобального поиска:"
		state.Mode = "awaiting_executor"
	} else {
		if showSearch {
			// Обязательно экранируем скобки для MarkdownV2!
			text += "\n_\\(показаны не все, используйте поиск, если нужно\\)_"
		}
		state.Mode = "awaiting_executor" // Режим ожидания текста, если захотят найти кого-то другого
	}

	keyboard = append(keyboard, []telegram.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: fmt.Sprintf(`{"action":"select_order","order_id":%d}`, state.OrderID)},
	})

	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	return c.tgService.EditMessageText(ctx, chatID, messageID, text,
		telegram.WithKeyboard(keyboard), telegram.WithMarkdownV2())
}
func (c *TelegramController) handleSetExecutorFromText(ctx context.Context, chatID int64, text string) error {
	// 1. Ограничиваем поиск до 15 человек через Limit
	users, _, err := c.userRepo.GetUsers(ctx, types.Filter{
		Filter: map[string]interface{}{"fio_like": text},
		Limit:  15,
		Page:   1,
	})

	if err != nil || len(users) == 0 {
		return c.tgService.SendMessageEx(ctx, chatID,
			"❌ Сотрудники не найдены\\.\nПопробуйте уточнить запрос\\.",
			telegram.WithMarkdownV2())
	}

	if len(users) > 1 {
		var keyboard [][]telegram.InlineKeyboardButton
		// Показываем максимум 10 кнопок, чтобы не сломать Telegram
		count := 0
		for _, user := range users {
			if count >= 10 {
				break
			}
			cb := fmt.Sprintf(`{"action":"set_executor","user_id":%d}`, user.ID)
			keyboard = append(keyboard, []telegram.InlineKeyboardButton{
				{Text: user.Fio, CallbackData: cb},
			})
			count++
		}
		
		msgText := "Выберите сотрудника:"
		if len(users) > 10 {
			msgText += " _(показаны первые 10)_"
		}

		return c.tgService.SendMessageEx(ctx, chatID, msgText,
			telegram.WithKeyboard(keyboard), telegram.WithMarkdownV2())
	}

	// Если нашелся ровно один сотрудник - назначаем сразу
	return c.handleSetSomething(ctx, chatID, "executor_id", users[0].ID, "✅ Исполнитель назначен!")
}

func (c *TelegramController) handleSetSomething(ctx context.Context, chatID int64, key string, value interface{}, popupText string) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, 0)
	}

	// --- Логика обновления State (StatusID, ExecutorID и т.д.) осталась прежней ---
	switch key {
	case "status_id":
		if id, ok := value.(uint64); ok {
			state.SetStatusID(id)
		} else if idFloat, ok := value.(float64); ok {
			state.SetStatusID(uint64(idFloat))
		} else {
			c.logger.Error("Неверный тип для status_id", zap.Any("value", value))
			return c.sendInternalError(ctx, chatID)
		}
	case "executor_id":
		if id, ok := value.(uint64); ok {
			state.SetExecutorID(id)
		} else if idFloat, ok := value.(float64); ok {
			state.SetExecutorID(uint64(idFloat))
		} else {
			c.logger.Error("Неверный тип для executor_id", zap.Any("value", value))
			return c.sendInternalError(ctx, chatID)
		}
	case "comment":
		if comment, ok := value.(string); ok {
			state.SetComment(comment)
		} else {
			c.logger.Error("Неверный тип для comment", zap.Any("value", value))
			return c.sendInternalError(ctx, chatID)
		}
	case "duration":
		if value == nil {
			state.ClearDuration()
		} else if t, ok := value.(time.Time); ok {
			state.SetDuration(&t)
		} else if tPtr, ok := value.(*time.Time); ok {
			state.SetDuration(tPtr)
		} else {
			c.logger.Error("Неверный тип для duration", zap.Any("value", value))
			return c.sendInternalError(ctx, chatID)
		}
	default:
		c.logger.Error("Неизвестный ключ", zap.String("key", key))
		return c.sendInternalError(ctx, chatID)
	}
	
	// --- СОХРАНЕНИЕ STATE ---
	state.Mode = "editing_order"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	_ = c.tgService.AnswerCallbackQuery(ctx, "", popupText)

	
	
	user, userCtx, err := c.prepareUserContext(ctx, chatID) 
	if err != nil {
		// Если тут ошибка, значит юзера вообще нет
		return c.sendInternalError(ctx, chatID)
	}

	// Используем userCtx вместо ctx
	order, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, state.OrderID)
	if err != nil {
		c.logger.Error("Не удалось получить заявку для обновления меню", 
            zap.Error(err), 
            zap.Uint64("order_id", state.OrderID),
            zap.Int64("user_id", int64(user.ID)))
            
		return c.tgService.EditMessageText(ctx, chatID, state.MessageID,
			"❌ Ошибка: заявка не найдена или нет прав\\.")
	}

	return c.sendEditMenu(ctx, chatID, state.MessageID, order)
}
func (c *TelegramController) handleSaveChanges(ctx context.Context, chatID int64, messageID int) error {
	_, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}
	if !state.HasChanges() {
		_ = c.tgService.AnswerCallbackQuery(ctx, "", "Нет изменений для сохранения")
		return nil
	}
	currentOrder, err := c.orderService.FindOrderByID(ctx, state.OrderID)
	if err != nil {
		c.logger.Error("Не удалось получить заявку", zap.Error(err))
		return c.tgService.EditMessageText(ctx, chatID, messageID,
			"❌ Ошибка при получении данных заявки\\.")
	}
	updateDTO := dto.UpdateOrderDTO{}
	changesMap := make(map[string]interface{})
	
	// Статус
	sid, sidExists, _ := state.GetStatusID()
	if sidExists && currentOrder.StatusID != sid {
		updateDTO.StatusID = &sid
		changesMap["status_id"] = sid
	}
	
// Исполнитель
eid, eidExists, _ := state.GetExecutorID()
if eidExists {
	// Проверяем: если eid == 0, значит пользователь хочет удалить исполнителя
	if eid == 0 {
		changesMap["executor_id"] = nil
		var nullID *uint64
		updateDTO.ExecutorID = nullID
	} else if currentOrder.ExecutorID == nil || *currentOrder.ExecutorID != eid {
		updateDTO.ExecutorID = &eid
		changesMap["executor_id"] = eid
	}
}
	
	// Комментарий
	com, comExists := state.GetComment()
	if comExists && strings.TrimSpace(com) != "" {
		v := com
		updateDTO.Comment = &v
	}
	
	// Срок (Duration)
	dur, _ := state.GetDuration()
	if dur != nil && (currentOrder.Duration == nil || !currentOrder.Duration.Equal(*dur)) {
		updateDTO.Duration = dur
		changesMap["duration"] = dur
	} else {
		_, durExists := state.Changes["duration"]
		if durExists && currentOrder.Duration != nil {
			changesMap["duration"] = nil
			zeroTime := time.Time{}
			updateDTO.Duration = &zeroTime
		}
	} // ← ЭТА СКОБКА БЫЛА ПРОПУЩЕНА!
	
	// Сохранение
	_, err = c.orderService.UpdateOrder(userCtx, state.OrderID, updateDTO, nil, changesMap)
	if err != nil {
		c.logger.Error("Ошибка сохранения",
			zap.Error(err),
			zap.Uint64("order_id", state.OrderID),
			zap.Any("changes", changesMap))
		return c.tgService.EditMessageText(ctx, chatID, messageID,
			"❌ Ошибка при сохранении\\. Попробуйте позже\\.")
	}
	
	// Очистка
	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
	_ = c.tgService.AnswerCallbackQuery(ctx, "", "💾 Сохранено!")
	return c.handleMyTasksCommand(ctx, chatID, messageID)
}
func (c *TelegramController) handleCallbackQuery(ctx context.Context, query *TelegramCallbackQuery) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(query.Data), &data); err != nil {
		c.logger.Error("Неверный формат callback",
			zap.String("data", query.Data),
			zap.Error(err))
		return nil
	}
	action, _ := data["action"].(string)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID
	switch action {
	case "main_menu":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.sendMainMenu(ctx, chatID)
	case "show_my_tasks":
		return c.handleMyTasksCommand(ctx, chatID, msgID)
	case "sel", "select_order":
		var orderID uint64
		if idFloat, ok := data["order_id"].(float64); ok {
			orderID = uint64(idFloat)
		} else if idFloat, ok := data["id"].(float64); ok {
			orderID = uint64(idFloat)
		}
		return c.handleSelectOrderAction(ctx, chatID, msgID, orderID)
	case "edit_cancel":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleMyTasksCommand(ctx, chatID, msgID)
	case "edit_save":
		return c.handleSaveChanges(ctx, chatID, msgID)
	case "edit_status_start":
		return c.handleEditStatusStart(ctx, chatID, msgID)
	case "edit_duration_start":
		return c.handleEditDurationStart(ctx, chatID, msgID)
	case "edit_comment_start":
		return c.handleEditCommentStart(ctx, chatID, msgID)
	case "edit_delegate_start":
		return c.handleDelegateStart(ctx, chatID, msgID)
	case "set_status":
		if id, ok := data["status_id"].(float64); ok {
			return c.handleSetSomething(ctx, chatID, "status_id", uint64(id), "✅ Статус!")
		}
	case "set_duration":
		if val, ok := data["value"].(string); ok {
			return c.handleSetDuration(ctx, chatID, val)
		}
	case "set_executor":
		if id, ok := data["user_id"].(float64); ok {
			return c.handleSetSomething(ctx, chatID, "executor_id", uint64(id), "✅ Назначен!")
		}
	default:
		c.logger.Warn("Неизвестный action", zap.String("action", action))
	}
	return nil
}

// ==================== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ====================
func (c *TelegramController) sendEditMenu(ctx context.Context, chatID int64, messageID int, order *entities.Order) error {
	status, err := c.statusRepo.FindStatus(ctx, order.StatusID)
	if err != nil {
		c.logger.Error("Не удалось получить статус", zap.Error(err))
		return c.sendInternalError(ctx, chatID)
	}
	creator, _ := c.userRepo.FindUserByID(ctx, order.CreatorID)
	var executor *entities.User
	if order.ExecutorID != nil {
		executor, _ = c.userRepo.FindUserByID(ctx, *order.ExecutorID)
	}
	// Получаем последний комментарий
	lastComment := ""
	historyItems, err := c.orderHistoryRepo.GetOrderHistory(ctx, order.ID,
		types.Filter{Limit: maxHistoryItems, Page: 1})
	if err == nil && len(historyItems) > 0 {
		for _, item := range historyItems {
			if item.EventType == "COMMENT" && item.Comment.Valid && item.Comment.String != "" {
				lastComment = item.Comment.String
				break
			}
		}
	}
	var text strings.Builder
	text.WriteString(fmt.Sprintf("📋 *Заявка №%d*\n━━━━━━━━━━━━━━━━━━━━\n\n", order.ID))
	text.WriteString(fmt.Sprintf("📝 *Описание:*\n%s\n\n",
		telegram.EscapeTextForMarkdownV2(order.Name)))
	statusEmoji := getStatusEmoji(status)
	text.WriteString(fmt.Sprintf("%s *Статус:* %s\n", statusEmoji,
		telegram.EscapeTextForMarkdownV2(status.Name)))
	if creator != nil {
		text.WriteString(fmt.Sprintf("👤 *Создатель:* %s\n",
			telegram.EscapeTextForMarkdownV2(creator.Fio)))
	}
	if executor != nil {
		text.WriteString(fmt.Sprintf("👨‍💼 *Исполнитель:* %s\n",
			telegram.EscapeTextForMarkdownV2(executor.Fio)))
	} else {
		text.WriteString("👨‍💼 *Исполнитель:* _не назначен_\n")
	}
	if order.Duration != nil {
		durationStr := order.Duration.Format("02.01.2006 15:04")
		if order.Duration.Before(time.Now()) {
			text.WriteString(fmt.Sprintf("⏰ *Срок:* ~%s~ ⚠️ _просрочено_\n",
				telegram.EscapeTextForMarkdownV2(durationStr)))
		} else {
			text.WriteString(fmt.Sprintf("⏰ *Срок:* %s\n",
				telegram.EscapeTextForMarkdownV2(durationStr)))
		}
	} else {
		text.WriteString("⏰ *Срок:* _не задан_\n")
	}
	if order.Address != nil && *order.Address != "" {
		text.WriteString(fmt.Sprintf("📍 *Адрес:* %s\n",
			telegram.EscapeTextForMarkdownV2(*order.Address)))
	}
	createdAt := order.CreatedAt.Format("02.01.2006 15:04")
	text.WriteString(fmt.Sprintf("📅 *Создана:* %s\n",
		telegram.EscapeTextForMarkdownV2(createdAt)))
	if lastComment != "" {
		if len(lastComment) > 100 {
			lastComment = lastComment[:100] + "..."
		}
		text.WriteString(fmt.Sprintf("\n💬 *Последний комментарий:*\n_%s_\n",
			telegram.EscapeTextForMarkdownV2(lastComment)))
	}
	text.WriteString("\n━━━━━━━━━━━━━━━━━━━━\n")
	var keyboard [][]telegram.InlineKeyboardButton
	if status.Code != nil && *status.Code == "CLOSED" {
		text.WriteString("\n🔒 *Заявка закрыта\\.*\n_Редактирование недоступно\\._")
		keyboard = append(keyboard, []telegram.InlineKeyboardButton{
			{Text: "◀️ К списку", CallbackData: `{"action":"edit_cancel"}`},
		})
	} else {
		text.WriteString("\n_Выберите действие:_")
		keyboard = [][]telegram.InlineKeyboardButton{
			{{Text: "🔄 Статус", CallbackData: `{"action":"edit_status_start"}`},
			 {Text: "⏰ Срок", CallbackData: `{"action":"edit_duration_start"}`}},
			{{Text: "💬 Комментарий", CallbackData: `{"action":"edit_comment_start"}`},
			 {Text: "👤 Делегировать", CallbackData: `{"action":"edit_delegate_start"}`}},
			{{Text: "✅ Сохранить", CallbackData: `{"action":"edit_save"}`},
			 {Text: "◀️ Назад", CallbackData: `{"action":"edit_cancel"}`}},
		}
	}
	return c.tgService.EditMessageText(ctx, chatID, messageID, text.String(),
		telegram.WithKeyboard(keyboard), telegram.WithMarkdownV2())
}

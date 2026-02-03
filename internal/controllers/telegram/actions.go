package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/pkg/telegram"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

func (c *TelegramController) handleSelectOrderAction(ctx context.Context, chatID int64, mid int, orderID uint64) error {
	// Подготовка контекста (для проверки прав чтения)
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}
	
	order, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, orderID)
	if err != nil {
		_ = c.tgService.AnswerCallbackQuery(ctx, "", "❌ Заявка не найдена или нет доступа")
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
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	order, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, state.OrderID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	
	// Получаем текущий статус и список доступных
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
			Text:         status.Name,
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
		Label    string
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
			Text:         buttonText,
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

// ======================= DELEGATION LOGIC =======================
func (c *TelegramController) handleDelegateStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}

	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	// Нужна только чтобы исключить текущего исполнителя
	order, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, state.OrderID)
	if err != nil {
		return c.tgService.EditMessageText(ctx, chatID, messageID, "❌ Ошибка: заявка не найдена\\.", telegram.WithMarkdownV2())
	}

	filter := types.Filter{Filter: make(map[string]interface{}), WithPagination: false}
	listTitle := ""

	if user.OtdelID != nil {
		filter.Filter["otdel_id"] = *user.OtdelID
		listTitle = "👤 *Коллеги вашего отдела:*"
	} else if user.DepartmentID != nil {
		filter.Filter["department_id"] = *user.DepartmentID
		listTitle = "👤 *Коллеги вашего департамента:*"
	} else if user.OfficeID != nil {
		filter.Filter["office_id"] = *user.OfficeID
		listTitle = "👤 *Сотрудники вашего офиса:*"
	} else if user.BranchID != nil {
		filter.Filter["branch_id"] = *user.BranchID
		listTitle = "👤 *Сотрудники вашего филиала:*"
	} else {
		listTitle = "👤 *Все сотрудники (Вы не привязаны к отделу):*"
	}

	users, _, err := c.userRepo.GetUsers(userCtx, filter)

	text := listTitle
	var keyboard [][]telegram.InlineKeyboardButton

	showSearch := false
	if err != nil || len(users) == 0 {
		showSearch = true
	}

	addedCount := 0
if !showSearch {
	maxButtons := 8
	
	for _, u := range users {
		if u.ID == user.ID {
			continue // Самого себя не предлагать
		}
		if order.ExecutorID != nil && u.ID == *order.ExecutorID {
			continue // Текущего исполнителя не предлагать
		}

		if addedCount >= maxButtons {  
			showSearch = true
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
		text = "В вашем подразделении больше никого нет\\.\n\n" +
			"Введите ФИО сотрудника для глобального поиска:"
		state.Mode = "awaiting_executor"
	} else {
		if showSearch {
			text += "\n_\\(показаны не все, используйте поиск\\)_"
		}
		state.Mode = "awaiting_executor"
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
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	filterMap := map[string]interface{}{
		"fio_like": text,
	}

	if user.OtdelID != nil {
		filterMap["otdel_id"] = *user.OtdelID
	} else if user.DepartmentID != nil {
		filterMap["department_id"] = *user.DepartmentID
	} else if user.OfficeID != nil {
		filterMap["office_id"] = *user.OfficeID
	} else if user.BranchID != nil {
		filterMap["branch_id"] = *user.BranchID
	}

	users, _, err := c.userRepo.GetUsers(userCtx, types.Filter{
		Filter: filterMap,
		Limit:  10,
		Page:   1,
	})

	if err != nil || len(users) == 0 {
		return c.tgService.SendMessageEx(ctx, chatID,
			"❌ Сотрудники в *вашем подразделении* не найдены\\.\nПопробуйте уточнить запрос\\.",
			telegram.WithMarkdownV2())
	}

	if len(users) > 1 {
		var keyboard [][]telegram.InlineKeyboardButton
		for _, u := range users {
			cb := fmt.Sprintf(`{"action":"set_executor","user_id":%d}`, u.ID)
			keyboard = append(keyboard, []telegram.InlineKeyboardButton{
				{Text: u.Fio, CallbackData: cb},
			})
		}
		msgText := fmt.Sprintf("Найдено %d сотрудников:", len(users))
		return c.tgService.SendMessageEx(ctx, chatID, msgText, telegram.WithKeyboard(keyboard))
	}

	return c.handleSetSomething(ctx, chatID, "executor_id", users[0].ID, "✅ Исполнитель назначен!")
}

// ==================== STATE AND SAVING ====================

func (c *TelegramController) handleSetSomething(ctx context.Context, chatID int64, key string, value interface{}, popupText string) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, 0)
	}

	switch key {
	case "status_id":
		if id, ok := value.(uint64); ok {
			state.SetStatusID(id)
		} else if idFloat, ok := value.(float64); ok {
			state.SetStatusID(uint64(idFloat))
		} else {
			return c.sendInternalError(ctx, chatID)
		}
	case "executor_id":
		if id, ok := value.(uint64); ok {
			state.SetExecutorID(id)
		} else if idFloat, ok := value.(float64); ok {
			state.SetExecutorID(uint64(idFloat))
		} else {
			return c.sendInternalError(ctx, chatID)
		}
	case "comment":
		if comment, ok := value.(string); ok {
			state.SetComment(comment)
		}
	case "duration":
		if value == nil {
			state.ClearDuration()
		} else if t, ok := value.(time.Time); ok {
			state.SetDuration(&t)
		} else if tPtr, ok := value.(*time.Time); ok {
			state.SetDuration(tPtr)
		}
	}

	state.Mode = "editing_order"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	
	_ = c.tgService.AnswerCallbackQuery(ctx, "", popupText)
	
	// Используем корректный контекст для отрисовки меню
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	
	order, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, state.OrderID)
	if err != nil {
		return c.tgService.EditMessageText(ctx, chatID, state.MessageID,
			"❌ Ошибка: заявка не найдена\\.")
	}
	return c.sendEditMenu(ctx, chatID, state.MessageID, order)
}

func (c *TelegramController) handleSaveChanges(ctx context.Context, chatID int64, messageID int) error {
	// ✅ ИСПРАВЛЕНИЕ 1: Сохраняем user (был "_")
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		c.logger.Error("Ошибка получения контекста пользователя при сохранении",
			zap.Error(err),
			zap.Int64("chat_id", chatID))
		return c.sendInternalError(ctx, chatID)
	}

	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}
	
	// ✅ ЗАЩИТА: Проверяем валидность state
	if state.OrderID == 0 {
		c.logger.Error("State с пустым OrderID",
			zap.Int64("chat_id", chatID),
			zap.Uint64("user_id", user.ID))
		return c.sendStaleStateError(ctx, chatID, messageID)
	}
	
	if !state.HasChanges() {
		_ = c.tgService.AnswerCallbackQuery(ctx, "", "Нет изменений для сохранения")
		return nil
	}

	// ✅ ИСПРАВЛЕНИЕ 2: Передаем user.ID вместо 0
	currentOrder, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, state.OrderID)
	if err != nil {
		c.logger.Error("Не удалось получить заявку для сохранения",
			zap.Error(err),
			zap.Uint64("order_id", state.OrderID),
			zap.Uint64("user_id", user.ID))
		return c.tgService.EditMessageText(ctx, chatID, messageID,
			"❌ Ошибка при получении данных заявки\\.", telegram.WithMarkdownV2())
	}

	// 🔥 ПРОВЕРКА ОБЯЗАТЕЛЬНОГО КОММЕНТАРИЯ ДЛЯ ТЕЛЕГРАМА
	orderTypeCode, _ := c.orderTypeRepo.FindCodeByID(ctx, *currentOrder.OrderTypeID)
	if orderTypeCode != "EQUIPMENT" {
		comment, exists := state.GetComment()
		if !exists || strings.TrimSpace(comment) == "" {
			_ = c.tgService.AnswerCallbackQuery(ctx, "", "⚠️ Ошибка: Комментарий ОБЯЗАТЕЛЕН!")
			return nil 
		}
	}

	updateDTO := dto.UpdateOrderDTO{}
	changesMap := make(map[string]interface{})

	// Заполняем DTO и Map из State
	sid, sidExists, _ := state.GetStatusID()
	if sidExists && currentOrder.StatusID != sid {
		updateDTO.StatusID = &sid
		changesMap["status_id"] = sid
	}

	eid, eidExists, _ := state.GetExecutorID()
	if eidExists {
		if eid == 0 {
			// Сброс исполнителя (если это поддерживается)
			changesMap["executor_id"] = nil
			updateDTO.ExecutorID = nil
		} else if currentOrder.ExecutorID == nil || *currentOrder.ExecutorID != eid {
			updateDTO.ExecutorID = &eid
			changesMap["executor_id"] = eid
		}
	}

	com, comExists := state.GetComment()
	if comExists && strings.TrimSpace(com) != "" {
		v := com
		updateDTO.Comment = &v
		// changesMap для комментария не обязателен, т.к. коммент идет отдельно в history
	}

	dur, _ := state.GetDuration()
	if dur != nil {
		if currentOrder.Duration == nil || !currentOrder.Duration.Equal(*dur) {
			updateDTO.Duration = dur
			changesMap["duration"] = dur
		}
	} else {
		// Если в state длительность явно nil (сброс), проверяем Changes map
		if _, chExists := state.Changes["duration"]; chExists && currentOrder.Duration != nil {
			changesMap["duration"] = nil
		}
	}

	// ✅ ИСПРАВЛЕНИЕ 3: Улучшенная обработка ошибок
	_, err = c.orderService.UpdateOrder(userCtx, state.OrderID, updateDTO, nil, changesMap)
	
	if err != nil {
		c.logger.Error("Ошибка сохранения через Телеграм",
			zap.Error(err),
			zap.Uint64("order_id", state.OrderID),
			zap.Uint64("user_id", user.ID),
			zap.Any("updateDTO", updateDTO),
			zap.Any("changesMap", changesMap))
		
		// ✅ УЛУЧШЕНИЕ: Более информативные сообщения об ошибках
		errorMsg := "❌ Ошибка при сохранении\\.\n\n"
		errStr := err.Error()
		
		if strings.Contains(errStr, "Forbidden") || strings.Contains(errStr, "прав") {
			errorMsg += "_Недостаточно прав для этой операции\\._"
		} else if strings.Contains(errStr, "закрыта") || strings.Contains(errStr, "CLOSED") {
			errorMsg += "_Заявка закрыта\\. Редактирование запрещено\\._"
		} else if strings.Contains(errStr, "комментарий") {
			errorMsg += "_Необходимо добавить комментарий с описанием\\._"
		} else if strings.Contains(errStr, "no changes") || strings.Contains(errStr, "Нет изменений") {
			errorMsg += "_Нет изменений для сохранения\\._"
		} else {
			errorMsg += "_Попробуйте позже или обратитесь в поддержку\\._"
		}
		
		return c.tgService.EditMessageText(ctx, chatID, messageID, errorMsg, telegram.WithMarkdownV2())
	}

	// ✅ Очистка состояния и уведомление
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

// ==================== sendEditMenu: ИСПРАВЛЕННЫЙ ВЫВОД КНОПОК ====================
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

	// 1. Получаем права
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	perms, _ := utils.GetPermissionsMapFromCtx(userCtx)

	// Создаем контекст авторизации для ЭТОЙ заявки
	isCreator := (order.CreatorID == user.ID)
	isExecutor := (order.ExecutorID != nil && *order.ExecutorID == user.ID)

	authCtx := authz.Context{
		Actor:         user,
		Permissions:   perms,
		Target:        order,
		IsParticipant: isCreator || isExecutor,
	}

	// --- Текст сообщения ---
	var text strings.Builder
	text.WriteString(fmt.Sprintf("📋 *Заявка №%d*\n━━━━━━━━━━━━━━━━━━━━\n\n", order.ID))
	text.WriteString(fmt.Sprintf("📝 *Описание:*\n%s\n\n", telegram.EscapeTextForMarkdownV2(order.Name)))
	
	statusEmoji := getStatusEmoji(status)
	text.WriteString(fmt.Sprintf("%s *Статус:* %s\n", statusEmoji, telegram.EscapeTextForMarkdownV2(status.Name)))

	if creator != nil {
		text.WriteString(fmt.Sprintf("👤 *Создатель:* %s\n", telegram.EscapeTextForMarkdownV2(creator.Fio)))
	}
	if executor != nil {
		text.WriteString(fmt.Sprintf("👨‍💼 *Исполнитель:* %s\n", telegram.EscapeTextForMarkdownV2(executor.Fio)))
	} else {
		text.WriteString("👨‍💼 *Исполнитель:* _не назначен_\n")
	}

	if order.Duration != nil {
		durationStr := order.Duration.Format("02.01.2006 15:04")
		if order.Duration.Before(time.Now()) {
			text.WriteString(fmt.Sprintf("⏰ *Срок:* ~%s~ ⚠️ _просрочено_\n", telegram.EscapeTextForMarkdownV2(durationStr)))
		} else {
			text.WriteString(fmt.Sprintf("⏰ *Срок:* %s\n", telegram.EscapeTextForMarkdownV2(durationStr)))
		}
	} else {
		text.WriteString("⏰ *Срок:* _не задан_\n")
	}
	text.WriteString("\n━━━━━━━━━━━━━━━━━━━━\n")

	// --- КНОПКИ (Строгая проверка привилегий) ---
	var keyboard [][]telegram.InlineKeyboardButton
	
isClosed := false
	if status.Code != nil && *status.Code == "CLOSED" { 
		isClosed = true
	}

	if isClosed {
		text.WriteString("\n🔒 *Заявка закрыта\\.*\n_Редактирование недоступно\\._")
		keyboard = append(keyboard, []telegram.InlineKeyboardButton{
			{Text: "◀️ К списку", CallbackData: `{"action":"edit_cancel"}`},
		})
	} else {
		text.WriteString("\n_Выберите действие:_")

		// ПРОВЕРКИ НА ОСНОВЕ СПИСКА ПРИВИЛЕГИЙ
		
		// 1. Статус (order:update:status_id)
		canStatus := authz.CanDo(authz.OrdersUpdateStatusID, authCtx)
		
		// 2. Срок (order:update:duration)
		canDuration := authz.CanDo(authz.OrdersUpdateDuration, authCtx)
		
		// 3. Комментарий (order:update:comment или order:view - обычно комментарии разрешены всем, кто видит)
		// Используем 'order:update:comment', чтобы быть последовательными со списком
		canComment := authz.CanDo(authz.OrdersUpdateComment, authCtx)

		// 4. Делегирование (order:update:executor_id)
		canDelegate := authz.CanDo(authz.OrdersUpdateExecutorID, authCtx)

		// === Формирование клавиатуры ===
		
		// Ряд 1: Статус и Срок
		row1 := []telegram.InlineKeyboardButton{}
		if canStatus {
			row1 = append(row1, telegram.InlineKeyboardButton{Text: "🔄 Статус", CallbackData: `{"action":"edit_status_start"}`})
		}
		if canDuration {
			row1 = append(row1, telegram.InlineKeyboardButton{Text: "⏰ Срок", CallbackData: `{"action":"edit_duration_start"}`})
		}
		if len(row1) > 0 {
			keyboard = append(keyboard, row1)
		}

		// Ряд 2: Комментарий и Делегирование
		row2 := []telegram.InlineKeyboardButton{}
		if canComment {
			row2 = append(row2, telegram.InlineKeyboardButton{Text: "💬 Коммент", CallbackData: `{"action":"edit_comment_start"}`})
		}
		if canDelegate {
			row2 = append(row2, telegram.InlineKeyboardButton{Text: "👤 Делегировать", CallbackData: `{"action":"edit_delegate_start"}`})
		}
		if len(row2) > 0 {
			keyboard = append(keyboard, row2)
		}

		keyboard = append(keyboard, []telegram.InlineKeyboardButton{
			{Text: "✅ Сохранить", CallbackData: `{"action":"edit_save"}`},
			{Text: "◀️ Назад", CallbackData: `{"action":"edit_cancel"}`},
		})
	}

	return c.tgService.EditMessageText(ctx, chatID, messageID, text.String(),
		telegram.WithKeyboard(keyboard), telegram.WithMarkdownV2())
}

package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"request-system/internal/dto"
	tgapi "request-system/pkg/telegram"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

func (c *TelegramController) ensureStateMessage(ctx context.Context, chatID int64, messageID int) (*dto.TelegramState, error) {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return nil, err
	}

	if messageID > 0 && state.MessageID != messageID {
		state.MessageID = messageID
		if err := c.setUserState(ctx, chatID, state); err != nil {
			return nil, err
		}
	}

	return state, nil
}

func (c *TelegramController) handleEditStatusStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.ensureStateMessage(ctx, chatID, messageID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}

	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return c.handlePrepareUserContextError(ctx, chatID, err)
	}

	order, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, state.OrderID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	currentStatus, err := c.statusRepo.FindStatus(ctx, order.StatusID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	allowedStatuses := c.getAllowedStatuses(ctx, currentStatus, order.StatusID)
	if len(allowedStatuses) == 0 {
		_ = c.answerCallback(ctx, "Нет доступных статусов")
		return nil
	}

	state.Mode = "awaiting_new_status"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	var keyboard [][]tgapi.InlineKeyboardButton
	currentRow := []tgapi.InlineKeyboardButton{}
	for _, status := range allowedStatuses {
		cb := fmt.Sprintf(`{"action":"set_status","status_id":%d}`, status.ID)
		currentRow = append(currentRow, tgapi.InlineKeyboardButton{Text: status.Name, CallbackData: cb})
		if len(currentRow) == 2 {
			keyboard = append(keyboard, currentRow)
			currentRow = []tgapi.InlineKeyboardButton{}
		}
	}
	if len(currentRow) > 0 {
		keyboard = append(keyboard, currentRow)
	}
	keyboard = append(keyboard, c.orderBackKeyboard(state.OrderID)...)

	return c.renderStateScreen(ctx, chatID, state, "Выберите новый статус:", tgapi.WithKeyboard(keyboard))
}

func (c *TelegramController) handleEditDurationStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.ensureStateMessage(ctx, chatID, messageID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}

	state.Mode = "awaiting_duration"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	return c.renderDurationPrompt(ctx, chatID, state, "")
}

func (c *TelegramController) handleSetDuration(ctx context.Context, chatID int64, text string) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, 0)
	}

	text = strings.TrimSpace(text)
	if len(text) > 20 {
		return c.renderDurationPrompt(ctx, chatID, state, "❌ Неверный формат даты\\.")
	}

	var value interface{}
	var parsedTime time.Time

	if strings.EqualFold(text, "clear") {
		value = nil
	} else {
		formats := []string{"2006-01-02 15:04", "02.01.2006 15:04", "02.01.2006"}
		var parseErr error
		for _, format := range formats {
			parsedTime, parseErr = time.ParseInLocation(format, text, c.loc)
			if parseErr == nil {
				break
			}
		}
		if parseErr != nil {
			return c.renderDurationPrompt(
				ctx,
				chatID,
				state,
				"❌ Неверный формат даты\\. Используйте `ДД\\.ММ\\.ГГГГ ЧЧ:ММ`\\.",
			)
		}

		now := time.Now().In(c.loc)
		if parsedTime.Before(now) {
			return c.renderDurationPrompt(ctx, chatID, state, "❌ Дата не может быть в прошлом\\.")
		}

		maxDate := now.AddDate(0, 0, maxDateInFutureDays)
		if parsedTime.After(maxDate) {
			return c.renderDurationPrompt(ctx, chatID, state, "❌ Дата слишком далеко в будущем \\(макс\\. 1 год\\)\\.")
		}
		value = parsedTime
	}

	return c.handleSetSomething(ctx, chatID, "duration", value, "Срок обновлён")
}

func (c *TelegramController) handleEditCommentStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.ensureStateMessage(ctx, chatID, messageID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}

	state.Mode = "awaiting_comment"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	return c.renderCommentPrompt(ctx, chatID, state, "")
}

func (c *TelegramController) handleSetComment(ctx context.Context, chatID int64, text string) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, 0)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return c.renderCommentPrompt(ctx, chatID, state, "❌ Комментарий не может быть пустым\\.")
	}
	if len(text) > maxCommentLength {
		return c.renderCommentPrompt(
			ctx,
			chatID,
			state,
			fmt.Sprintf("❌ Комментарий слишком длинный \\(макс\\. %d символов\\)\\.", maxCommentLength),
		)
	}

	return c.handleSetSomething(ctx, chatID, "comment", text, "Комментарий добавлен")
}

func (c *TelegramController) handleDelegateStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.ensureStateMessage(ctx, chatID, messageID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}

	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return c.handlePrepareUserContextError(ctx, chatID, err)
	}

	order, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, state.OrderID)
	if err != nil {
		return c.renderStateScreen(
			ctx,
			chatID,
			state,
			"❌ Ошибка: заявка не найдена\\.",
			tgapi.WithKeyboard(c.orderBackKeyboard(state.OrderID)),
			tgapi.WithMarkdownV2(),
		)
	}

	const maxDelegateCandidates = 9
	filter := types.Filter{
		Filter:         make(map[string]interface{}),
		Limit:          maxDelegateCandidates,
		Page:           1,
		Offset:         0,
		WithPagination: true,
	}
	text := ""
	switch {
	case user.OtdelID != nil:
		filter.Filter["otdel_id"] = *user.OtdelID
		text = "👤 *Коллеги вашего отдела:*"
	case user.DepartmentID != nil:
		filter.Filter["department_id"] = *user.DepartmentID
		text = "👤 *Коллеги вашего департамента:*"
	case user.OfficeID != nil:
		filter.Filter["office_id"] = *user.OfficeID
		text = "👤 *Сотрудники вашего офиса:*"
	case user.BranchID != nil:
		filter.Filter["branch_id"] = *user.BranchID
		text = "👤 *Сотрудники вашего филиала:*"
	default:
		text = "👤 *Все сотрудники:*"
	}

	users, totalUsers, err := c.userRepo.GetUsers(userCtx, filter)
	showSearch := err != nil || totalUsers == 0

	var rows [][]tgapi.InlineKeyboardButton
	addedCount := 0
	if !showSearch {
		const maxButtons = 8
		for _, candidate := range users {
			if candidate.ID == user.ID {
				continue
			}
			if order.ExecutorID != nil && candidate.ID == *order.ExecutorID {
				continue
			}
			if addedCount >= maxButtons {
				showSearch = true
				break
			}

			cb := fmt.Sprintf(`{"action":"set_executor","user_id":%d}`, candidate.ID)
			rows = append(rows, []tgapi.InlineKeyboardButton{{Text: candidate.Fio, CallbackData: cb}})
			addedCount++
		}
		if totalUsers > uint64(addedCount) {
			showSearch = true
		}
	}

	state.Mode = "awaiting_executor"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	if addedCount == 0 {
		return c.renderExecutorSelection(ctx, chatID, state,
			"В вашем подразделении больше никого нет\\.\n\nВведите ФИО сотрудника для поиска:",
			nil,
		)
	}

	if showSearch {
		text += "\n_Показаны не все, можно уточнить сотрудника текстом_"
	}

	return c.renderExecutorSelection(ctx, chatID, state, text, rows)
}

func (c *TelegramController) handleSetExecutorFromText(ctx context.Context, chatID int64, text string) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, 0)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return c.renderExecutorSelection(ctx, chatID, state, "❌ Введите ФИО сотрудника для поиска\\.", nil)
	}

	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return c.handlePrepareUserContextError(ctx, chatID, err)
	}

	filterMap := map[string]interface{}{}
	switch {
	case user.OtdelID != nil:
		filterMap["otdel_id"] = *user.OtdelID
	case user.DepartmentID != nil:
		filterMap["department_id"] = *user.DepartmentID
	case user.OfficeID != nil:
		filterMap["office_id"] = *user.OfficeID
	case user.BranchID != nil:
		filterMap["branch_id"] = *user.BranchID
	}

	users, _, err := c.userRepo.GetUsers(userCtx, types.Filter{
		Search:         text,
		Filter:         filterMap,
		Limit:          10,
		Page:           1,
		Offset:         0,
		WithPagination: true,
	})
	if err != nil || len(users) == 0 {
		return c.renderExecutorSelection(
			ctx,
			chatID,
			state,
			"❌ Сотрудники по вашему запросу не найдены\\. Попробуйте уточнить поиск\\.",
			nil,
		)
	}

	if len(users) > 1 {
		var rows [][]tgapi.InlineKeyboardButton
		for _, candidate := range users {
			cb := fmt.Sprintf(`{"action":"set_executor","user_id":%d}`, candidate.ID)
			rows = append(rows, []tgapi.InlineKeyboardButton{{Text: candidate.Fio, CallbackData: cb}})
		}
		return c.renderExecutorSelection(
			ctx,
			chatID,
			state,
			fmt.Sprintf("Найдено %d сотрудников\\. Выберите нужного:", len(users)),
			rows,
		)
	}

	return c.handleSetSomething(ctx, chatID, "executor_id", users[0].ID, "Исполнитель назначен")
}

func (c *TelegramController) handleSetSomething(ctx context.Context, chatID int64, key string, value interface{}, popupText string) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, 0)
	}

	switch key {
	case "status_id":
		switch v := value.(type) {
		case uint64:
			state.SetStatusID(v)
		case float64:
			state.SetStatusID(uint64(v))
		default:
			return c.sendInternalError(ctx, chatID)
		}
	case "executor_id":
		switch v := value.(type) {
		case uint64:
			state.SetExecutorID(v)
		case float64:
			state.SetExecutorID(uint64(v))
		default:
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

	_ = c.answerCallback(ctx, popupText)

	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return c.handlePrepareUserContextError(ctx, chatID, err)
	}

	order, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, state.OrderID)
	if err != nil {
		return c.renderStateScreen(
			ctx,
			chatID,
			state,
			"❌ Ошибка: заявка не найдена\\.",
			tgapi.WithKeyboard(c.orderBackKeyboard(state.OrderID)),
			tgapi.WithMarkdownV2(),
		)
	}

	return c.showEditMenuForState(ctx, chatID, state, order)
}

func (c *TelegramController) handleSearchStart(ctx context.Context, chatID int64, messageID int) error {
	state := &dto.TelegramState{Mode: "awaiting_search", MessageID: messageID, Source: "search", Page: 1, Changes: make(map[string]string)}
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	if err := c.renderSearchPrompt(ctx, chatID, messageID, ""); err != nil {
		return err
	}

	return c.syncStateMessageID(ctx, chatID, state)
}

func (c *TelegramController) handleSearchQuery(ctx context.Context, chatID int64, text string) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, 0)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return c.renderSearchPrompt(ctx, chatID, state.MessageID, "❌ Поисковый запрос не может быть пустым\\.")
	}
	if len(text) > maxSearchQueryLength {
		return c.renderSearchPrompt(ctx, chatID, state.MessageID, "❌ Запрос слишком длинный \\(макс\\. 100 символов\\)\\.")
	}

	return c.renderSearchResults(ctx, chatID, state.MessageID, text, true, 1)
}

func (c *TelegramController) renderSearchResults(ctx context.Context, chatID int64, messageID int, query string, allowDirectExact bool, page int) error {
	query = strings.TrimSpace(query)
	if page < 1 {
		page = 1
	}
	_, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	if allowDirectExact {
		var orderID uint64
		if _, err := fmt.Sscanf(query, "%d", &orderID); err == nil {
			userID, _ := utils.GetUserIDFromCtx(userCtx)
			order, exactErr := c.orderService.FindOrderByIDForTelegram(userCtx, userID, orderID)
			if exactErr == nil {
				orderState := dto.NewTelegramState(orderID, messageID, "search", query, page)
				if err := c.setUserState(ctx, chatID, orderState); err != nil {
					return c.sendInternalError(ctx, chatID)
				}
				return c.showEditMenuForState(ctx, chatID, orderState, order)
			}
		}
	}

	filter := c.newTelegramOrderFilter("search", page)
	filter.Search = query
	resp, err := c.orderService.GetOrders(userCtx, filter, false, false, false)
	if err != nil {
		c.logger.Error("Telegram search failed", zap.Error(err), zap.Int64("chat_id", chatID))
		return c.renderSearchPrompt(ctx, chatID, messageID, "❌ Ошибка поиска\\.")
	}

	if len(resp.List) == 0 {
		return c.renderSearchPrompt(
			ctx,
			chatID,
			messageID,
			fmt.Sprintf("❌ По запросу \"%s\" ничего не найдено\\.", tgapi.EscapeTextForMarkdownV2(query)),
		)
	}

	return c.renderOrderList(ctx, chatID, resp.List, resp.TotalCount, page, "🔍 *Результаты поиска*", "", "search", query, messageID)
}

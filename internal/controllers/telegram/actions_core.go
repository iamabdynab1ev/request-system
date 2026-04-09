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
	"request-system/pkg/utils"
)

func (c *TelegramController) handleSelectOrderAction(ctx context.Context, chatID int64, mid int, orderID uint64) error {
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	order, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, orderID)
	if err != nil {
		_ = c.answerCallback(ctx, "Заявка не найдена или нет доступа")
		return nil
	}

	source := ""
	searchQuery := ""
	page := 1
	currentState, err := c.getUserState(ctx, chatID)
	if err == nil && currentState != nil && currentState.MessageID == mid {
		source = currentState.Source
		searchQuery = currentState.SearchQuery
		if currentState.Page > 0 {
			page = currentState.Page
		}
	}

	state := dto.NewTelegramState(orderID, mid, source, searchQuery, page)
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	return c.showEditMenuForState(ctx, chatID, state, order)
}

func (c *TelegramController) handleSaveChanges(ctx context.Context, chatID int64, messageID int) error {
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		c.logger.Error("Ошибка получения контекста пользователя при сохранении", zap.Error(err), zap.Int64("chat_id", chatID))
		return c.handlePrepareUserContextError(ctx, chatID, err)
	}

	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}

	if state.MessageID != messageID {
		_ = c.answerCallback(ctx, "Меню уже обновлено")
		return nil
	}

	if state.OrderID == 0 {
		c.logger.Error("State не содержит OrderID", zap.Int64("chat_id", chatID), zap.Uint64("user_id", user.ID))
		return c.sendStaleStateError(ctx, chatID, messageID)
	}

	if !state.HasChanges() {
		_ = c.answerCallback(ctx, "Нет изменений для сохранения")
		return nil
	}

	currentOrder, err := c.orderService.FindOrderByIDForTelegram(userCtx, user.ID, state.OrderID)
	if err != nil {
		c.logger.Error("Не удалось получить заявку для сохранения", zap.Error(err), zap.Uint64("order_id", state.OrderID), zap.Uint64("user_id", user.ID))
		return c.tgService.EditMessageText(ctx, chatID, messageID, "❌ Ошибка при получении данных заявки\\.", telegram.WithMarkdownV2())
	}

	orderTypeCode, _ := c.orderTypeRepo.FindCodeByID(ctx, *currentOrder.OrderTypeID)
	if orderTypeCode != "EQUIPMENT" {
		comment, exists := state.GetComment()
		if !exists || strings.TrimSpace(comment) == "" {
			return c.tgService.EditMessageText(
				ctx,
				chatID,
				messageID,
				"⚠️ *Ошибка сохранения*\n\nДля этого типа заявки комментарий обязателен\\. Сначала нажмите *💬 Комментарий* и заполните его\\.",
				telegram.WithMarkdownV2(),
			)
		}
	}

	updateDTO := dto.UpdateOrderDTO{}
	changesMap := make(map[string]interface{})

	sid, sidExists, _ := state.GetStatusID()
	if sidExists && currentOrder.StatusID != sid {
		updateDTO.StatusID = &sid
		changesMap["status_id"] = sid
	}

	eid, eidExists, _ := state.GetExecutorID()
	if eidExists {
		if eid == 0 {
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
	}

	dur, durExists, durErr := state.GetDuration()
	if durErr != nil {
		return c.sendInternalError(ctx, chatID)
	}
	if durExists {
		updateDTO.Duration = dur
		changesMap["duration"] = dur
	}

	c.logger.Info("Сохранение через Telegram", zap.Uint64("order_id", state.OrderID), zap.Uint64("user_id", user.ID), zap.Any("updateDTO", updateDTO), zap.Any("changesMap", changesMap))

	_, err = c.orderService.UpdateOrder(userCtx, state.OrderID, updateDTO, nil, changesMap)
	if err != nil {
		c.logger.Error("Ошибка сохранения через Telegram", zap.Error(err), zap.Uint64("order_id", state.OrderID), zap.Uint64("user_id", user.ID), zap.Any("updateDTO", updateDTO), zap.Any("changesMap", changesMap))

		errorMsg := "❌ *Ошибка сохранения*\n\n"
		errStr := err.Error()

		switch {
		case strings.Contains(errStr, "Forbidden") || strings.Contains(errStr, "прав"):
			errorMsg += "_Недостаточно прав для этой операции\\._"
		case strings.Contains(errStr, "закрыта") || strings.Contains(errStr, "CLOSED"):
			errorMsg += "_Заявка закрыта\\. Редактирование запрещено\\._"
		case strings.Contains(errStr, "коммент"):
			errorMsg += "_Не заполнен обязательный комментарий\\._"
		case strings.Contains(errStr, "no changes") || strings.Contains(errStr, "изменен"):
			errorMsg += "_Нет изменений для сохранения\\._"
		default:
			errorMsg += fmt.Sprintf("_Причина: %s_", telegram.EscapeTextForMarkdownV2(errStr))
		}
		return c.tgService.EditMessageText(ctx, chatID, messageID, errorMsg, telegram.WithMarkdownV2())
	}

	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
	_ = c.answerCallback(ctx, "Сохранено")
	return c.returnToStateSource(ctx, chatID, messageID, state)
}

func (c *TelegramController) returnToStateSource(ctx context.Context, chatID int64, messageID int, state *dto.TelegramState) error {
	if state == nil {
		return c.handleMyTasksCommand(ctx, chatID, messageID)
	}

	page := state.Page
	if page < 1 {
		page = 1
	}

	switch state.Source {
	case "all":
		return c.handleAllOrdersPage(ctx, chatID, page, messageID)
	case "assigned":
		return c.handleAssignedToMePage(ctx, chatID, page, messageID)
	case "today":
		return c.handleTodayTasksPage(ctx, chatID, page, messageID)
	case "overdue":
		return c.handleOverdueTasksPage(ctx, chatID, page, messageID)
	case "search":
		if strings.TrimSpace(state.SearchQuery) != "" {
			return c.renderSearchResults(ctx, chatID, messageID, state.SearchQuery, false, page)
		}
		return c.handleSearchStart(ctx, chatID, messageID)
	default:
		return c.handleMyTasksPage(ctx, chatID, page, messageID)
	}
}

func (c *TelegramController) handleCallbackQuery(ctx context.Context, query *TelegramCallbackQuery) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(query.Data), &data); err != nil {
		c.logger.Error("Не удалось декодировать callback", zap.String("data", query.Data), zap.Error(err))
		return nil
	}

	action, _ := data["action"].(string)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	switch action {
	case "main_menu":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.sendMainMenu(ctx, chatID)
	case "main_all":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleAllOrdersCommand(ctx, chatID, msgID)
	case "main_my_tasks":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleMyTasksCommand(ctx, chatID, msgID)
	case "main_assigned":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleAssignedToMeCommand(ctx, chatID, msgID)
	case "main_involved":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleInvolvedCommand(ctx, chatID, msgID)
	case "main_today":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleTodayTasksCommand(ctx, chatID, msgID)
	case "main_overdue":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleOverdueTasksCommand(ctx, chatID, msgID)
	case "main_search":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleSearchStart(ctx, chatID, msgID)
	case "main_stats":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleStatsCommand(ctx, chatID, msgID)
	case "main_status":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleLinkStatusCommand(ctx, chatID)
	case "main_help":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleHelpCommand(ctx, chatID)
	case "list_page":
		page := 1
		if pageRaw, ok := data["page"].(float64); ok {
			page = int(pageRaw)
		}
		return c.handleListPageAction(ctx, chatID, msgID, page)
	case "list_page_info":
		_ = c.answerCallback(ctx, "Текущая страница")
		return nil
	case "unlink_prompt":
		return c.handleUnlinkCommand(ctx, chatID)
	case "unlink_confirm":
		return c.handleConfirmUnlinkAction(ctx, chatID)
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
		state, _ := c.getUserState(ctx, chatID)
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.returnToStateSource(ctx, chatID, msgID, state)
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
			return c.handleSetSomething(ctx, chatID, "status_id", uint64(id), "Статус обновлён")
		}
	case "set_duration":
		if val, ok := data["value"].(string); ok {
			return c.handleSetDuration(ctx, chatID, val)
		}
	case "set_executor":
		if id, ok := data["user_id"].(float64); ok {
			return c.handleSetSomething(ctx, chatID, "executor_id", uint64(id), "Исполнитель назначен")
		}
	default:
		c.logger.Warn("Неизвестный callback action", zap.String("action", action))
	}

	return nil
}

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

	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return c.handlePrepareUserContextError(ctx, chatID, err)
	}

	perms, _ := utils.GetPermissionsMapFromCtx(userCtx)
	authCtx := authz.Context{
		Actor:         user,
		Permissions:   perms,
		Target:        order,
		IsParticipant: order.CreatorID == user.ID || (order.ExecutorID != nil && *order.ExecutorID == user.ID),
	}

	canStatus := authz.CanDo(authz.OrdersUpdateStatusID, authCtx)
	canDuration := authz.CanDo(authz.OrdersUpdateDuration, authCtx)
	canComment := authz.CanDo(authz.OrdersUpdateComment, authCtx)
	canDelegate := authz.CanDo(authz.OrdersUpdateExecutorID, authCtx)
	canEdit := canStatus || canDuration || canComment || canDelegate

	var text strings.Builder
	text.WriteString(fmt.Sprintf("📋 *Заявка №%d*\n\n", order.ID))
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

		history, historyErr := c.orderHistoryRepo.FindByOrderID(ctx, order.ID, 1, 0)
		if historyErr == nil && len(history) > 0 {
			for i := len(history) - 1; i >= 0; i-- {
				if history[i].Comment.Valid && strings.TrimSpace(history[i].Comment.String) != "" {
					text.WriteString(fmt.Sprintf("\n💬 *Последний комментарий:*\n_%s_\n", telegram.EscapeTextForMarkdownV2(history[i].Comment.String)))
					break
				}
			}
		}
	} else {
		text.WriteString("⏰ *Срок:* _не задан_\n")
	}

	var keyboard [][]telegram.InlineKeyboardButton
	isClosed := status.Code != nil && *status.Code == "CLOSED"

	if isClosed || !canEdit {
		if isClosed {
			text.WriteString("\n🔒 *Заявка закрыта*\\.\n_Карточка доступна только для просмотра\\. Редактирование недоступно\\._")
		} else {
			text.WriteString("\n👁️ *Режим просмотра*\\.\n_У вас есть доступ к просмотру заявки, но нет прав на её редактирование\\._")
		}
		keyboard = append(keyboard, []telegram.InlineKeyboardButton{{Text: "◀️ К списку", CallbackData: `{"action":"edit_cancel"}`}})
	} else {
		text.WriteString("\n_Выберите действие:_")

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

		row2 := []telegram.InlineKeyboardButton{}
		if canComment {
			row2 = append(row2, telegram.InlineKeyboardButton{Text: "💬 Комментарий", CallbackData: `{"action":"edit_comment_start"}`})
		}
		if canDelegate {
			row2 = append(row2, telegram.InlineKeyboardButton{Text: "👤 Делегировать", CallbackData: `{"action":"edit_delegate_start"}`})
		}
		if len(row2) > 0 {
			keyboard = append(keyboard, row2)
		}

		keyboard = append(keyboard, []telegram.InlineKeyboardButton{
			{Text: "✅ Сохранить", CallbackData: `{"action":"edit_save"}`},
			{Text: menuBackButton, CallbackData: `{"action":"edit_cancel"}`},
		})
	}

	return c.renderScreen(ctx, chatID, messageID, text.String(), telegram.WithKeyboard(keyboard), telegram.WithMarkdownV2())
}

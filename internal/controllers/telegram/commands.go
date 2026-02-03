// internal/controllers/telegram/commands.go
package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/pkg/telegram"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

// ==================== ОБРАБОТКА КОМАНД ====================
func (c *TelegramController) handleCommand(ctx context.Context, chatID int64, text string) {
	switch {
	case strings.HasPrefix(text, "/start"):
		_ = c.handleStartCommand(ctx, chatID, text)
	case strings.HasPrefix(text, "/my_tasks") && c.cfg.AdvancedMode:
		_ = c.handleMyTasksCommand(ctx, chatID)
	case strings.HasPrefix(text, "/stats") && c.cfg.AdvancedMode:
		_ = c.handleStatsCommand(ctx, chatID)
	case strings.HasPrefix(text, "/help"):
		_ = c.handleHelpCommand(ctx, chatID)
	default:
		_ = c.tgService.SendMessageEx(ctx, chatID,
			"❓ Неизвестная команда\\. Используйте /help для помощи\\.",
			telegram.WithMarkdownV2())
	}
}

// НОВАЯ ЛОГИКА /start - теперь запрашивает токен
func (c *TelegramController) handleStartCommand(ctx context.Context, chatID int64, text string) error {
	existingUser, _, err := c.prepareUserContext(ctx, chatID)
	if err == nil && existingUser != nil {
		msg := fmt.Sprintf(
			"👤 Вы уже авторизованы как: *%s*\n\n"+
				"🔹 Для работы используйте меню ниже\\.\n"+
				"🔹 Для смены аккаунта отправьте новый токен\\.",
			telegram.EscapeTextForMarkdownV2(existingUser.Fio))
		_ = c.tgService.SendMessageEx(ctx, chatID, msg, telegram.WithMarkdownV2())
		return c.sendMainMenu(ctx, chatID)
	}
	welcomeMsg := "👋 *Добро пожаловать в систему заявок банка\\!*\n\n" +
		"Для начала работы отправьте мне *код привязки* из вашего профиля на сайте\\.\n\n" +
		"📍 Где взять код:\n" +
		"1\\. Войдите на сайт системы заявок\n" +
		"2\\. Откройте раздел *\"Профиль\"*\n" +
		"3\\. Нажмите *\"Привязать Telegram\"*\n" +
		"4\\. Скопируйте код и отправьте его сюда\n\n" +
		"_Код действителен 5 минут_"
	return c.tgService.SendMessageEx(ctx, chatID, welcomeMsg, telegram.WithMarkdownV2())
}

func (c *TelegramController) handleHelpCommand(ctx context.Context, chatID int64) error {
	helpText := "📖 *Справка по боту*\n\n" +
		"*Основные команды:*\n" +
		"/start \\- Начало работы и привязка аккаунта\n" +
		"/my\\_tasks \\- Мои заявки\n" +
		"/stats \\- Моя статистика\n" +
		"/help \\- Эта справка\n\n" +
		"*Кнопки меню:*\n" +
		"📋 Мои Заявки \\- Все активные заявки\n" +
		"⏰ На сегодня \\- Заявки со сроком сегодня\n" +
		"🔴 Просроченные \\- Требуют срочного внимания\n" +
		"🔍 Поиск \\- Найти заявку по номеру/тексту\n" +
		"📊 Статистика \\- Ваши показатели\n\n" +
		"_По вопросам обращайтесь в поддержку_"
	return c.tgService.SendMessageEx(ctx, chatID, helpText, telegram.WithMarkdownV2())
}

// ==================== ОБРАБОТКА ТЕКСТА ====================
func (c *TelegramController) handleTextMessage(ctx context.Context, chatID int64, text string) error {
	if isUUIDFormat(text) {
		return c.handleTokenLink(ctx, chatID, text)
	}
	state, err := c.getUserState(ctx, chatID)
	if err == nil && state != nil {
		return c.handleStateInput(ctx, chatID, text, state)
	}
	return c.handleMenuButton(ctx, chatID, text)
}

func (c *TelegramController) handleTokenLink(ctx context.Context, chatID int64, token string) error {
	err := c.userService.ConfirmTelegramLink(ctx, token, chatID)
	if err != nil {
		c.logger.Warn("Неверный токен привязки",
			zap.Int64("chat_id", chatID),
			zap.Error(err))
		return c.tgService.SendMessageEx(ctx, chatID,
			"❌ *Ошибка привязки*\n\n"+
				"Код неверный или устарел\\.\n"+
				"Получите новый код на сайте\\.",
			telegram.WithMarkdownV2())
	}
	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
	_ = c.tgService.SendMessageEx(ctx, chatID,
		"✅ *Аккаунт успешно привязан\\!*\n\n"+
			"Теперь вы будете получать уведомления о заявках\\.",
		telegram.WithMarkdownV2())
	return c.sendMainMenu(ctx, chatID)
}

func (c *TelegramController) handleStateInput(ctx context.Context, chatID int64, text string, state *dto.TelegramState) error {
	switch state.Mode {
	case "awaiting_comment":
		return c.handleSetComment(ctx, chatID, text)
	case "awaiting_duration":
		return c.handleSetDuration(ctx, chatID, text)
	case "awaiting_executor":
		return c.handleSetExecutorFromText(ctx, chatID, text)
	case "awaiting_search":
		return c.handleSearchQuery(ctx, chatID, text)
	default:
		return c.handleMenuButton(ctx, chatID, text)
	}
}

func (c *TelegramController) handleMenuButton(ctx context.Context, chatID int64, text string) error {
	switch text {
	case "📋 Мои Заявки":
		return c.handleMyTasksCommand(ctx, chatID)
	case "⏰ На сегодня":
		return c.handleTodayTasksCommand(ctx, chatID)
	case "🔴 Просроченные":
		return c.handleOverdueTasksCommand(ctx, chatID)
	case "📊 Статистика":
		return c.handleStatsCommand(ctx, chatID)
	case "🔍 Поиск":
		return c.handleSearchStart(ctx, chatID, 0)
	default:
		return nil
	}
}

// ==================== КОМАНДЫ СПИСКА ЗАЯВОК ====================
func (c *TelegramController) handleMyTasksCommand(ctx context.Context, chatID int64, messageID ...int) error {
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}
	
	filter := types.Filter{
		Limit: 10, 
		Page: 1,
		Filter: map[string]interface{}{
			"user_id": user.ID, 
		},
	}
	resp, err := c.orderService.GetOrders(userCtx, filter, true)
	if err != nil {
		c.logger.Error("GetOrders failed", zap.Error(err), zap.Int64("chat_id", chatID))
		return c.tgService.SendMessageEx(ctx, chatID, "❌ Ошибка загрузки заявок\\.", telegram.WithMarkdownV2())
	}
	statusMap := c.getStatusMap(ctx)
	var text strings.Builder
	var keyboard [][]telegram.InlineKeyboardButton
	
	if len(resp.List) == 0 {
		text.WriteString("✅ У вас нет активных заявок\\.")
	} else {
		// Отображаем, сколько показываем
		text.WriteString(fmt.Sprintf("📋 *Ваши последние заявки* \\(%d\\):\n\n", len(resp.List)))
		
		currentRow := []telegram.InlineKeyboardButton{}
		for _, order := range resp.List {
			emoji := getStatusEmoji(statusMap[order.StatusID])
			
			// Формируем красивый список текстом
			text.WriteString(fmt.Sprintf("%s *№%d* • %s\n",
				emoji, order.ID, telegram.EscapeTextForMarkdownV2(order.Name)))
			
			// Формируем кнопку
			cb := fmt.Sprintf(`{"action":"select_order","order_id":%d}`, order.ID)
			currentRow = append(currentRow, telegram.InlineKeyboardButton{
				Text:         fmt.Sprintf("№%d", order.ID),
				CallbackData: cb,
			})
			
			// Разбиваем кнопки по 5 в ряд
			if len(currentRow) >= 5 {
				keyboard = append(keyboard, currentRow)
				currentRow = []telegram.InlineKeyboardButton{}
			}
		}
		
		if len(currentRow) > 0 {
			keyboard = append(keyboard, currentRow)
		}
		
		if len(resp.List) >= 10 {
			text.WriteString("\n_Показаны первые 10 заявок\\._")
		} else {
			text.WriteString("\n_Выберите заявку:_")
		}
	}
	
	// ✅ ИЗМЕНЕНИЕ: Добавляем кнопку выхода/обновления, чтобы меню выглядело законченным
	keyboard = append(keyboard, []telegram.InlineKeyboardButton{
		{Text: "❌ Закрыть список", CallbackData: `{"action":"main_menu"}`},
	})

	mid := 0
	if len(messageID) > 0 {
		mid = messageID[0]
	}
	return c.tgService.EditOrSendMessage(ctx, chatID, mid, text.String(),
		telegram.WithKeyboard(keyboard), telegram.WithMarkdownV2())
}

func (c *TelegramController) handleTodayTasksCommand(ctx context.Context, chatID int64, messageID ...int) error {
	_, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}
	now := time.Now().In(c.loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, c.loc)
	endOfDay := startOfDay.Add(24 * time.Hour)
	
	filter := types.Filter{
		Limit: maxOrdersPerPage,
		Page:  1,
		Filter: map[string]interface{}{
			"created_from": startOfDay,  
			"created_to":   endOfDay,
		},
	}

	resp, err := c.orderService.GetOrders(userCtx, filter, true)
	if err != nil {
		c.logger.Error("GetOrders failed", zap.Error(err))
		return c.tgService.SendMessageEx(ctx, chatID, "❌ Ошибка загрузки заявок\\.", telegram.WithMarkdownV2())
	}
	return c.renderOrderList(ctx, chatID, resp.List, "⏰ *Заявки на сегодня*",
		"✅ *Заявок на сегодня нет\\!*\n\n_Можете отдохнуть_ 😊", messageID...)
}
func (c *TelegramController) handleOverdueTasksCommand(ctx context.Context, chatID int64, messageID ...int) error {
	_, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}
	now := time.Now().In(c.loc)
	filter := types.Filter{
		Limit: maxOrdersPerPage,
		Page:  1,
		Filter: map[string]interface{}{
			"overdue": true,
		},
	}
	resp, err := c.orderService.GetOrders(userCtx, filter, true)
	if err != nil {
		c.logger.Error("GetOrders failed", zap.Error(err))
		return c.tgService.SendMessageEx(ctx, chatID, "❌ Ошибка загрузки заявок\\.", telegram.WithMarkdownV2())
	}
	var overdueOrders []dto.OrderResponseDTO
	for _, order := range resp.List {
		if order.Duration != nil && order.Duration.Before(now) {
			status, err := c.statusRepo.FindStatus(ctx, order.StatusID)
			if err == nil && status.Code != nil &&
				*status.Code != "CLOSED" && *status.Code != "REJECTED" {
				overdueOrders = append(overdueOrders, order)
			}
		}
	}
	return c.renderOrderList(ctx, chatID, overdueOrders, "🔴 *Просроченные заявки*",
		"✅ *Просроченных заявок нет\\!*\n\n_Отличная работа_ 👍", messageID...)
}

func (c *TelegramController) handleStatsCommand(ctx context.Context, chatID int64, messageID ...int) error {
	user, _, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}
	stats, err := c.orderService.GetUserStats(ctx, user.ID)
	if err != nil {
		return c.tgService.SendMessageEx(ctx, chatID, "❌ Ошибка получения статистики\\.", telegram.WithMarkdownV2())
	}
	avgHours := int(stats.AvgResolutionSeconds / 3600)
	avgMinutes := int((stats.AvgResolutionSeconds - float64(avgHours*3600)) / 60)
	var text strings.Builder
	text.WriteString("📊 *Ваша статистика*\n━━━━━━━━━━━━━━━━━━━━\n\n")
	text.WriteString(fmt.Sprintf("📝 *Всего:* %d\n", stats.TotalCount))
	text.WriteString(fmt.Sprintf("⚙️ В работе: %d\n", stats.InProgressCount))
	text.WriteString(fmt.Sprintf("✅ Готово: %d\n\n", stats.CompletedCount))
	if avgHours > 0 || avgMinutes > 0 {
		text.WriteString(fmt.Sprintf("⏱️ Среднее время: %dч %dмин\n", avgHours, avgMinutes))
	}
	mid := 0
	if len(messageID) > 0 {
		mid = messageID[0]
	}
	return c.tgService.EditOrSendMessage(ctx, chatID, mid, text.String(),
		telegram.WithMarkdownV2())
}

// ==================== ПОИСК ====================
func (c *TelegramController) handleSearchStart(ctx context.Context, chatID int64, messageID int) error {
	state := &dto.TelegramState{
		Mode:      "awaiting_search",
		MessageID: messageID,
		Changes:   make(map[string]string),
	}
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	text := "🔍 *Поиск заявки*\n\n" +
		"Введите:\n" +
		"• Номер заявки \\(например: `123`\\)\n" +
		"• Или текст из описания"
	keyboard := [][]telegram.InlineKeyboardButton{
		{{Text: "❌ Отменить", CallbackData: `{"action":"main_menu"}`}},
	}
	return c.tgService.EditMessageText(ctx, chatID, messageID, text,
		telegram.WithKeyboard(keyboard), telegram.WithMarkdownV2())
}

func (c *TelegramController) handleSearchQuery(ctx context.Context, chatID int64, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return c.tgService.SendMessageEx(ctx, chatID,
			"❌ Поисковый запрос не может быть пустым\\.",
			telegram.WithMarkdownV2())
	}
	if len(text) > maxSearchQueryLength {
		return c.tgService.SendMessageEx(ctx, chatID,
			"❌ Запрос слишком длинный \\(макс\\. 100 символов\\)\\.",
			telegram.WithMarkdownV2())
	}
	_, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}
	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
	var orderID uint64
	if _, err := fmt.Sscanf(text, "%d", &orderID); err == nil {
		userID, _ := utils.GetUserIDFromCtx(userCtx)
		order, err := c.orderService.FindOrderByIDForTelegram(userCtx, userID, orderID)
		if err == nil {
			return c.sendEditMenu(ctx, chatID, 0, order)
		}
	}
	filter := types.Filter{
		Limit:  20,
		Page:   1,
		Search: text,
	}
	resp, err := c.orderService.GetOrders(userCtx, filter, true)
	if err != nil {
		c.logger.Error("Search failed", zap.Error(err))
		return c.tgService.SendMessageEx(ctx, chatID, "❌ Ошибка поиска\\.", telegram.WithMarkdownV2())
	}
	if len(resp.List) == 0 {
		return c.tgService.SendMessageEx(ctx, chatID,
			fmt.Sprintf("❌ По запросу \"%s\" ничего не найдено\\.",
				telegram.EscapeTextForMarkdownV2(text)),
			telegram.WithMarkdownV2())
	}
	return c.renderOrderList(ctx, chatID, resp.List,
		fmt.Sprintf("🔍 *Результаты поиска* \\(%d\\):", len(resp.List)), "")
}

// ==================== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ====================
func (c *TelegramController) sendMainMenu(ctx context.Context, chatID int64) error {
	if !c.cfg.AdvancedMode {
		return c.tgService.SendMessageEx(ctx, chatID, "✅ Подключено к боту\\!", telegram.WithMarkdownV2())
	}
	text := "🏠 *Главное меню*\n\n" +
		"Система заявок банка\\.\n" +
		"Выберите действие из меню ниже\\."
	keyboard := [][]telegram.ReplyKeyboardButton{
		{{Text: "📋 Мои Заявки"}},
		{{Text: "⏰ На сегодня"}, {Text: "🔴 Просроченные"}},
		{{Text: "🔍 Поиск"}, {Text: "📊 Статистика"}},
	}
	return c.tgService.SendMessageEx(ctx, chatID, text,
		telegram.WithReplyKeyboard(keyboard),
		telegram.WithMarkdownV2())
}

func (c *TelegramController) renderOrderList(ctx context.Context, chatID int64, orders []dto.OrderResponseDTO,
	title string, emptyText string, messageID ...int) error {
	var text strings.Builder
	var keyboard [][]telegram.InlineKeyboardButton
	if len(orders) == 0 {
		text.WriteString(emptyText)
	} else {
		text.WriteString(fmt.Sprintf("%s \\(%d\\):\n\n", title, len(orders)))
		statusMap := c.getStatusMap(ctx)
		currentRow := []telegram.InlineKeyboardButton{}
		for _, order := range orders {
			emoji := getStatusEmoji(statusMap[order.StatusID])
			text.WriteString(fmt.Sprintf("%s *№%d* • %s\n",
				emoji, order.ID, telegram.EscapeTextForMarkdownV2(order.Name)))
			cb := fmt.Sprintf(`{"action":"select_order","order_id":%d}`, order.ID)
			currentRow = append(currentRow, telegram.InlineKeyboardButton{
				Text:         fmt.Sprintf("№%d", order.ID),
				CallbackData: cb,
			})
			if len(currentRow) >= 5 {
				keyboard = append(keyboard, currentRow)
				currentRow = []telegram.InlineKeyboardButton{}
			}
		}
		if len(currentRow) > 0 {
			keyboard = append(keyboard, currentRow)
		}
		text.WriteString("\n_Выберите заявку:_")
	}
	keyboard = append(keyboard, []telegram.InlineKeyboardButton{
		{Text: "🏠 Главное меню", CallbackData: `{"action":"main_menu"}`},
	})
	mid := 0
	if len(messageID) > 0 {
		mid = messageID[0]
	}
	return c.tgService.EditOrSendMessage(ctx, chatID, mid, text.String(),
		telegram.WithKeyboard(keyboard), telegram.WithMarkdownV2())
}

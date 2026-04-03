// internal/controllers/telegram/commands.go
package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"request-system/internal/dto"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/telegram"
	"request-system/pkg/types"
)

func (c *TelegramController) handleCommand(ctx context.Context, chatID int64, text string) error {
	switch {
	case strings.HasPrefix(text, "/start"):
		return c.handleStartCommand(ctx, chatID, text)
	case strings.HasPrefix(text, "/menu"):
		return c.sendMainMenu(ctx, chatID)
	case strings.HasPrefix(text, "/my_tasks") && c.cfg.AdvancedMode:
		return c.handleMyTasksCommand(ctx, chatID)
	case strings.HasPrefix(text, "/stats") && c.cfg.AdvancedMode:
		return c.handleStatsCommand(ctx, chatID)
	case strings.HasPrefix(text, "/status"):
		return c.handleLinkStatusCommand(ctx, chatID)
	case strings.HasPrefix(text, "/unlink"):
		return c.handleUnlinkCommand(ctx, chatID)
	case strings.HasPrefix(text, "/help"):
		return c.handleHelpCommand(ctx, chatID)
	default:
		return c.tgService.SendMessageEx(
			ctx,
			chatID,
			"❌ Неизвестная команда. Используйте /menu или /help.",
			telegram.WithMarkdownV2(),
		)
	}
}

func (c *TelegramController) handleStartCommand(ctx context.Context, chatID int64, text string) error {
	if token := extractStartToken(text); token != "" {
		return c.handleTokenLink(ctx, chatID, token)
	}

	existingUser, _, err := c.prepareUserContext(ctx, chatID)
	if err == nil && existingUser != nil {
		msg := fmt.Sprintf(
			"👤 *Вы уже авторизованы как:* %s\n\n"+
				"Используйте меню ниже для работы с заявками\\.\n\n"+
				"*Команды:*\n"+
				"/status \\- показать, к какому аккаунту привязан этот Telegram\n"+
				"/unlink \\- отвязать этот Telegram от текущего аккаунта",
			telegram.EscapeTextForMarkdownV2(existingUser.Fio),
		)
		return c.renderScreen(ctx, chatID, 0, msg, c.mainMenuScreenOptions()...)
	}

	welcomeMsg := "👋 *Добро пожаловать в Telegram\\-бот HelpDesk*\n\n" +
		"Этот бот позволяет работать с заявками банка прямо из Telegram\\.\n\n" +
		"*Как привязать Telegram:*\n" +
		"1\\. Откройте сайт HelpDesk\n" +
		"2\\. Перейдите в профиль\n" +
		"3\\. Нажмите *Привязать Telegram*\n" +
		"4\\. Получите код и отправьте его сюда командой `/start <код>`\n\n" +
		"После привязки будут доступны список заявок, поиск, статистика, комментарии, изменение срока и делегирование\\."

	return c.renderScreen(ctx, chatID, 0, welcomeMsg, telegram.WithMarkdownV2())
}

func extractStartToken(text string) string {
	parts := strings.Fields(strings.TrimSpace(text))
	if len(parts) < 2 {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

func (c *TelegramController) handleHelpCommand(ctx context.Context, chatID int64) error {
	helpText := "📖 *Справка по боту*\n\n" +
		"Бот работает с теми же правами доступа, что и веб\\-проект\\. Если у вас нет доступа к заявке на сайте, бот тоже её не покажет\\.\n\n" +
		"*Основные команды:*\n" +
		"/start \\- начало работы и привязка аккаунта по коду из профиля\n" +
		"/menu \\- открыть главное меню\n" +
		"/my\\_tasks \\- показать ваши последние заявки\n" +
		"/stats \\- показать личную статистику за последние 30 дней\n" +
		"/status \\- показать, к какому аккаунту привязан этот Telegram\n" +
		"/unlink \\- отвязать этот Telegram от текущего аккаунта\n" +
		"/help \\- открыть эту справку\n\n" +
		"*Кнопки меню:*\n" +
		"📋 *Мои заявки* \\- ваши последние активные заявки\n" +
		"👨‍💼 *Назначены мне* \\- заявки, где вы указаны исполнителем\n" +
		"⏰ *На сегодня* \\- заявки, созданные сегодня\n" +
		"🔴 *Просроченные* \\- заявки с просроченным сроком\n" +
		"🔍 *Поиск* \\- найти заявку по номеру или по тексту\n" +
		"📊 *Статистика* \\- ваша краткая сводка по заявкам\n" +
		"🔐 *Статус* \\- проверить текущую привязку Telegram\n" +
		"📖 *Справка* \\- снова открыть эту подсказку\n\n" +
		"*Что можно делать в карточке заявки:*\n" +
		"• открыть заявку из списка\n" +
		"• изменить статус \\(если у вас есть права\\)\n" +
		"• изменить срок\n" +
		"• добавить комментарий\n" +
		"• делегировать другому сотруднику\n" +
		"• сохранить изменения\n\n" +
		"*Важно:*\n" +
		"• все действия зависят от ваших прав и текущего статуса заявки\n" +
		"• критические действия требуют подтверждения\n" +
		"• если потеряли навигацию, используйте /menu"

	return c.renderScreen(ctx, chatID, 0, helpText, c.mainMenuScreenOptions()...)
}

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

func (c *TelegramController) sendTelegramLinkError(ctx context.Context, chatID int64, errMessage string) error {
	return c.tgService.SendMessageEx(
		ctx,
		chatID,
		"❌ *Ошибка привязки*\n\n"+telegram.EscapeTextForMarkdownV2(errMessage),
		telegram.WithMarkdownV2(),
	)
}

func (c *TelegramController) handleTokenLink(ctx context.Context, chatID int64, token string) error {
	err := c.userService.ConfirmTelegramLink(ctx, token, chatID)
	if err != nil {
		c.logger.Warn("Неверный токен привязки", zap.Int64("chat_id", chatID), zap.Error(err))

		errMessage := "Код неверный или устарел. Получите новый код на сайте."
		var httpErr *apperrors.HttpError
		if errors.As(err, &httpErr) && strings.TrimSpace(httpErr.Message) != "" {
			errMessage = httpErr.Message
		}
		return c.sendTelegramLinkError(ctx, chatID, errMessage)
	}

	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
	return c.renderScreen(
		ctx,
		chatID,
		0,
		"✅ *Аккаунт успешно привязан\\!*\n\nТеперь вы можете работать с заявками через меню ниже\\.",
		c.mainMenuScreenOptions()...,
	)
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
	case menuMyTasksButton:
		return c.handleMyTasksCommand(ctx, chatID)
	case menuAssignedButton:
		return c.handleAssignedToMeCommand(ctx, chatID)
	case menuTodayButton:
		return c.handleTodayTasksCommand(ctx, chatID)
	case menuOverdueButton:
		return c.handleOverdueTasksCommand(ctx, chatID)
	case menuStatsButton:
		return c.handleStatsCommand(ctx, chatID)
	case menuSearchButton:
		return c.handleSearchStart(ctx, chatID, 0)
	case menuStatusButton:
		return c.handleLinkStatusCommand(ctx, chatID)
	case menuHelpButton:
		return c.handleHelpCommand(ctx, chatID)
	case menuMainButton:
		return c.sendMainMenu(ctx, chatID)
	default:
		return nil
	}
}

func (c *TelegramController) handleMyTasksCommand(ctx context.Context, chatID int64, messageID ...int) error {
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	filter := types.Filter{
		Limit: 10,
		Page:  1,
		Filter: map[string]interface{}{
			"creator_id": user.ID,
		},
	}
	resp, err := c.orderService.GetOrders(userCtx, filter, true, false)
	if err != nil {
		c.logger.Error("GetOrders failed", zap.Error(err), zap.Int64("chat_id", chatID))
		mid := 0
		if len(messageID) > 0 {
			mid = messageID[0]
		}
		return c.renderHomeScreen(ctx, chatID, mid, "❌ Ошибка загрузки заявок\\.")
	}

	return c.renderOrderList(
		ctx,
		chatID,
		resp.List,
		"📋 *Мои заявки*",
		"✅ У вас нет активных заявок\\.",
		"my_tasks",
		"",
		messageID...,
	)
}

func (c *TelegramController) handleAssignedToMeCommand(ctx context.Context, chatID int64, messageID ...int) error {
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	filter := types.Filter{
		Limit: 10,
		Page:  1,
		Filter: map[string]interface{}{
			"executor_id": user.ID,
		},
	}
	resp, err := c.orderService.GetOrders(userCtx, filter, false, true)
	if err != nil {
		c.logger.Error("GetOrders failed", zap.Error(err), zap.Int64("chat_id", chatID))
		mid := 0
		if len(messageID) > 0 {
			mid = messageID[0]
		}
		return c.renderHomeScreen(ctx, chatID, mid, "❌ Ошибка загрузки заявок\\.")
	}

	return c.renderOrderList(
		ctx,
		chatID,
		resp.List,
		"👨‍💼 *Назначены мне*",
		"✅ На вас сейчас нет назначенных заявок\\.",
		"assigned",
		"",
		messageID...,
	)
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

	resp, err := c.orderService.GetOrders(userCtx, filter, false, false)
	if err != nil {
		c.logger.Error("GetOrders failed", zap.Error(err), zap.Int64("chat_id", chatID))
		mid := 0
		if len(messageID) > 0 {
			mid = messageID[0]
		}
		return c.renderHomeScreen(ctx, chatID, mid, "❌ Ошибка загрузки заявок\\.")
	}

	return c.renderOrderList(
		ctx,
		chatID,
		resp.List,
		"⏰ *На сегодня*",
		"✅ Сегодня новых заявок нет\\.",
		"today",
		"",
		messageID...,
	)
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
	resp, err := c.orderService.GetOrders(userCtx, filter, false, false)
	if err != nil {
		c.logger.Error("GetOrders failed", zap.Error(err), zap.Int64("chat_id", chatID))
		mid := 0
		if len(messageID) > 0 {
			mid = messageID[0]
		}
		return c.renderHomeScreen(ctx, chatID, mid, "❌ Ошибка загрузки заявок\\.")
	}

	var overdueOrders []dto.OrderResponseDTO
	for _, order := range resp.List {
		if order.Duration != nil && order.Duration.Before(now) {
			status, statusErr := c.statusRepo.FindStatus(ctx, order.StatusID)
			if statusErr == nil && status.Code != nil && *status.Code != "CLOSED" && *status.Code != "REJECTED" {
				overdueOrders = append(overdueOrders, order)
			}
		}
	}

	return c.renderOrderList(
		ctx,
		chatID,
		overdueOrders,
		"🔴 *Просроченные заявки*",
		"✅ Просроченных заявок нет\\.",
		"overdue",
		"",
		messageID...,
	)
}

func (c *TelegramController) handleStatsCommand(ctx context.Context, chatID int64, messageID ...int) error {
	user, _, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	stats, err := c.orderService.GetUserStats(ctx, user.ID)
	if err != nil {
		mid := 0
		if len(messageID) > 0 {
			mid = messageID[0]
		}
		return c.renderHomeScreen(ctx, chatID, mid, "❌ Ошибка получения статистики\\.")
	}

	avgHours := int(stats.AvgResolutionSeconds / 3600)
	avgMinutes := int((stats.AvgResolutionSeconds - float64(avgHours*3600)) / 60)

	var text strings.Builder
	text.WriteString("📊 *Ваша статистика за 30 дней*\n\n")
	text.WriteString(fmt.Sprintf("📌 *Всего заявок:* %d\n", stats.TotalCount))
	text.WriteString(fmt.Sprintf("⚙️ *В работе:* %d\n", stats.InProgressCount))
	text.WriteString(fmt.Sprintf("✅ *Выполнено:* %d\n", stats.CompletedCount))
	text.WriteString(fmt.Sprintf("🔴 *Просрочено:* %d\n", stats.OverdueCount))
	text.WriteString(fmt.Sprintf("📁 *Закрыто:* %d\n", stats.ClosedCount))
	if avgHours > 0 || avgMinutes > 0 {
		text.WriteString(fmt.Sprintf("\n⏱ *Среднее время решения:* %d ч %d мин\n", avgHours, avgMinutes))
	}

	mid := 0
	if len(messageID) > 0 {
		mid = messageID[0]
	}
	return c.renderHomeScreen(ctx, chatID, mid, text.String())
}

func (c *TelegramController) mainMenuKeyboard() [][]telegram.InlineKeyboardButton {
	return [][]telegram.InlineKeyboardButton{
		{
			{Text: menuMyTasksButton, CallbackData: `{"action":"main_my_tasks"}`},
			{Text: menuAssignedButton, CallbackData: `{"action":"main_assigned"}`},
		},
		{
			{Text: menuTodayButton, CallbackData: `{"action":"main_today"}`},
			{Text: menuOverdueButton, CallbackData: `{"action":"main_overdue"}`},
		},
		{
			{Text: menuSearchButton, CallbackData: `{"action":"main_search"}`},
			{Text: menuStatsButton, CallbackData: `{"action":"main_stats"}`},
		},
		{
			{Text: menuStatusButton, CallbackData: `{"action":"main_status"}`},
			{Text: menuHelpButton, CallbackData: `{"action":"main_help"}`},
		},
	}
}

func (c *TelegramController) mainMenuScreenOptions() []telegram.MessageOption {
	return []telegram.MessageOption{
		telegram.WithKeyboard(c.mainMenuKeyboard()),
		telegram.WithMarkdownV2(),
	}
}

func (c *TelegramController) renderHomeScreen(ctx context.Context, chatID int64, messageID int, text string) error {
	return c.renderScreen(ctx, chatID, messageID, text, c.mainMenuScreenOptions()...)
}

func (c *TelegramController) sendMainMenu(ctx context.Context, chatID int64) error {
	if !c.cfg.AdvancedMode {
		return c.tgService.SendMessageEx(ctx, chatID, "✅ Подключение к боту активно\\.", telegram.WithMarkdownV2())
	}

	text := "🏠 *Главное меню*\n\n" +
		"Система заявок банка\\.\n" +
		"Выберите действие из меню ниже\\.\n\n" +
		"*Команды:*\n" +
		"/status \\- показать, к какому аккаунту привязан этот Telegram\n" +
		"/unlink \\- отвязать этот Telegram от текущего аккаунта"

	return c.renderScreen(ctx, chatID, 0, text, c.mainMenuScreenOptions()...)
}

func (c *TelegramController) renderOrderList(
	ctx context.Context,
	chatID int64,
	orders []dto.OrderResponseDTO,
	title string,
	emptyText string,
	source string,
	searchQuery string,
	messageID ...int,
) error {
	var text strings.Builder
	var keyboard [][]telegram.InlineKeyboardButton

	if len(orders) == 0 {
		text.WriteString(emptyText)
	} else {
		text.WriteString(fmt.Sprintf("%s \\(%d\\):\n\n", title, len(orders)))
		text.WriteString("_Нажмите на заявку:_")

		statusMap := c.getStatusMap(ctx)
		for _, order := range orders {
			emoji := getStatusEmoji(statusMap[order.StatusID])
			buttonText := order.Name
			if len(buttonText) > 30 {
				buttonText = buttonText[:27] + "..."
			}
			buttonText = fmt.Sprintf("%s №%d • %s", emoji, order.ID, buttonText)
			cb := fmt.Sprintf(`{"action":"select_order","order_id":%d}`, order.ID)
			keyboard = append(keyboard, []telegram.InlineKeyboardButton{{Text: buttonText, CallbackData: cb}})
		}
	}

	keyboard = append(keyboard, []telegram.InlineKeyboardButton{{Text: menuMainButton, CallbackData: `{"action":"main_menu"}`}})

	mid := 0
	if len(messageID) > 0 {
		mid = messageID[0]
	}

	listState := &dto.TelegramState{
		Mode:        "list_view",
		MessageID:   mid,
		Source:      source,
		SearchQuery: searchQuery,
		Changes:     make(map[string]string),
	}
	if err := c.setUserState(ctx, chatID, listState); err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	if err := c.renderScreen(ctx, chatID, mid, text.String(), telegram.WithKeyboard(keyboard), telegram.WithMarkdownV2()); err != nil {
		return err
	}
	return c.syncStateMessageID(ctx, chatID, listState)
}

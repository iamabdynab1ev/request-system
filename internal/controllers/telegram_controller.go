package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/telegram"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

type TelegramController struct {
	userService           services.UserServiceInterface
	orderService          services.OrderServiceInterface
	statusRepo            repositories.StatusRepositoryInterface
	userRepo              repositories.UserRepositoryInterface
	orderHistoryRepo      repositories.OrderHistoryRepositoryInterface
	tgService             telegram.ServiceInterface
	cacheRepo             repositories.CacheRepositoryInterface
	authPermissionService services.AuthPermissionServiceInterface
	deduplicator          *RequestDeduplicator
	botToken              string
	logger                *zap.Logger
	cfg                   config.TelegramConfig
	loc                   *time.Location
}

func NewTelegramController(
	userService services.UserServiceInterface,
	orderService services.OrderServiceInterface,
	tgService telegram.ServiceInterface,
	cacheRepo repositories.CacheRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	orderHistoryRepo repositories.OrderHistoryRepositoryInterface,
	authPermissionService services.AuthPermissionServiceInterface,
	botToken string,
	logger *zap.Logger,
	cfg config.TelegramConfig,
) *TelegramController {
	loc, err := time.LoadLocation("Asia/Dushanbe")
	if err != nil {
		logger.Warn("Failed to load location, using UTC", zap.Error(err))
		loc = time.UTC
	}
	return &TelegramController{
		userService:           userService,
		orderService:          orderService,
		tgService:             tgService,
		cacheRepo:             cacheRepo,
		statusRepo:            statusRepo,
		userRepo:              userRepo,
		orderHistoryRepo:      orderHistoryRepo,
		authPermissionService: authPermissionService,
		deduplicator:          NewRequestDeduplicator(5 * time.Second),
		botToken:              botToken,
		logger:                logger,
		cfg:                   cfg,
		loc:                   loc,
	}
}

const telegramStateKey = "tg_user_state:%d"

func (c *TelegramController) HandleTelegramWebhook(ctx echo.Context) error {
	var update TelegramUpdate
	if err := ctx.Bind(&update); err != nil {
		c.logger.Error("Не удалось распарсить обновление от Telegram", zap.Error(err))
		return ctx.NoContent(http.StatusBadRequest)
	}

	// 1. Обрабатываем нажатия на кнопки
	if update.CallbackQuery != nil {
		if !c.cfg.AdvancedMode {
			return ctx.NoContent(http.StatusOK)
		}

		// Create a new background context for the goroutine to prevent it from being
		// canceled when the HTTP request handler returns.
		go func() {
			bgCtx := context.Background()
			_ = c.tgService.AnswerCallbackQuery(bgCtx, update.CallbackQuery.ID, "")
			if err := c.handleCallbackQuery(bgCtx, update.CallbackQuery); err != nil {
				c.logger.Error("Error handling callback query", zap.Error(err))
			}
		}()

		chatID := update.CallbackQuery.Message.Chat.ID
		if !c.deduplicator.TryAcquire(chatID, "callback") {
			c.logger.Warn("Дублирующийся callback запрос игнорирован", zap.Int64("chatID", chatID))
			return ctx.NoContent(http.StatusOK)
		}
		defer c.deduplicator.Release(chatID, "callback")
	}

	// 2. Обрабатываем текстовые сообщения и команды
	if update.Message != nil {
		chatID := update.Message.Chat.ID
		text := update.Message.Text
		c.logger.Info("Получено сообщение от Telegram", zap.Int64("chatID", chatID), zap.String("text", text))

		// Launch message processing in a goroutine to respond to Telegram quickly.
		go func(msg *TelegramMessage) {
			bgCtx := context.Background()
			chatID := msg.Chat.ID
			text := msg.Text

			// Команды ("/start", "/my_tasks", "/stats")
			if strings.HasPrefix(text, "/") {

				if !c.deduplicator.TryAcquire(chatID, text) {
					c.logger.Warn("Дублирующаяся команда игнорирована", zap.Int64("chatID", chatID), zap.String("command", text))
					return
				}
				defer c.deduplicator.Release(chatID, text)

				if strings.HasPrefix(text, "/start") {
					_ = c.handleStartCommand(bgCtx, chatID, text)
					return
				}
				if c.cfg.AdvancedMode && strings.HasPrefix(text, "/my_tasks") {
					_ = c.handleMyTasksCommand(bgCtx, chatID)
					return
				}
				if c.cfg.AdvancedMode && strings.HasPrefix(text, "/stats") {
					_ = c.handleStatsCommand(bgCtx, chatID)
					return
				}
				return
			}

			// Если это не команда, возможно, это ответ на наш вопрос (комментарий, срок)
			if c.cfg.AdvancedMode {
				if text == "📋 Мои Заявки" {

					if !c.deduplicator.TryAcquire(chatID, "my_tasks_button") {
						c.logger.Warn("Дублирующееся нажатие кнопки игнорировано", zap.Int64("chatID", chatID))
						return
					}
					defer c.deduplicator.Release(chatID, "my_tasks_button")

					_ = c.handleMyTasksCommand(bgCtx, chatID)
					return
				}

				_ = c.handleTextMessage(bgCtx, chatID, text)
			}
		}(update.Message)
	}

	return ctx.NoContent(http.StatusOK) // Always respond immediately.
}

func (c *TelegramController) handleStatsCommand(ctx context.Context, chatID int64, messageID ...int) error {
	c.logger.Info("handleStatsCommand: НАЧАЛО", zap.Int64("chatID", chatID))

	user, _, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		c.logger.Error("handleStatsCommand: ошибка prepareUserContext", zap.Error(err))
		return err
	}

	// Получаем статистику
	stats, err := c.orderService.GetUserStats(ctx, user.ID)
	if err != nil {
		c.logger.Error("handleStatsCommand: ошибка получения статистики", zap.Error(err))
		return c.tgService.SendMessage(ctx, chatID, "❌ Ошибка при получении статистики\\.")
	}

	// Вычисляем среднее время
	avgHours := int(stats.AvgResolutionSeconds / 3600)
	avgMinutes := int((stats.AvgResolutionSeconds - float64(avgHours*3600)) / 60)

	// Формируем красивое сообщение
	var text strings.Builder
	text.WriteString("📊 *Ваша статистика*\n")
	text.WriteString("_за последний месяц_\n")
	text.WriteString("━━━━━━━━━━━━━━━━━━━━\n\n")

	// Общее количество
	text.WriteString(fmt.Sprintf("📝 *Всего заявок:* %d\n\n", stats.TotalCount))

	// Детализация по статусам
	text.WriteString("*По статусам:*\n")
	text.WriteString(fmt.Sprintf("⚙️ В работе: %d\n", stats.InProgressCount))
	text.WriteString(fmt.Sprintf("✅ Выполнено: %d\n", stats.CompletedCount))
	text.WriteString(fmt.Sprintf("🔒 Закрыто: %d\n\n", stats.ClosedCount))

	// Просроченные (если есть)
	if stats.OverdueCount > 0 {
		text.WriteString(fmt.Sprintf("⚠️ *Просрочено:* %d \n\n", stats.OverdueCount))
	}

	// Среднее время выполнения
	if avgHours > 0 || avgMinutes > 0 {
		text.WriteString(fmt.Sprintf("⏱️ *Среднее время:*\n"))
		if avgHours > 0 {
			text.WriteString(fmt.Sprintf("%d ч ", avgHours))
		}
		if avgMinutes > 0 {
			text.WriteString(fmt.Sprintf("%d мин", avgMinutes))
		}
		text.WriteString("\n")
	}

	text.WriteString("\n━━━━━━━━━━━━━━━━━━━━")

	var msgIDToEdit int
	if len(messageID) > 0 {
		msgIDToEdit = messageID[0]
	}

	return c.tgService.EditOrSendMessage(ctx, chatID, msgIDToEdit, text.String(),
		telegram.WithMarkdownV2(),
	)
}

func (c *TelegramController) handleStartCommand(ctx context.Context, chatID int64, text string) error {
	token := strings.TrimSpace(strings.TrimPrefix(text, "/start"))

	if token == "" {
		// Если токен не указан, отправляем приветствие
		welcomeMsg := "👋 *Добро пожаловать в систему заявок\\!*\n\n" +
			"Для привязки вашего аккаунта:\n" +
			"1\\. Откройте веб\\-приложение\n" +
			"2\\. Перейдите в профиль\n" +
			"3\\. Нажмите \"Связать Telegram\"\n" +
			"4\\. Отправьте мне полученный код\n\n" +
			"Код выглядит примерно так:\n" +
			"`74b55710\\-3293\\-4b89\\-a7aa\\-a31f38282af9`"

		_ = c.tgService.SendMessageEx(ctx, chatID, welcomeMsg, telegram.WithMarkdownV2())
		return nil
	}

	// Если токен указан, пытаемся привязать
	err := c.userService.ConfirmTelegramLink(ctx, token, chatID)
	if err != nil {
		_ = c.tgService.SendMessage(ctx, chatID, "❌ Ошибка\\. Неверный код или истекло время его действия\\. Попробуйте снова\\.")
	} else {
		_ = c.tgService.SendMessage(ctx, chatID, "✅ Ваш аккаунт успешно привязан\\!")
		// Отправляем главное меню с кнопками
		return c.sendMainMenu(ctx, chatID)
	}
	return nil
}

func isUUIDFormat(text string) bool {
	if len(text) != 36 {
		return false
	}
	if text[8] != '-' || text[13] != '-' || text[18] != '-' || text[23] != '-' {
		return false
	}

	hexChars := "0123456789abcdefABCDEF"
	for i, c := range text {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !strings.ContainsRune(hexChars, c) {
			return false
		}
	}

	return true
}

// sendMainMenu отправляет главное меню с постоянными кнопками
func (c *TelegramController) sendMainMenu(ctx context.Context, chatID int64) error {
	if !c.cfg.AdvancedMode {
		return c.tgService.SendMessage(ctx, chatID, "✅ Вы успешно подключены к боту\\!")
	}

	text := "🏠 *Главное меню*\n\n" +
		"Добро пожаловать в систему заявок\\!\n" +
		"Выберите нужное действие из меню ниже\\."

	// ИЗМЕНЕНИЕ: Переносим все основные действия в постоянные кнопки.
	// Это делает интерфейс более интуитивным и быстрым для пользователя.
	replyKeyboard := [][]telegram.ReplyKeyboardButton{
		{{Text: "📋 Мои Заявки"}},
		{{Text: "⏰ На сегодня"}, {Text: "🔴 Просроченные"}},
		{{Text: "🔍 Поиск"}, {Text: "📊 Статистика"}},
	}

	return c.tgService.SendMessageEx(ctx, chatID, text,
		telegram.WithReplyKeyboard(replyKeyboard),
		telegram.WithMarkdownV2(),
	)
}

func (c *TelegramController) handleMyTasksCommand(ctx context.Context, chatID int64, messageID ...int) error {
	_, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	filter := types.Filter{Limit: 50, Page: 1} // Увеличили лимит до 50
	orderListResponse, err := c.orderService.GetOrders(userCtx, filter, true)
	if err != nil {
		c.logger.Error("handleMyTasksCommand: orderService.GetOrders error", zap.Error(err))
		return c.tgService.SendMessage(ctx, chatID, "Произошла ошибка при получении списка ваших заявок.")
	}

	var text strings.Builder
	var keyboardRows [][]telegram.InlineKeyboardButton

	if len(orderListResponse.List) == 0 {
		text.WriteString("✅ У вас нет активных заявок.")
	} else {
		text.WriteString(fmt.Sprintf("📋 *Ваши заявки* \\(%d\\):\n\n", len(orderListResponse.List)))

		statusesMap := make(map[uint64]*entities.Status)
		allStatuses, err := c.statusRepo.FindAll(ctx)
		if err != nil {
			c.logger.Error("handleMyTasksCommand: не удалось получить все статусы", zap.Error(err))
			// Если не можем получить статусы, работаем без эмодзи, чтобы не падать
		} else {
			for i := range allStatuses {
				statusesMap[allStatuses[i].ID] = &allStatuses[i]
			}
		}

		// 2. Формируем компактный список (только номера и статус)
		for _, order := range orderListResponse.List {
			// 3. Получаем статус и соответствующий ему эмодзи
			var statusEmoji string
			if status, ok := statusesMap[order.StatusID]; ok {
				statusEmoji = getStatusEmoji(status)
			} else {
				statusEmoji = "🔵" // Эмодзи по умолчанию, если статус не найден
			}

			text.WriteString(fmt.Sprintf("%s *№%d* • %s\n",
				statusEmoji,
				order.ID,
				telegram.EscapeTextForMarkdownV2(order.Name),
			))
		}

		text.WriteString("\n_Выберите заявку:_")

		// ✅ КОМПАКТНЫЕ КНОПКИ: 5 колонок
		currentRow := []telegram.InlineKeyboardButton{}
		for _, order := range orderListResponse.List {
			buttonText := fmt.Sprintf("№%d", order.ID)
			callbackData := fmt.Sprintf(`{"action":"select_order","order_id":%d}`, order.ID)

			currentRow = append(currentRow, telegram.InlineKeyboardButton{
				Text:         buttonText,
				CallbackData: callbackData,
			})

			// Когда набралось 5 кнопок в ряду, добавляем ряд и начинаем новый
			if len(currentRow) == 5 {
				keyboardRows = append(keyboardRows, currentRow)
				currentRow = []telegram.InlineKeyboardButton{}
			}
		}

		// Добавляем оставшиеся кнопки (если меньше 5)
		if len(currentRow) > 0 {
			keyboardRows = append(keyboardRows, currentRow)
		}
	}

	var msgIDToEdit int
	if len(messageID) > 0 {
		msgIDToEdit = messageID[0]
	}

	return c.tgService.EditOrSendMessage(ctx, chatID, msgIDToEdit, text.String(),
		telegram.WithKeyboard(keyboardRows),
		telegram.WithMarkdownV2(),
	)
}

// В файле: internal/controllers/telegram_controller.go
func (c *TelegramController) handleTextMessage(ctx context.Context, chatID int64, text string) error {
	// 1. Проверяем, не является ли это токеном привязки (UUID формат)
	text = strings.TrimSpace(text)

	if isUUIDFormat(text) {
		err := c.userService.ConfirmTelegramLink(ctx, text, chatID)
		if err != nil {
			_ = c.tgService.SendMessage(ctx, chatID, "❌ Неверный код или истекло время его действия\\. Попробуйте получить новый код на сайте\\.")
		} else {
			_ = c.tgService.SendMessage(ctx, chatID, "✅ Ваш аккаунт успешно привязан\\!")
			return c.sendMainMenu(ctx, chatID)
		}
		return nil
	}

	if text == "📊 Статистика" {
		if !c.deduplicator.TryAcquire(chatID, "stats_button") {
			return nil
		}
		defer c.deduplicator.Release(chatID, "stats_button")

		return c.handleStatsCommand(ctx, chatID)
	}

	// ИЗМЕНЕНИЕ: Добавляем обработку новых постоянных кнопок.
	if text == "⏰ На сегодня" {
		return c.handleTodayTasksCommand(ctx, chatID)
	}
	if text == "🔴 Просроченные" {
		return c.handleOverdueTasksCommand(ctx, chatID)
	}
	if text == "🔍 Поиск" {
		return c.handleSearchStart(ctx, chatID, 0) // 0 т.к. нет сообщения для редактирования
	}

	if text == "📋 Мои Заявки" {
		if !c.deduplicator.TryAcquire(chatID, "my_tasks_button") {
			return nil
		}
		defer c.deduplicator.Release(chatID, "my_tasks_button")

		return c.handleMyTasksCommand(ctx, chatID)
	}

	state, err := c.getUserState(ctx, chatID)
	if err != nil || state == nil {
		return nil
	}

	// 3. Определяем, что делать с текстом в зависимости от режима
	switch state.Mode {
	case "awaiting_comment":
		return c.handleSetComment(ctx, chatID, text)
	case "awaiting_duration":
		return c.handleSetDuration(ctx, chatID, text)
	case "awaiting_executor":
		return c.handleSetExecutorFromText(ctx, chatID, text)
	case "awaiting_search":
		return c.handleSearchQuery(ctx, chatID, text)
	}

	return nil
}

func (c *TelegramController) handleTodayTasksCommand(ctx context.Context, chatID int64, messageID ...int) error {
	_, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	// Получаем заявки со сроком на сегодня
	now := time.Now().In(c.loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, c.loc)
	endOfDay := startOfDay.Add(24 * time.Hour)

	filter := types.Filter{
		Limit: 50,
		Page:  1,
		Filter: map[string]interface{}{
			"duration_from": startOfDay,
			"duration_to":   endOfDay,
		},
	}

	orderListResponse, err := c.orderService.GetOrders(userCtx, filter, true)
	if err != nil {
		c.logger.Error("handleTodayTasksCommand: orderService.GetOrders error", zap.Error(err))
		return c.tgService.SendMessage(ctx, chatID, "❌ Произошла ошибка при получении заявок\\.")
	}

	var text strings.Builder
	var keyboardRows [][]telegram.InlineKeyboardButton

	if len(orderListResponse.List) == 0 {
		text.WriteString("✅ *Заявок на сегодня нет\\!*\n\n")
		text.WriteString("_Можете отдохнуть_ 😊")
	} else {
		text.WriteString(fmt.Sprintf("⏰ *Заявки на сегодня* \\(%d\\):\n\n", len(orderListResponse.List)))

		// Получаем статусы для эмодзи
		statusesMap := make(map[uint64]*entities.Status)
		allStatuses, err := c.statusRepo.FindAll(ctx)
		if err == nil {
			for i := range allStatuses {
				statusesMap[allStatuses[i].ID] = &allStatuses[i]
			}
		}

		// Формируем список
		for _, order := range orderListResponse.List {
			var statusEmoji string
			if status, ok := statusesMap[order.StatusID]; ok {
				statusEmoji = getStatusEmoji(status)
			} else {
				statusEmoji = "🔵"
			}

			// Время дедлайна
			timeStr := ""
			if order.Duration != nil {
				timeStr = order.Duration.Format("15:04")
			}

			text.WriteString(fmt.Sprintf("%s *№%d* • %s",
				statusEmoji,
				order.ID,
				telegram.EscapeTextForMarkdownV2(order.Name),
			))
			if timeStr != "" {
				text.WriteString(fmt.Sprintf(" ⏱ _%s_", timeStr))
			}
			text.WriteString("\n")
		}

		text.WriteString("\n_Выберите заявку для подробностей:_")

		// Кнопки для заявок (5 в ряд)
		currentRow := []telegram.InlineKeyboardButton{}
		for _, order := range orderListResponse.List {
			buttonText := fmt.Sprintf("№%d", order.ID)
			callbackData := fmt.Sprintf(`{"action":"select_order","order_id":%d}`, order.ID)

			currentRow = append(currentRow, telegram.InlineKeyboardButton{
				Text:         buttonText,
				CallbackData: callbackData,
			})

			if len(currentRow) == 5 {
				keyboardRows = append(keyboardRows, currentRow)
				currentRow = []telegram.InlineKeyboardButton{}
			}
		}
		if len(currentRow) > 0 {
			keyboardRows = append(keyboardRows, currentRow)
		}
	}

	// Кнопка "Назад"
	keyboardRows = append(keyboardRows, []telegram.InlineKeyboardButton{
		{Text: "🏠 Главное меню", CallbackData: `{"action":"main_menu"}`},
	})

	var msgIDToEdit int
	if len(messageID) > 0 {
		msgIDToEdit = messageID[0]
	}

	return c.tgService.EditOrSendMessage(ctx, chatID, msgIDToEdit, text.String(),
		telegram.WithKeyboard(keyboardRows),
		telegram.WithMarkdownV2(),
	)
}

// handleOverdueTasksCommand - Просроченные заявки
func (c *TelegramController) handleOverdueTasksCommand(ctx context.Context, chatID int64, messageID ...int) error {
	_, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	// Получаем просроченные заявки
	now := time.Now().In(c.loc)

	filter := types.Filter{
		Limit: 50,
		Page:  1,
		Filter: map[string]interface{}{
			"overdue": true, // Предполагаю, что у вас есть такой фильтр
		},
	}

	orderListResponse, err := c.orderService.GetOrders(userCtx, filter, true)
	if err != nil {
		c.logger.Error("handleOverdueTasksCommand: orderService.GetOrders error", zap.Error(err))
		return c.tgService.SendMessage(ctx, chatID, "❌ Произошла ошибка при получении заявок\\.")
	}

	var overdueOrders []dto.OrderResponseDTO
	for _, order := range orderListResponse.List {
		if order.Duration != nil && order.Duration.Before(now) {

			status, err := c.statusRepo.FindStatus(ctx, order.StatusID)
			if err == nil && status.Code != nil && *status.Code != "CLOSED" && *status.Code != "REJECTED" {
				overdueOrders = append(overdueOrders, order)
			}
		}
	}

	var text strings.Builder
	var keyboardRows [][]telegram.InlineKeyboardButton

	if len(overdueOrders) == 0 {
		text.WriteString("✅ *Просроченных заявок нет\\!*\n\n")
		text.WriteString("_Отличная работа_ 👍")
	} else {
		text.WriteString(fmt.Sprintf("🔴 *Просроченные заявки* \\(%d\\):\n\n", len(overdueOrders)))
		text.WriteString("⚠️ _Требуют срочного внимания\\!_\n\n")

		// Получаем статусы для эмодзи
		statusesMap := make(map[uint64]*entities.Status)
		allStatuses, err := c.statusRepo.FindAll(ctx)
		if err == nil {
			for i := range allStatuses {
				statusesMap[allStatuses[i].ID] = &allStatuses[i]
			}
		}

		// Формируем список
		for _, order := range overdueOrders {
			var statusEmoji string
			if status, ok := statusesMap[order.StatusID]; ok {
				statusEmoji = getStatusEmoji(status)
			} else {
				statusEmoji = "🔵"
			}

			// Вычисляем, насколько просрочено
			overdueDuration := now.Sub(*order.Duration)
			overdueStr := ""
			if overdueDuration.Hours() >= 24 {
				days := int(overdueDuration.Hours() / 24)
				overdueStr = fmt.Sprintf("\\(%d дн\\.", days)
			} else {
				hours := int(overdueDuration.Hours())
				overdueStr = fmt.Sprintf("\\(%dч", hours)
			}

			text.WriteString(fmt.Sprintf("%s *№%d* • %s 🔴_%s назад_\n",
				statusEmoji,
				order.ID,
				telegram.EscapeTextForMarkdownV2(order.Name),
				overdueStr,
			))
		}

		text.WriteString("\n_Выберите заявку:_")

		// Кнопки для заявок (5 в ряд)
		currentRow := []telegram.InlineKeyboardButton{}
		for _, order := range overdueOrders {
			buttonText := fmt.Sprintf("№%d", order.ID)
			callbackData := fmt.Sprintf(`{"action":"select_order","order_id":%d}`, order.ID)

			currentRow = append(currentRow, telegram.InlineKeyboardButton{
				Text:         buttonText,
				CallbackData: callbackData,
			})

			if len(currentRow) == 5 {
				keyboardRows = append(keyboardRows, currentRow)
				currentRow = []telegram.InlineKeyboardButton{}
			}
		}
		if len(currentRow) > 0 {
			keyboardRows = append(keyboardRows, currentRow)
		}
	}

	// Кнопка "Назад"
	keyboardRows = append(keyboardRows, []telegram.InlineKeyboardButton{
		{Text: "🏠 Главное меню", CallbackData: `{"action":"main_menu"}`},
	})

	var msgIDToEdit int
	if len(messageID) > 0 {
		msgIDToEdit = messageID[0]
	}

	return c.tgService.EditOrSendMessage(ctx, chatID, msgIDToEdit, text.String(),
		telegram.WithKeyboard(keyboardRows),
		telegram.WithMarkdownV2(),
	)
}

// handleSearchStart - Начало поиска
func (c *TelegramController) handleSearchStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		// Создаём новое состояние
		state = &dto.TelegramState{
			Mode:      "awaiting_search",
			MessageID: messageID,
			Changes:   make(map[string]string),
		}
	} else {
		state.Mode = "awaiting_search"
		state.MessageID = messageID
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
		telegram.WithKeyboard(keyboard),
		telegram.WithMarkdownV2(),
	)
}

// handleSearchQuery - Обработка поискового запроса
func (c *TelegramController) handleSearchQuery(ctx context.Context, chatID int64, text string) error {
	_, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return c.tgService.SendMessage(ctx, chatID, "❌ Поисковый запрос не может быть пустым\\.")
	}

	// Очищаем состояние
	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))

	// Пытаемся найти по номеру
	var orderID uint64
	if _, err := fmt.Sscanf(text, "%d", &orderID); err == nil {
		// ИСПРАВЛЕНИЕ: Получаем userID из контекста, а не используем 0.
		userID, _ := utils.GetUserIDFromCtx(userCtx)
		// Это номер заявки
		order, err := c.orderService.FindOrderByIDForTelegram(userCtx, userID, orderID)
		if err == nil {
			return c.sendEditMenu(ctx, chatID, 0, order)
		}
	}

	// Поиск по тексту
	// ИСПРАВЛЕНИЕ: Помещаем `text` в поле `Search`, а не в `Filter`.
	filter := types.Filter{
		Limit:  20,
		Page:   1,
		Search: text,
	}

	orderListResponse, err := c.orderService.GetOrders(userCtx, filter, true)
	if err != nil {
		c.logger.Error("handleSearchQuery: orderService.GetOrders error", zap.Error(err))
		return c.tgService.SendMessage(ctx, chatID, "❌ Произошла ошибка при поиске\\.")
	}

	if len(orderListResponse.List) == 0 {
		return c.tgService.SendMessage(ctx, chatID, fmt.Sprintf("❌ По запросу `%s` ничего не найдено\\.", telegram.EscapeTextForMarkdownV2(text)))
	}

	// Показываем результаты
	var resultText strings.Builder
	resultText.WriteString(fmt.Sprintf("🔍 *Результаты поиска* \\(%d\\):\n\n", len(orderListResponse.List)))

	// Получаем статусы
	statusesMap := make(map[uint64]*entities.Status)
	allStatuses, err := c.statusRepo.FindAll(ctx)
	if err == nil {
		for i := range allStatuses {
			statusesMap[allStatuses[i].ID] = &allStatuses[i]
		}
	}

	var keyboardRows [][]telegram.InlineKeyboardButton
	for _, order := range orderListResponse.List {
		var statusEmoji string
		if status, ok := statusesMap[order.StatusID]; ok {
			statusEmoji = getStatusEmoji(status)
		} else {
			statusEmoji = "🔵"
		}

		resultText.WriteString(fmt.Sprintf("%s *№%d* • %s\n",
			statusEmoji,
			order.ID,
			telegram.EscapeTextForMarkdownV2(order.Name),
		))
	}

	resultText.WriteString("\n_Выберите заявку:_")

	// Кнопки
	currentRow := []telegram.InlineKeyboardButton{}
	for _, order := range orderListResponse.List {
		buttonText := fmt.Sprintf("№%d", order.ID)
		callbackData := fmt.Sprintf(`{"action":"select_order","order_id":%d}`, order.ID)

		currentRow = append(currentRow, telegram.InlineKeyboardButton{
			Text:         buttonText,
			CallbackData: callbackData,
		})

		if len(currentRow) == 5 {
			keyboardRows = append(keyboardRows, currentRow)
			currentRow = []telegram.InlineKeyboardButton{}
		}
	}
	if len(currentRow) > 0 {
		keyboardRows = append(keyboardRows, currentRow)
	}

	keyboardRows = append(keyboardRows, []telegram.InlineKeyboardButton{
		{Text: "🏠 Главное меню", CallbackData: `{"action":"main_menu"}`},
	})

	return c.tgService.SendMessageEx(ctx, chatID, resultText.String(),
		telegram.WithKeyboard(keyboardRows),
		telegram.WithMarkdownV2(),
	)
}

func (c *TelegramController) handleSetExecutorFromText(ctx context.Context, chatID int64, text string) error {
	// Поиск пользователя по ФИО
	users, _, err := c.userRepo.GetUsers(ctx, types.Filter{Filter: map[string]interface{}{"fio_like": text}})
	if err != nil || len(users) == 0 {
		_ = c.tgService.SendMessage(ctx, chatID, "Не найдено пользователей по запросу.")
		return nil
	}
	if len(users) > 1 {
		// Если несколько, показать клавиатуру для выбора
		var keyboardRows [][]telegram.InlineKeyboardButton
		for _, user := range users {
			callbackData := fmt.Sprintf(`{"action":"set_executor","user_id":%d}`, user.ID)
			keyboardRows = append(keyboardRows, []telegram.InlineKeyboardButton{{Text: user.Fio, CallbackData: callbackData}})
		}
		return c.tgService.SendMessageEx(ctx, chatID, "Выберите пользователя:", telegram.WithKeyboard(keyboardRows))
	}

	return c.handleSetSomething(ctx, chatID, "executor_id", users[0].ID, "✅ Исполнитель назначен!")
}

func (c *TelegramController) handleCallbackQuery(ctx context.Context, query *TelegramCallbackQuery) error {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(query.Data), &data); err != nil {
		c.logger.Error("handleCallbackQuery: не удалось распарсить callback data", zap.String("data", query.Data))
		return nil
	}

	action, _ := data["action"].(string)
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID

	switch action {
	// ✅ ДОБАВЬТЕ ЭТИ НОВЫЕ ОБРАБОТЧИКИ:
	case "main_menu":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.sendMainMenu(ctx, chatID)

	case "today_tasks":
		return c.handleTodayTasksCommand(ctx, chatID, messageID)

	case "overdue_tasks":
		return c.handleOverdueTasksCommand(ctx, chatID, messageID)

	case "search_start":
		return c.handleSearchStart(ctx, chatID, messageID)

	// СУЩЕСТВУЮЩИЕ ОБРАБОТЧИКИ (не меняйте):
	case "show_my_tasks":
		return c.handleMyTasksCommand(ctx, chatID)
	case "select_order":
		orderID, _ := data["order_id"].(float64)
		return c.handleSelectOrderAction(ctx, chatID, messageID, uint64(orderID))
	case "edit_cancel":
		_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.handleMyTasksCommand(ctx, chatID, messageID)
	case "edit_save":
		return c.handleSaveChanges(ctx, chatID, messageID)

	case "edit_status_start":
		return c.handleEditStatusStart(ctx, chatID, messageID)
	case "set_status":
		statusID, _ := data["status_id"].(float64)
		return c.handleSetSomething(ctx, chatID, "status_id", uint64(statusID), "✅ Статус обновлен!")

	case "edit_duration_start":
		return c.handleEditDurationStart(ctx, chatID, messageID)
	case "set_duration":
		durationStr, _ := data["value"].(string)
		return c.handleSetDuration(ctx, chatID, durationStr)

	case "edit_comment_start":
		return c.handleEditCommentStart(ctx, chatID, messageID)

	case "edit_delegate_start":
		return c.handleDelegateStart(ctx, chatID, messageID)
	case "set_executor":
		executorID, _ := data["user_id"].(float64)
		return c.handleSetSomething(ctx, chatID, "executor_id", uint64(executorID), "✅ Исполнитель назначен!")

	default:
		c.logger.Warn("handleCallbackQuery: получен неизвестный action", zap.String("action", action))
	}
	return nil
}

func (c *TelegramController) handleSelectOrderAction(ctx context.Context, chatID int64, messageID int, orderID uint64) error {
	user, _, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	order, err := c.orderService.FindOrderByIDForTelegram(ctx, user.ID, orderID)
	if err != nil {
		if errors.Is(err, apperrors.ErrForbidden) {
			_ = c.tgService.AnswerCallbackQuery(ctx, "", "⛔ У вас нет прав на редактирование этой заявки.")
			return nil
		}
		c.logger.Error("handleSelectOrderAction: не удалось найти заявку", zap.Error(err))
		_ = c.tgService.AnswerCallbackQuery(ctx, "", "❌ Ошибка: заявка не найдена.")
		return nil
	}

	state := dto.NewTelegramState(orderID, messageID)
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	return c.sendEditMenu(ctx, chatID, messageID, order)
}

func (c *TelegramController) handleEditStatusStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}
	state.Mode = "awaiting_new_status"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	// 1. Получаем текущую заявку
	user, err := c.userService.FindUserByTelegramChatID(ctx, chatID)
	if err != nil {
		c.logger.Error("handleEditStatusStart: не удалось найти пользователя", zap.Error(err))
		return c.sendInternalError(ctx, chatID)
	}
	order, err := c.orderService.FindOrderByIDForTelegram(ctx, user.ID, state.OrderID)
	if err != nil {
		c.logger.Error("handleEditStatusStart: не удалось найти заявку", zap.Error(err))
		return c.sendInternalError(ctx, chatID)
	}

	currentStatus, err := c.statusRepo.FindStatus(ctx, order.StatusID)
	if err != nil {
		c.logger.Error("handleEditStatusStart: не удалось получить текущий статус", zap.Error(err))
		return c.sendInternalError(ctx, chatID)
	}

	allStatuses, err := c.statusRepo.FindAll(ctx)
	if err != nil {
		return c.tgService.EditMessageText(ctx, chatID, messageID, "❌ Не удалось загрузить список статусов.")
	}

	// ✅ БЛОКИРУЕМ НЕНУЖНЫЕ СТАТУСЫ ДЛЯ ТЕЛЕГРАМ-БОТА
	blockedStatusCodes := map[string]bool{
		"ACTIVE":   true, // Активный (не нужен для заявок)
		"INACTIVE": true, // Неактивный (не нужен для заявок)
		"OPEN":     true, // Открыто (автоматически при создании)
	}

	var allowedStatuses []entities.Status

	// 2. Логика выбора доступных статусов в зависимости от текущего
	if currentStatus != nil && currentStatus.Code != nil {
		switch *currentStatus.Code {
		case "COMPLETED":
			// Если заявка "Выполнена", можно только:
			// - CLOSED (Закрыть) - принять работу
			// - REFINEMENT (Доработка) - отправить на доработку
			for _, s := range allStatuses {
				if s.Code != nil && (*s.Code == "CLOSED" || *s.Code == "REFINEMENT") {
					allowedStatuses = append(allowedStatuses, s)
				}
			}

		case "CLOSED":
			// Если заявка "Закрыта", статус менять нельзя
			// Но эта ситуация не должна возникнуть, т.к. в sendEditMenu мы блокируем редактирование
			_ = c.tgService.AnswerCallbackQuery(ctx, "", "Закрытую заявку нельзя редактировать.")
			return nil

		default:
			// Для всех остальных статусов показываем все доступные, кроме:
			// - текущего статуса
			// - заблокированных (ACTIVE, INACTIVE, OPEN)
			// - CLOSED (закрыть можно только из COMPLETED)
			for _, s := range allStatuses {
				// Пропускаем текущий статус
				if s.ID == order.StatusID {
					continue
				}

				// Пропускаем заблокированные статусы
				if s.Code != nil && blockedStatusCodes[*s.Code] {
					continue
				}

				// Пропускаем CLOSED (закрыть можно только из COMPLETED)
				if s.Code != nil && *s.Code == "CLOSED" {
					continue
				}

				allowedStatuses = append(allowedStatuses, s)
			}
		}
	} else {
		// Если по какой-то причине не определили код статуса, показываем все кроме заблокированных
		for _, s := range allStatuses {
			if s.ID == order.StatusID {
				continue
			}
			if s.Code != nil && blockedStatusCodes[*s.Code] {
				continue
			}
			allowedStatuses = append(allowedStatuses, s)
		}
	}

	c.logger.Debug("Allowed statuses for order",
		zap.Uint64("orderID", order.ID),
		zap.String("currentStatus", func() string {
			if currentStatus != nil && currentStatus.Code != nil {
				return *currentStatus.Code
			}
			return "unknown"
		}()),
		zap.Int("allowedCount", len(allowedStatuses)),
	)

	// 3. Проверка: есть ли доступные статусы
	if len(allowedStatuses) == 0 {
		_ = c.tgService.AnswerCallbackQuery(ctx, "", "Нет доступных статусов для смены.")
		return nil
	}

	// 4. Формирование кнопок для Telegram (2 кнопки в ряд)
	var keyboardRows [][]telegram.InlineKeyboardButton
	currentRow := []telegram.InlineKeyboardButton{}

	for _, status := range allowedStatuses {
		callbackData := fmt.Sprintf(`{"action":"set_status","status_id":%d}`, status.ID)
		currentRow = append(currentRow, telegram.InlineKeyboardButton{
			Text:         status.Name,
			CallbackData: callbackData,
		})

		if len(currentRow) == 2 {
			keyboardRows = append(keyboardRows, currentRow)
			currentRow = []telegram.InlineKeyboardButton{}
		}
	}

	// Добавляем оставшиеся кнопки (если нечетное количество)
	if len(currentRow) > 0 {
		keyboardRows = append(keyboardRows, currentRow)
	}

	// Кнопка "Назад"
	keyboardRows = append(keyboardRows, []telegram.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: fmt.Sprintf(`{"action":"select_order","order_id":%d}`, state.OrderID)},
	})

	return c.tgService.EditMessageText(ctx, chatID, messageID, "Выберите новый статус:", telegram.WithKeyboard(keyboardRows))
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

	var keyboardRows [][]telegram.InlineKeyboardButton
	row := []telegram.InlineKeyboardButton{}

	now := time.Now().In(c.loc)

	for _, qd := range quickDurations {

		futureTime := now.Add(qd.Duration)

		futureTime = futureTime.Round(30 * time.Minute)

		callbackValue := futureTime.Format("02.01.2006 15:04")
		buttonText := fmt.Sprintf("%s (%s)", qd.Label, futureTime.Format("02.01 15:04"))

		row = append(row, telegram.InlineKeyboardButton{Text: buttonText, CallbackData: fmt.Sprintf(`{"action":"set_duration","value":"%s"}`, callbackValue)})

		if len(row) == 2 {
			keyboardRows = append(keyboardRows, row)
			row = []telegram.InlineKeyboardButton{}
		}
	}

	if len(row) > 0 {
		keyboardRows = append(keyboardRows, row)
	}

	keyboardRows = append(keyboardRows, []telegram.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: fmt.Sprintf(`{"action":"select_order","order_id":%d}`, state.OrderID)},
	})

	text := "Выберите срок или отправьте его текстом в формате `ДД.ММ.ГГГГ ЧЧ:ММ`"
	return c.tgService.EditMessageText(ctx, chatID, messageID, text, telegram.WithKeyboard(keyboardRows), telegram.WithMarkdownV2())
}

func (c *TelegramController) handleSetDuration(ctx context.Context, chatID int64, text string) error {
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
			_ = c.tgService.SendMessage(ctx, chatID, "❌ Неверный формат даты. Попробуйте `ДД.ММ.ГГГГ ЧЧ:ММ`.")
			return nil
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
	text := "Введите ваш комментарий:"
	keyboard := [][]telegram.InlineKeyboardButton{
		{{Text: "◀️ Назад", CallbackData: fmt.Sprintf(`{"action":"select_order","order_id":%d}`, state.OrderID)}},
	}
	return c.tgService.EditMessageText(ctx, chatID, messageID, text,
		telegram.WithKeyboard(keyboard),
		telegram.WithMarkdownV2(),
	)
}

func (c *TelegramController) handleSetComment(ctx context.Context, chatID int64, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		_ = c.tgService.SendMessage(ctx, chatID, "❌ Комментарий не может быть пустым.")
		return nil
	}
	return c.handleSetSomething(ctx, chatID, "comment", text, "✅ Комментарий добавлен!")
}

func (c *TelegramController) handleDelegateStart(ctx context.Context, chatID int64, messageID int) error {
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}
	state.Mode = "awaiting_executor"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	user, err := c.userService.FindUserByTelegramChatID(ctx, chatID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	order, err := c.orderService.FindOrderByIDForTelegram(ctx, user.ID, state.OrderID)
	if err != nil {
		c.logger.Error("handleDelegateStart: не удалось найти заявку", zap.Error(err))
		return c.sendInternalError(ctx, chatID)
	}

	userFilter := types.Filter{Filter: make(map[string]interface{}), WithPagination: false}

	if order.DepartmentID != nil && *order.DepartmentID > 0 {
		userFilter.Filter["department_id"] = *order.DepartmentID
	}
	if order.OtdelID != nil {
		userFilter.Filter["otdel_id"] = *order.OtdelID
	}
	if order.BranchID != nil {
		userFilter.Filter["branch_id"] = *order.BranchID
	}
	if order.OfficeID != nil {
		userFilter.Filter["office_id"] = *order.OfficeID
	}
	users, _, err := c.userRepo.GetUsers(ctx, userFilter)
	if err != nil || len(users) == 0 {
		text := "Не найдено коллег в подразделении этой заявки. Введите ФИО сотрудника для поиска."
		return c.tgService.EditMessageText(ctx, chatID, messageID, text)
	}

	var keyboardRows [][]telegram.InlineKeyboardButton
	for _, user := range users {
		callbackData := fmt.Sprintf(`{"action":"set_executor","user_id":%d}`, user.ID)
		keyboardRows = append(keyboardRows, []telegram.InlineKeyboardButton{{Text: user.Fio, CallbackData: callbackData}})
	}
	keyboardRows = append(keyboardRows, []telegram.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: fmt.Sprintf(`{"action":"select_order","order_id":%d}`, state.OrderID)},
	})
	text := "Выберите нового исполнителя:"
	return c.tgService.EditMessageText(ctx, chatID, messageID, text, telegram.WithKeyboard(keyboardRows))
}

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
			c.logger.Error("handleSetSomething: неверный тип для status_id", zap.Any("value", value))
			return c.sendInternalError(ctx, chatID)
		}

	case "executor_id":
		if id, ok := value.(uint64); ok {
			state.SetExecutorID(id)
		} else if idFloat, ok := value.(float64); ok {
			state.SetExecutorID(uint64(idFloat))
		} else {
			c.logger.Error("handleSetSomething: неверный тип для executor_id", zap.Any("value", value))
			return c.sendInternalError(ctx, chatID)
		}

	case "comment":
		if comment, ok := value.(string); ok {
			state.SetComment(comment)
		} else {
			c.logger.Error("handleSetSomething: неверный тип для comment", zap.Any("value", value))
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
			c.logger.Error("handleSetSomething: неверный тип для duration", zap.Any("value", value))
			return c.sendInternalError(ctx, chatID)
		}

	default:
		c.logger.Error("handleSetSomething: неизвестный ключ", zap.String("key", key))
		return c.sendInternalError(ctx, chatID)
	}

	state.Mode = "editing_order"
	if err := c.setUserState(ctx, chatID, state); err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	_ = c.tgService.AnswerCallbackQuery(ctx, "", popupText)

	user, err := c.userService.FindUserByTelegramChatID(ctx, chatID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}
	order, err := c.orderService.FindOrderByIDForTelegram(ctx, user.ID, state.OrderID)
	if err != nil {
		c.logger.Error("handleSetSomething: не удалось получить заявку для обновления меню", zap.Error(err))
		return c.tgService.EditMessageText(ctx, chatID, state.MessageID, "❌ Ошибка при обновлении меню: заявка не найдена или доступ запрещен.")
	}

	return c.sendEditMenu(ctx, chatID, state.MessageID, order)
}

// --- Шаг Финал: Сохранение ---
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
		_ = c.tgService.AnswerCallbackQuery(ctx, "", "Вы не внесли никаких изменений.")
		return nil
	}

	// Собираем DTO для обновления (ИСПОЛЬЗУЕМ УКАЗАТЕЛИ, БЕЗ NULL-типов)
	updateDTO := dto.UpdateOrderDTO{}

	// StatusID
	if statusID, exists, err := state.GetStatusID(); err != nil {
		c.logger.Error("handleSaveChanges: ошибка парсинга status_id", zap.Error(err))
		return c.tgService.EditMessageText(ctx, chatID, messageID, "❌ Ошибка обработки статуса.")
	} else if exists {
		updateDTO.StatusID = &statusID
	}

	// ExecutorID
	if executorID, exists, err := state.GetExecutorID(); err != nil {
		c.logger.Error("handleSaveChanges: ошибка парсинга executor_id", zap.Error(err))
		return c.tgService.EditMessageText(ctx, chatID, messageID, "❌ Ошибка обработки исполнителя.")
	} else if exists {
		updateDTO.ExecutorID = &executorID
	}

	// Comment
	if comment, exists := state.GetComment(); exists {
		// Копируем значение, чтобы взять его адрес
		commentVal := comment
		updateDTO.Comment = &commentVal
	}

	// Duration
	duration, err := state.GetDuration()
	if err != nil {
		c.logger.Error("handleSaveChanges: ошибка парсинга duration", zap.Error(err))
		return c.tgService.EditMessageText(ctx, chatID, messageID, "❌ Ошибка обработки срока.")
	}

	if duration != nil {
		// Если есть новая дата — ставим указатель
		updateDTO.Duration = duration
	} else if _, exists := state.Changes["duration"]; exists {
		// Если было "очищение" даты: отправляем zero time
		// (Это компромисс новой системы обновлений)
		zeroTime := time.Time{}
		updateDTO.Duration = &zeroTime
	}

	// Вызываем сервис для обновления
	_, err = c.orderService.UpdateOrder(userCtx, state.OrderID, updateDTO, nil)
	if err != nil {
		c.logger.Error("handleSaveChanges: ошибка при обновлении заявки", zap.Error(err))
		return c.tgService.EditMessageText(ctx, chatID, messageID, "❌ Ошибка при сохранении. Попробуйте еще раз.")
	}
	c.logger.Info("Заявка успешно обновлена через Telegram",
		zap.Uint64("orderID", state.OrderID),
		zap.Int64("chatID", chatID),
		zap.Any("changes", state.Changes),
	)
	// Очищаем состояние
	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
	_ = c.tgService.AnswerCallbackQuery(ctx, "", "💾 Изменения сохранены!")

	return c.handleMyTasksCommand(ctx, chatID, messageID)
}

func (c *TelegramController) prepareUserContext(ctx context.Context, chatID int64) (*entities.User, context.Context, error) {
	user, err := c.userService.FindUserByTelegramChatID(ctx, chatID)
	if err != nil {
		_ = c.tgService.SendMessage(ctx, chatID, "Не удалось найти ваш аккаунт. Пожалуйста, привяжите его на сайте.")
		return nil, nil, err
	}

	userCtx := context.WithValue(ctx, contextkeys.UserIDKey, user.ID)
	permissions, err := c.authPermissionService.GetAllUserPermissions(userCtx, user.ID)
	if err != nil {
		_ = c.tgService.SendMessage(ctx, chatID, "Произошла ошибка при проверке ваших прав доступа.")
		return nil, nil, err
	}
	permissionsMap := make(map[string]bool)
	for _, p := range permissions {
		permissionsMap[p] = true
	}
	userCtx = context.WithValue(userCtx, contextkeys.UserPermissionsMapKey, permissionsMap)
	return user, userCtx, nil
}

func (c *TelegramController) sendEditMenu(ctx context.Context, chatID int64, messageID int, order *entities.Order) error {
	// Получаем детальную информацию
	status, err := c.statusRepo.FindStatus(ctx, order.StatusID)
	if err != nil {
		c.logger.Error("sendEditMenu: не удалось получить статус", zap.Error(err))
		return c.sendInternalError(ctx, chatID)
	}

	// Получаем информацию о создателе
	creator, err := c.userRepo.FindUserByID(ctx, order.CreatorID)
	if err != nil {
		c.logger.Warn("sendEditMenu: не удалось получить создателя", zap.Error(err))
	}

	// Получаем информацию об исполнителе
	var executor *entities.User
	if order.ExecutorID != nil {
		executor, err = c.userRepo.FindUserByID(ctx, *order.ExecutorID)
		if err != nil {
			c.logger.Warn("sendEditMenu: не удалось получить исполнителя", zap.Error(err))
		}
	}

	// Получаем последний комментарий из истории
	lastComment := ""
	historyItems, err := c.orderHistoryRepo.GetOrderHistory(ctx, order.ID, types.Filter{Limit: 10, Page: 1})
	if err == nil && len(historyItems) > 0 {
		// Ищем последний комментарий
		for _, item := range historyItems {
			if item.EventType == "COMMENT" && item.Comment.Valid && item.Comment.String != "" {
				lastComment = item.Comment.String
				break
			}
		}
	}

	// Формируем красивое сообщение
	var text strings.Builder

	text.WriteString(fmt.Sprintf("📋 *Заявка №%d*\n", order.ID))
	text.WriteString(fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━\n\n"))

	// Название
	text.WriteString(fmt.Sprintf("📝 *Описание:*\n%s\n\n",
		telegram.EscapeTextForMarkdownV2(order.Name),
	))

	// Статус
	statusEmoji := getStatusEmoji(status)
	text.WriteString(fmt.Sprintf("%s *Статус:* %s\n",
		statusEmoji,
		telegram.EscapeTextForMarkdownV2(status.Name),
	))

	// Создатель
	if creator != nil {
		text.WriteString(fmt.Sprintf("👤 *Создатель:* %s\n",
			telegram.EscapeTextForMarkdownV2(creator.Fio),
		))
	}

	// Исполнитель
	if executor != nil {
		text.WriteString(fmt.Sprintf("👨‍💼 *Исполнитель:* %s\n",
			telegram.EscapeTextForMarkdownV2(executor.Fio),
		))
	} else {
		text.WriteString("👨‍💼 *Исполнитель:* _не назначен_\n")
	}

	// Срок
	if order.Duration != nil {
		durationStr := order.Duration.Format("02.01.2006 15:04")

		// Проверяем, просрочена ли заявка
		now := time.Now()
		if order.Duration.Before(now) {
			text.WriteString(fmt.Sprintf("⏰ *Срок:* ~%s~ ⚠️ _просрочено_\n",
				telegram.EscapeTextForMarkdownV2(durationStr),
			))
		} else {
			text.WriteString(fmt.Sprintf("⏰ *Срок:* %s\n",
				telegram.EscapeTextForMarkdownV2(durationStr),
			))
		}
	} else {
		text.WriteString("⏰ *Срок:* _не задан_\n")
	}

	// Адрес (если есть)
	if order.Address != nil && *order.Address != "" {
		text.WriteString(fmt.Sprintf("📍 *Адрес:* %s\n",
			telegram.EscapeTextForMarkdownV2(*order.Address),
		))
	}

	// Дата создания
	createdAt := order.CreatedAt.Format("02.01.2006 15:04")
	text.WriteString(fmt.Sprintf("📅 *Создана:* %s\n",
		telegram.EscapeTextForMarkdownV2(createdAt),
	))

	// Последний комментарий
	if lastComment != "" {
		// Обрезаем комментарий, если он слишком длинный
		if len(lastComment) > 100 {
			lastComment = lastComment[:100] + "..."
		}
		text.WriteString(fmt.Sprintf("\n💬 *Последний комментарий:*\n_%s_\n",
			telegram.EscapeTextForMarkdownV2(lastComment),
		))
	}

	text.WriteString("\n━━━━━━━━━━━━━━━━━━━━\n")

	// Кнопки управления
	var keyboardRows [][]telegram.InlineKeyboardButton

	// Единственная проверка: если статус "Закрыто", то показываем только кнопку "Назад".
	if status.Code != nil && *status.Code == "CLOSED" {
		text.WriteString("\n🔒 *Заявка закрыта\\.*\n_Редактирование недоступно\\._")
		keyboardRows = append(keyboardRows, []telegram.InlineKeyboardButton{
			{Text: "◀️ К списку заявок", CallbackData: `{"action":"edit_cancel"}`},
		})
	} else {
		// Во всех остальных случаях (включая "Выполнено", "Отклонено") - показываем полное меню
		text.WriteString("\n_Выберите действие:_")
		keyboardRows = [][]telegram.InlineKeyboardButton{
			{{Text: "🔄 Статус", CallbackData: `{"action":"edit_status_start"}`}, {Text: "⏰ Срок", CallbackData: `{"action":"edit_duration_start"}`}},
			{{Text: "💬 Комментарий", CallbackData: `{"action":"edit_comment_start"}`}, {Text: "👤 Делегировать", CallbackData: `{"action":"edit_delegate_start"}`}},
			{{Text: "✅ Сохранить", CallbackData: `{"action":"edit_save"}`}, {Text: "◀️ Назад", CallbackData: `{"action":"edit_cancel"}`}},
		}
	}

	return c.tgService.EditMessageText(ctx, chatID, messageID, text.String(),
		telegram.WithKeyboard(keyboardRows),
		telegram.WithMarkdownV2(),
	)
}

// Вспомогательная функция для эмодзи статусов
func getStatusEmoji(status *entities.Status) string {
	if status == nil || status.Code == nil {
		return "🔷" // неизвестно / по умолчанию
	}

	switch *status.Code {
	case "OPEN":
		return "❗" // Открыто (требует внимания)
	case "IN_PROGRESS":
		return "⏳" // В работе
	case "REFINEMENT":
		return "🔺" // Доработка
	case "CLARIFICATION":
		return "❓" // Уточнение
	case "COMPLETED":
		return "🆗" // Выполнено исполнителем (ждет приёмки)
	case "CLOSED":
		return "✔️" // Принято заявителем (окончательно)
	case "REJECTED":
		return "❌" // Отклонено
	case "CONFIRMED":
		return "🔀" // Перенаправлено (не моя зона)
	case "SERVICE":
		return "🛠️" // Сервис
	default:
		return "🔷" // По умолчанию
	}
}

// -- Хелперы для работы с состоянием в Redis --
func (c *TelegramController) getUserState(ctx context.Context, chatID int64) (*dto.TelegramState, error) {
	stateJSON, err := c.cacheRepo.Get(ctx, fmt.Sprintf(telegramStateKey, chatID))
	if err != nil || stateJSON == "" {
		return nil, errors.New("state not found")
	}

	state, err := dto.FromJSON(stateJSON)
	if err != nil {
		c.logger.Error("getUserState: не удалось десериализовать состояние", zap.Error(err))
		return nil, err
	}

	return state, nil
}

func (c *TelegramController) setUserState(ctx context.Context, chatID int64, state *dto.TelegramState) error {
	stateJSON, err := state.ToJSON()
	if err != nil {
		c.logger.Error("setUserState: не удалось сериализовать состояние", zap.Error(err))
		return err
	}

	err = c.cacheRepo.Set(ctx, fmt.Sprintf(telegramStateKey, chatID), stateJSON, 15*time.Minute)
	if err != nil {
		c.logger.Error("setUserState: не удалось сохранить состояние в Redis", zap.Error(err))
	}
	return err
}

// -- Хелперы для отправки сообщений об ошибках --
func (c *TelegramController) sendInternalError(ctx context.Context, chatID int64) error {
	return c.tgService.SendMessage(ctx, chatID, "Произошла внутренняя ошибка. Попробуйте позже.")
}

func (c *TelegramController) sendStaleStateError(ctx context.Context, chatID int64, messageID int) error {
	return c.tgService.EditMessageText(ctx, chatID, messageID, "❌ Истекло время сессии редактирования. Начните заново с /my_tasks.")
}

// === Системные функции (генерация токена, регистрация вебхука) ===
// (Без изменений, копируем из вашего исходного кода)

func (c *TelegramController) HandleGenerateLinkToken(ctx echo.Context) error {
	token, err := c.userService.GenerateTelegramLinkToken(ctx.Request().Context())
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, map[string]string{"token": token}, "Токен для привязки сгенерирован", http.StatusOK)
}

func (c *TelegramController) RegisterWebhook(baseURL string) error {
	webhookURL := fmt.Sprintf("%s/api/webhooks/telegram", baseURL)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook?url=%s", c.botToken, webhookURL)

	resp, err := http.Get(apiURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ошибка регистрации вебхука: %s", string(body))
	}
	c.logger.Info("Telegram Webhook успешно зарегистрирован", zap.String("url", webhookURL))
	return nil
}

func (c *TelegramController) StartCleanup(ctx context.Context) {
	if c.deduplicator != nil {
		c.logger.Info("Запуск фоновой очистки дедупликатора...")
		c.deduplicator.Cleanup(ctx, 1*time.Minute)
		c.logger.Info("Фоновая очистка дедупликатора остановлена")
	}
}

// -- Вспомогательные структуры (остаются без изменений) --
type TelegramUpdate struct {
	UpdateID      int                    `json:"update_id"`
	Message       *TelegramMessage       `json:"message"`
	CallbackQuery *TelegramCallbackQuery `json:"callback_query"`
}
type TelegramMessage struct {
	MessageID int          `json:"message_id"`
	From      TelegramUser `json:"from"`
	Chat      TelegramChat `json:"chat"`
	Text      string       `json:"text"`
}
type TelegramUser struct {
	ID int64 `json:"id"`
}
type TelegramChat struct {
	ID int64 `json:"id"`
}
type TelegramCallbackQuery struct {
	ID      string           `json:"id"`
	From    TelegramUser     `json:"from"`
	Message *TelegramMessage `json:"message"`
	Data    string           `json:"data"`
}

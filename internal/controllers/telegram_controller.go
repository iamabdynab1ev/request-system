package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"crypto/tls"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/contextkeys"
	"request-system/pkg/telegram"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

const telegramStateKey = "tg_user_state:%d"

type TelegramController struct {
	repoMutex sync.RWMutex // Protects repository operations to prevent race conditions

	userService           services.UserServiceInterface
	orderService          services.OrderServiceInterface
	statusRepo            repositories.StatusRepositoryInterface
	userRepo              repositories.UserRepositoryInterface
	orderHistoryRepo      repositories.OrderHistoryRepositoryInterface
	tgService             telegram.ServiceInterface
	cacheRepo             repositories.CacheRepositoryInterface
	authPermissionService services.AuthPermissionServiceInterface

	// Используем наш дедупликатор из соседнего файла
	deduplicator *RequestDeduplicator

	botToken string
	logger   *zap.Logger
	cfg      config.TelegramConfig
	loc      *time.Location
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
	// Настройка часового пояса (Душанбе)
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
		deduplicator:          NewRequestDeduplicator(),
		botToken:              botToken,
		logger:                logger,
		cfg:                   cfg,
		loc:                   loc,
	}
}

func (c *TelegramController) HandleTelegramWebhook(ctx echo.Context) error {
	var update TelegramUpdate
	if err := ctx.Bind(&update); err != nil {
		c.logger.Error("Ошибка парсинга Telegram update", zap.Error(err))
		return ctx.NoContent(http.StatusOK) // Возвращаем OK, чтобы не зациклить ошибку
	}

	// === ЗАЩИТА ОТ ЛАВИНЫ (Если сервер лежал) ===
	// Игнорируем сообщения старше 60 секунд.
	const maxMessageAge = 60 * time.Second

	var msgDate int64 = 0
	if update.Message != nil {
		msgDate = update.Message.Date
	} else if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
		msgDate = update.CallbackQuery.Message.Date
	}

	if msgDate > 0 {
		msgTime := time.Unix(msgDate, 0)
		if time.Since(msgTime) > maxMessageAge {
			// Логируем (Warn), но не обрабатываем. Телеграму отдаем 200 OK.
			c.logger.Warn("Пропущено старое сообщение (сброс очереди)", zap.Int64("msg_ts", msgDate))
			return ctx.NoContent(http.StatusOK)
		}
	}

	// 1. Обработка кнопок (Callback)
	if update.CallbackQuery != nil {
		if !c.cfg.AdvancedMode {
			return ctx.NoContent(http.StatusOK)
		}

		chatID := update.CallbackQuery.Message.Chat.ID

		// ANTI-SPAM: Кулдаун 1 секунда для кнопок
		if !c.deduplicator.TryAcquire(chatID, "cb", 1*time.Second) {
			// Чтобы колесико загрузки исчезло, ответим пустышкой
			go c.tgService.AnswerCallbackQuery(context.Background(), update.CallbackQuery.ID, "")
			return ctx.NoContent(http.StatusOK)
		}

		go func() {
			bgCtx := context.Background()
			// Всегда отвечаем на колбэк
			_ = c.tgService.AnswerCallbackQuery(bgCtx, update.CallbackQuery.ID, "")
			if err := c.handleCallbackQuery(bgCtx, update.CallbackQuery); err != nil {
				c.logger.Error("Ошибка обработки callback", zap.Error(err))
			}
		}()
	}

	// 2. Обработка текста
	if update.Message != nil {
		chatID := update.Message.Chat.ID
		text := update.Message.Text

		// Логируем только важное
		// c.logger.Info("Сообщение", zap.Int64("chatID", chatID), zap.String("text", text))

		go func(msg *TelegramMessage) {
			bgCtx := context.Background()

			// Команды
			if strings.HasPrefix(text, "/") {
				// Кулдаун для команд побольше (2 сек)
				if !c.deduplicator.TryAcquire(chatID, "cmd", 2*time.Second) {
					return
				}

				if strings.HasPrefix(text, "/start") {
					_ = c.handleStartCommand(bgCtx, chatID, text)
					return
				}
				if c.cfg.AdvancedMode {
					if strings.HasPrefix(text, "/my_tasks") {
						_ = c.handleMyTasksCommand(bgCtx, chatID)
						return
					}
					if strings.HasPrefix(text, "/stats") {
						_ = c.handleStatsCommand(bgCtx, chatID)
						return
					}
				}
				return
			}

			// Кнопки меню и текст
			if c.cfg.AdvancedMode {
				// Кулдаун на меню (чтобы не спамили списком заявок)
				if text == "📋 Мои Заявки" || text == "⏰ На сегодня" || text == "🔴 Просроченные" {
					if !c.deduplicator.TryAcquire(chatID, "menu", 1500*time.Millisecond) {
						return
					}
				}

				if text == "📋 Мои Заявки" {
					_ = c.handleMyTasksCommand(bgCtx, chatID)
					return
				}

				_ = c.handleTextMessage(bgCtx, chatID, text)
			}
		}(update.Message)
	}

	return ctx.NoContent(http.StatusOK)
}

func (c *TelegramController) handleStatsCommand(ctx context.Context, chatID int64, messageID ...int) error {
	user, _, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	stats, err := c.orderService.GetUserStats(ctx, user.ID)
	if err != nil {
		return c.tgService.SendMessage(ctx, chatID, "❌ Ошибка получения статистики.")
	}

	avgHours := int(stats.AvgResolutionSeconds / 3600)
	avgMinutes := int((stats.AvgResolutionSeconds - float64(avgHours*3600)) / 60)

	var t strings.Builder
	t.WriteString("📊 *Ваша статистика*\n━━━━━━━━━━━━━━━━━━━━\n\n")
	t.WriteString(fmt.Sprintf("📝 *Всего:* %d\n", stats.TotalCount))
	t.WriteString(fmt.Sprintf("⚙️ В работе: %d\n", stats.InProgressCount))
	t.WriteString(fmt.Sprintf("✅ Готово: %d\n\n", stats.CompletedCount))

	if avgHours > 0 || avgMinutes > 0 {
		t.WriteString(fmt.Sprintf("⏱️ Среднее время: %dч %dмин\n", avgHours, avgMinutes))
	}

	mid := 0
	if len(messageID) > 0 {
		mid = messageID[0]
	}
	return c.tgService.EditOrSendMessage(ctx, chatID, mid, t.String(), telegram.WithMarkdownV2())
}

// 2. СТАРТ / ПРИВЯЗКА
func (c *TelegramController) handleStartCommand(ctx context.Context, chatID int64, text string) error {
	// 1. Проверяем, есть ли токен в команде (Deep Linking)
	// Текст приходит в формате "/start 123e4567-e89b-12d3-a456-426614174000"
	parts := strings.Fields(text)

	if len(parts) > 1 {
		token := parts[1]
		// Проверяем формат, чтобы не пытаться парсить мусор
		if isUUIDFormat(token) {
			// Пытаемся ПЕРЕПРИВЯЗАТЬ аккаунт прямо сейчас
			err := c.userService.ConfirmTelegramLink(ctx, token, chatID)
			if err != nil {
				// Если код просрочен или неверен
				_ = c.tgService.SendMessage(ctx, chatID, "❌ Ошибка. Код неверный или устарел.")
				// Не выходим, даем пользователю увидеть текущее меню, если он был залогинен
			} else {
				// УСПЕХ!
				// 1. Чистим кеш состояний (чтобы не осталось старых диалогов редактирования)
				_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))

				// 2. Отправляем сообщение об успехе
				_ = c.tgService.SendMessage(ctx, chatID, "✅ Аккаунт успешно обновлен! Теперь вы получаете уведомления для нового пользователя.")

				// 3. Показываем главное меню для НОВОГО пользователя
				return c.sendMainMenu(ctx, chatID)
			}
		}
	}

	// 2. Если токена нет (просто нажали /start)
	user, _, err := c.prepareUserContext(ctx, chatID)

	if err == nil {
		// --- Пользователь УЖЕ привязан ---

		// Формируем сообщение, которое объясняет, как сменить аккаунт
		msg := fmt.Sprintf("👤 Вы авторизованы как: *%s*\n\n"+
			"🔹 Чтобы продолжить работу, используйте меню ниже\\.\n"+
			"🔹 *Чтобы сменить аккаунт:* просто отправьте новый код\\-токен в этот чат\\.",
			telegram.EscapeTextForMarkdownV2(user.Fio))

		_ = c.tgService.SendMessageEx(ctx, chatID, msg, telegram.WithMarkdownV2())
		return c.sendMainMenu(ctx, chatID)
	}

	// --- Пользователь НЕ привязан ---
	welcomeMsg := "👋 *Добро пожаловать в систему заявок\\!*\n\n" +
		"Для привязки вашего аккаунта отправьте мне код, полученный на сайте в профиле."

	return c.tgService.SendMessageEx(ctx, chatID, welcomeMsg, telegram.WithMarkdownV2())
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

	filter := types.Filter{Limit: 20, Page: 1}
	resp, err := c.orderService.GetOrders(userCtx, filter, true) // true = "Мои заявки"
	if err != nil {
		c.logger.Error("Telegram: GetOrders failed", zap.Error(err))
		return c.tgService.SendMessage(ctx, chatID, "Ошибка загрузки данных.")
	}

	c.logger.Info("Telegram: Заявок для пользователя найдено", zap.Int("count", len(resp.List)), zap.Int64("chat_id", chatID))

	var text strings.Builder
	var kbr [][]telegram.InlineKeyboardButton

	if len(resp.List) == 0 {
		text.WriteString("✅ У вас нет активных заявок.")
	} else {
		text.WriteString(fmt.Sprintf("📋 *Ваши заявки* \\(%d\\):\n\n", len(resp.List)))

		c.repoMutex.RLock()
		sMap := make(map[uint64]*entities.Status)
		allS, _ := c.statusRepo.FindAll(ctx)
		for i := range allS {
			sMap[allS[i].ID] = &allS[i]
		}
		c.repoMutex.RUnlock()

		row := []telegram.InlineKeyboardButton{}

		for _, o := range resp.List {
			emo := getStatusEmoji(sMap[o.StatusID])
			text.WriteString(fmt.Sprintf("%s *№%d* • %s\n", emo, o.ID, telegram.EscapeTextForMarkdownV2(o.Name)))

			// Кнопка (ИСПОЛЬЗУЕМ "action":"select_order")
			cb := fmt.Sprintf(`{"action":"select_order","order_id":%d}`, o.ID)
			row = append(row, telegram.InlineKeyboardButton{Text: fmt.Sprintf("№%d", o.ID), CallbackData: cb})

			if len(row) >= 5 {
				kbr = append(kbr, row)
				row = []telegram.InlineKeyboardButton{}
			}
		}
		if len(row) > 0 {
			kbr = append(kbr, row)
		}

		text.WriteString("\n_Выберите заявку:_")

		mid := 0
		if len(messageID) > 0 {
			mid = messageID[0]
		}

		return c.tgService.EditOrSendMessage(ctx, chatID, mid, text.String(), telegram.WithKeyboard(kbr), telegram.WithMarkdownV2())
	}

	// Если список был пуст - просто отправляем текст.
	mid := 0
	if len(messageID) > 0 {
		mid = messageID[0]
	}
	return c.tgService.EditOrSendMessage(ctx, chatID, mid, text.String(), telegram.WithMarkdownV2())
}

// В файле: internal/controllers/telegram_controller.go
func (c *TelegramController) handleTextMessage(ctx context.Context, chatID int64, text string) error {
	text = strings.TrimSpace(text)

	// ШАГ 1: Проверяем, не является ли текст токеном. Это самая важная проверка.
	if isUUIDFormat(text) {
		err := c.userService.ConfirmTelegramLink(ctx, text, chatID)
		if err != nil {
			_ = c.tgService.SendMessage(ctx, chatID, "❌ Ошибка. Код неверный или устарел.")
		} else {
			_ = c.tgService.SendMessage(ctx, chatID, "✅ Аккаунт привязан!")
			return c.sendMainMenu(ctx, chatID)
		}
		return nil
	}

	// ШАГ 2: Обрабатываем кнопки меню (если текст - не токен)
	if text == "📋 Мои Заявки" {
		return c.handleMyTasksCommand(ctx, chatID)
	}
	if text == "⏰ На сегодня" {
		return c.handleTodayTasksCommand(ctx, chatID)
	}
	if text == "🔴 Просроченные" {
		return c.handleOverdueTasksCommand(ctx, chatID)
	}
	if text == "📊 Статистика" {
		return c.handleStatsCommand(ctx, chatID)
	}
	if text == "🔍 Поиск" {
		return c.handleSearchStart(ctx, chatID, 0)
	}

	state, err := c.getUserState(ctx, chatID)
	if err != nil || state == nil {
		return nil
	}

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
		c.repoMutex.RLock()
		statusesMap := make(map[uint64]*entities.Status)
		allStatuses, err := c.statusRepo.FindAll(ctx)
		if err == nil {
			for i := range allStatuses {
				statusesMap[allStatuses[i].ID] = &allStatuses[i]
			}
		}
		c.repoMutex.RUnlock()

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
	// Не игнорируем, а логируем ошибку парсинга
	if err := json.Unmarshal([]byte(query.Data), &data); err != nil {
		c.logger.Error("Telegram: Неверный формат callback data", zap.String("data", query.Data), zap.Error(err))
		return nil
	}

	action, _ := data["action"].(string)
	chatID := query.Message.Chat.ID
	msgID := query.Message.MessageID

	switch action {
	// ---- Управление меню ----
	case "main_menu":
		c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
		return c.sendMainMenu(ctx, chatID)
	case "show_my_tasks":
		return c.handleMyTasksCommand(ctx, chatID, msgID)

	// ---- Выбор заявки ----
	case "sel", "select_order":
		var orderID uint64
		if idFloat, ok := data["order_id"].(float64); ok {
			orderID = uint64(idFloat)
		} else if idFloat, ok := data["id"].(float64); ok {
			orderID = uint64(idFloat)
		}
		return c.handleSelectOrderAction(ctx, chatID, msgID, orderID)

	// ---- Действия в меню редактирования ----
	case "edit_cancel":
		c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
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
		c.logger.Warn("Telegram: Неизвестный action в callback", zap.String("action", action))
	}
	return nil
}

func (c *TelegramController) handleSelectOrderAction(ctx context.Context, chatID int64, mid int, orderID uint64) error {
	u, _, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	order, err := c.orderService.FindOrderByIDForTelegram(ctx, u.ID, orderID)
	if err != nil {
		c.tgService.AnswerCallbackQuery(ctx, "", "Заявка не найдена или нет доступа.")
		return nil
	}

	state := dto.NewTelegramState(orderID, mid)
	c.setUserState(ctx, chatID, state)
	return c.sendEditMenu(ctx, chatID, mid, order)
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

	user, err := c.userService.FindUserByTelegramChatID(ctx, chatID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	order, err := c.orderService.FindOrderByIDForTelegram(ctx, user.ID, state.OrderID)
	if err != nil {
		return c.sendInternalError(ctx, chatID)
	}

	filter := types.Filter{Filter: make(map[string]interface{}), WithPagination: false}
	if order.DepartmentID != nil {
		filter.Filter["department_id"] = *order.DepartmentID
	}
	if order.OtdelID != nil {
		filter.Filter["otdel_id"] = *order.OtdelID
	}
	if order.BranchID != nil {
		filter.Filter["branch_id"] = *order.BranchID
	}
	if order.OfficeID != nil {
		filter.Filter["office_id"] = *order.OfficeID
	}

	users, _, _ := c.userRepo.GetUsers(ctx, filter)

	text := "Выберите нового исполнителя:"
	var keyboardRows [][]telegram.InlineKeyboardButton

	if len(users) == 0 {
		text = "Не найдено коллег в подразделении этой заявки.\n\nВведите ФИО сотрудника для глобального поиска:"
	} else {
		for _, u := range users {
			cb := fmt.Sprintf(`{"action":"set_executor","user_id":%d}`, u.ID)
			keyboardRows = append(keyboardRows, []telegram.InlineKeyboardButton{{Text: u.Fio, CallbackData: cb}})
		}
	}

	keyboardRows = append(keyboardRows, []telegram.InlineKeyboardButton{
		{Text: "◀️ Назад", CallbackData: fmt.Sprintf(`{"action":"select_order","order_id":%d}`, state.OrderID)},
	})

	state.Mode = "awaiting_executor"
	_ = c.setUserState(ctx, chatID, state)

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
// handleSaveChanges собирает изменения из State, формирует Map для SmartUpdate и вызывает сервис
func (c *TelegramController) handleSaveChanges(ctx context.Context, chatID int64, messageID int) error {
	// 1. Подготовка контекста пользователя (права доступа)
	_, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	// 2. Получение состояния редактирования из Redis
	state, err := c.getUserState(ctx, chatID)
	if err != nil {
		return c.sendStaleStateError(ctx, chatID, messageID)
	}

	// 3. Проверка: были ли вообще изменения
	if !state.HasChanges() {
		_ = c.tgService.AnswerCallbackQuery(ctx, "", "Вы не внесли изменений.")
		return nil
	}

	// Получить текущую заявку для сравнения
	currentOrder, err := c.orderService.FindOrderByID(ctx, state.OrderID)
	if err != nil {
		c.logger.Error("handleSaveChanges: не удалось получить текущую заявку", zap.Error(err))
		return c.tgService.EditMessageText(ctx, chatID, messageID, "❌ Ошибка при получении данных заявки.")
	}

	// 4. Сборка DTO и Карты изменений (для SmartUpdate)
	updateDTO := dto.UpdateOrderDTO{}
	changesMap := make(map[string]interface{})

	// -- Статус (ТОЛЬКО ЕСЛИ ИЗМЕНИЛСЯ) --
	if sid, exists, _ := state.GetStatusID(); exists && currentOrder.StatusID != sid {
		updateDTO.StatusID = &sid
		changesMap["status_id"] = sid
	}

	// -- Исполнитель (ТОЛЬКО ЕСЛИ ИЗМЕНИЛСЯ) --
	if eid, exists, _ := state.GetExecutorID(); exists && (currentOrder.ExecutorID == nil || *currentOrder.ExecutorID != eid) {
		updateDTO.ExecutorID = &eid
		changesMap["executor_id"] = eid
	}

	// -- Комментарий --
	if com, exists := state.GetComment(); exists && strings.TrimSpace(com) != "" {
		v := com
		updateDTO.Comment = &v
	}

	// -- Срок (Duration) (ТОЛЬКО ЕСЛИ ИЗМЕНИЛСЯ) --
	dur, _ := state.GetDuration()
	if dur != nil && (currentOrder.Duration == nil || !currentOrder.Duration.Equal(*dur)) {
		// Установлена новая дата
		updateDTO.Duration = dur
		changesMap["duration"] = dur
	} else if _, exists := state.Changes["duration"]; exists && currentOrder.Duration != nil {
		// Если ключ есть в changes -> пользователь нажал "Очистить"
		changesMap["duration"] = nil
		zeroTime := time.Time{}
		updateDTO.Duration = &zeroTime
	}

	// 5. Вызов сервиса
	// 4-й аргумент (файл) = nil
	// 5-й аргумент (explicitFields) = changesMap <--- ВАЖНО для обнуления полей
	_, err = c.orderService.UpdateOrder(userCtx, state.OrderID, updateDTO, nil, changesMap)
	if err != nil {
		c.logger.Error("handleSaveChanges: ошибка сохранения", zap.Error(err))
		return c.tgService.EditMessageText(ctx, chatID, messageID, "❌ Ошибка при сохранении данных.")
	}

	// 6. Очистка и уведомление
	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
	_ = c.tgService.AnswerCallbackQuery(ctx, "", "💾 Сохранено!")

	// Возвращаем пользователя к просмотру (или в список, если хочешь handleMyTasksCommand)
	return c.handleMyTasksCommand(ctx, chatID, messageID)
}

func (c *TelegramController) prepareUserContext(ctx context.Context, chatID int64) (*entities.User, context.Context, error) {
	user, err := c.userService.FindUserByTelegramChatID(ctx, chatID)
	if err != nil {
		c.tgService.SendMessage(ctx, chatID, "Аккаунт не привязан. Используйте /start <код>.")
		return nil, nil, err
	}

	// Контекст с правами
	uc := context.WithValue(ctx, contextkeys.UserIDKey, user.ID)
	perms, _ := c.authPermissionService.GetAllUserPermissions(uc, user.ID)
	pm := make(map[string]bool)
	for _, p := range perms {
		pm[p] = true
	}
	uc = context.WithValue(uc, contextkeys.UserPermissionsMapKey, pm)
	return user, uc, nil
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
// thread-safe cache operations
func (c *TelegramController) getUserState(ctx context.Context, chatID int64) (*dto.TelegramState, error) {
	c.repoMutex.RLock()
	defer c.repoMutex.RUnlock()

	stateJSON, err := c.cacheRepo.Get(ctx, fmt.Sprintf(telegramStateKey, chatID))
	if err != nil || stateJSON == "" {
		return nil, errors.New("no state")
	}
	return dto.FromJSON(stateJSON)
}

func (c *TelegramController) setUserState(ctx context.Context, chatID int64, state *dto.TelegramState) error {
	c.repoMutex.RLock()
	defer c.repoMutex.RUnlock()

	js, _ := state.ToJSON()
	return c.cacheRepo.Set(ctx, fmt.Sprintf(telegramStateKey, chatID), js, 15*time.Minute)
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
	cleanBaseURL := strings.TrimSuffix(baseURL, "/")
	webhookURL := fmt.Sprintf("%s/api/webhooks/telegram", cleanBaseURL)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook?url=%s", c.botToken, webhookURL)

	c.logger.Info("Регистрация вебхука в Telegram (через Proxy)...", zap.String("url", webhookURL))

	tr := &http.Transport{
		Proxy:           http.ProxyFromEnvironment, // Берет настройки из os.Setenv (192.168.10.42:3128)
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr, Timeout: time.Second * 15}

	// СОЗДАЕМ ЗАПРОС ВРУЧНУЮ ДЛЯ ДОБАВЛЕНИЯ ЗАГОЛОВКОВ
	req, _ := http.NewRequest("GET", apiURL, nil)

	// !!! САМОЕ ВАЖНОЕ: ПРИКИДЫВАЕМСЯ БРАУЗЕРОМ CHROME !!!
	// Многие банковские фильтры не пропускают стандартный Go-клиент
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка запроса (возможно, Proxy банка блокирует Telegram): %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("отказ сервера (Код: %d). Ответ: %s", resp.StatusCode, string(body))
	}

	c.logger.Info("✅ ТЕЛЕГРАМ-БОТ ПОДКЛЮЧЕН УСПЕШНО")
	return nil

}

func (c *TelegramController) StartCleanup(ctx context.Context) {
	if c.deduplicator != nil {
		c.logger.Info("Запуск фоновой очистки дедупликатора...")
		c.deduplicator.Cleanup(ctx, 1*time.Minute)
		c.logger.Info("Фоновая очистка дедупликатора остановлена")
	}
}

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
	Date      int64        `json:"date"`
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

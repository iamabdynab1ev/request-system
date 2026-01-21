// internal/controllers/telegram/controller.go
package telegram

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"  // Добавлено: импорт для entities
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/contextkeys"
	"request-system/pkg/telegram"
	"request-system/pkg/utils"  // Добавлено: импорт для utils
)

const (
	telegramStateKey     = "tg_user_state:%d"
	maxMessageAgeSeconds = 60
	commandCooldown      = 2 * time.Second
	callbackCooldown     = 1 * time.Second
	menuCooldown         = 1500 * time.Millisecond
	stateExpiration      = 15 * time.Minute
	goroutineTimeout     = 30 * time.Second
	maxCommentLength     = 500
	maxSearchQueryLength = 100
	maxDateInFutureDays  = 365
	maxOrdersPerPage     = 10
	maxHistoryItems      = 1
)

type TelegramController struct {
	repoMutex        sync.RWMutex // Защита операций с репозиторием
	userService      services.UserServiceInterface
	orderService     services.OrderServiceInterface
	statusRepo       repositories.StatusRepositoryInterface
	userRepo         repositories.UserRepositoryInterface
	orderHistoryRepo repositories.OrderHistoryRepositoryInterface
	tgService        telegram.ServiceInterface
	cacheRepo        repositories.CacheRepositoryInterface
	authPermissionService services.AuthPermissionServiceInterface
	deduplicator     *RequestDeduplicator
	botToken         string
	logger           *zap.Logger
	cfg              config.TelegramConfig
	loc              *time.Location
	// Кеш для часто используемых данных
	statusCache      map[uint64]*entities.Status
	statusCacheMutex sync.RWMutex
	statusCacheTime  time.Time
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
		deduplicator:          NewRequestDeduplicator(),
		botToken:              botToken,
		logger:                logger,
		cfg:                   cfg,
		loc:                   loc,
		statusCache:           make(map[uint64]*entities.Status),
	}
}

// ==================== ОСНОВНОЙ ОБРАБОТЧИК WEBHOOK ====================
func (c *TelegramController) HandleTelegramWebhook(ctx echo.Context) error {
	var update TelegramUpdate
	if err := ctx.Bind(&update); err != nil {
		c.logger.Error("Ошибка парсинга Telegram update", zap.Error(err))
		return ctx.NoContent(http.StatusOK)
	}
	// Защита от лавины старых сообщений (если сервер был выключен)
	if !c.isMessageRecent(&update) {
		c.logger.Warn("Пропущено старое сообщение",
			zap.Int("update_id", update.UpdateID))
		return ctx.NoContent(http.StatusOK)
	}
	// Обработка callback кнопок
	if update.CallbackQuery != nil {
		if !c.cfg.AdvancedMode {
			return ctx.NoContent(http.StatusOK)
		}
		chatID := update.CallbackQuery.Message.Chat.ID
		if !c.deduplicator.TryAcquire(chatID, "cb", callbackCooldown) {
			go c.tgService.AnswerCallbackQuery(context.Background(), update.CallbackQuery.ID, "")
			return ctx.NoContent(http.StatusOK)
		}
		go c.handleCallbackQueryAsync(update.CallbackQuery)
	}
	// Обработка текстовых сообщений
	if update.Message != nil {
		go c.handleMessageAsync(update.Message)
	}
	return ctx.NoContent(http.StatusOK)
}

// ==================== АСИНХРОННАЯ ОБРАБОТКА ====================
func (c *TelegramController) handleCallbackQueryAsync(query *TelegramCallbackQuery) {
	defer c.recoverPanic("handleCallbackQueryAsync")
	bgCtx, cancel := context.WithTimeout(context.Background(), goroutineTimeout)
	defer cancel()
	_ = c.tgService.AnswerCallbackQuery(bgCtx, query.ID, "")
	if err := c.handleCallbackQuery(bgCtx, query); err != nil {
		c.logger.Error("Ошибка обработки callback",
			zap.Error(err),
			zap.Int64("chat_id", query.Message.Chat.ID))
	}
}

func (c *TelegramController) handleMessageAsync(msg *TelegramMessage) {
	defer c.recoverPanic("handleMessageAsync")
	bgCtx, cancel := context.WithTimeout(context.Background(), goroutineTimeout)
	defer cancel()
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)
	// Обработка команд
	if strings.HasPrefix(text, "/") {
		if !c.deduplicator.TryAcquire(chatID, "cmd", commandCooldown) {
			return
		}
		c.handleCommand(bgCtx, chatID, text)
		return
	}
	// Обработка текста
	if c.cfg.AdvancedMode {
		// Антиспам для кнопок меню
		if c.isMenuButton(text) {
			if !c.deduplicator.TryAcquire(chatID, "menu", menuCooldown) {
				return
			}
		}
		if err := c.handleTextMessage(bgCtx, chatID, text); err != nil {
			c.logger.Error("Ошибка обработки текста",
				zap.Error(err),
				zap.Int64("chat_id", chatID))
		}
	}
}

// ==================== СЛУЖЕБНЫЕ ФУНКЦИИ ====================
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
	c.repoMutex.Lock()
	defer c.repoMutex.Unlock()
	js, err := state.ToJSON()
	if err != nil {
		c.logger.Error("Ошибка сериализации состояния", zap.Error(err))
		return err
	}
	return c.cacheRepo.Set(ctx, fmt.Sprintf(telegramStateKey, chatID), js, stateExpiration)
}

func (c *TelegramController) isMessageRecent(update *TelegramUpdate) bool {
	// 1. Если это нажатие кнопки (Callback), мы ВСЕГДА считаем его свежим.
	// Потому что Update.CallbackQuery.Message.Date — это дата ОТПРАВКИ сообщения ботом,
	// а не дата нажатия кнопки. Меню может висеть час, и клик по нему валиден.
	if update.CallbackQuery != nil {
		return true
	}

	// 2. Только для новых текстовых сообщений проверяем время, чтобы не отвечать на старый спам
	// если бот лежал выключенным неделю.
	if update.Message != nil {
		msgDate := update.Message.Date
		if msgDate > 0 {
			msgTime := time.Unix(msgDate, 0)
			// Если сообщению больше 2 минут - игнорируем
			if time.Since(msgTime) > 2*time.Minute {
				return false
			}
		}
	}
	
	return true
}
func (c *TelegramController) isMenuButton(text string) bool {
	menuButtons := []string{"📋 Мои Заявки", "⏰ На сегодня", "🔴 Просроченные"}
	for _, btn := range menuButtons {
		if text == btn {
			return true
		}
	}
	return false
}

func (c *TelegramController) recoverPanic(funcName string) {
	if r := recover(); r != nil {
		c.logger.Error("PANIC в горутине",
			zap.String("function", funcName),
			zap.Any("panic", r),
			zap.Stack("stacktrace"))
	}
}

func (c *TelegramController) sendInternalError(ctx context.Context, chatID int64) error {
	return c.tgService.SendMessageEx(ctx, chatID,
		"❌ Внутренняя ошибка\\.\nПопробуйте позже или обратитесь в поддержку\\.",
		telegram.WithMarkdownV2())
}

func (c *TelegramController) sendStaleStateError(ctx context.Context, chatID int64, messageID int) error {
	
	return c.tgService.SendMessageEx(ctx, chatID, 
        "⚠️ Срок действия кнопки истек\\.\nПожалуйста, вызовите меню заново: /my\\_tasks", 
        telegram.WithMarkdownV2())
}

func getStatusEmoji(status *entities.Status) string {
	if status == nil || status.Code == nil {
		return "🔷"
	}
	switch *status.Code {
	case "OPEN":
		return "❗"
	case "IN_PROGRESS":
		return "⏳"
	case "REFINEMENT":
		return "🔺"
	case "CLARIFICATION":
		return "❓"
	case "COMPLETED":
		return "🆗"
	case "CLOSED":
		return "✔️"
	case "REJECTED":
		return "❌"
	case "CONFIRMED":
		return "🔀"
	case "SERVICE":
		return "🛠️"
	default:
		return "🔷"
	}
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

func (c *TelegramController) prepareUserContext(ctx context.Context, chatID int64) (*entities.User, context.Context, error) {
	user, err := c.userService.FindUserByTelegramChatID(ctx, chatID)
	if err != nil {
		_ = c.tgService.SendMessage(ctx, chatID,
			"❌ Аккаунт не привязан\\.\n\nИспользуйте /start для получения инструкций\\.")
		return nil, nil, err
	}
	userCtx := context.WithValue(ctx, contextkeys.UserIDKey, user.ID)
	perms, _ := c.authPermissionService.GetAllUserPermissions(userCtx, user.ID)
	permMap := make(map[string]bool)
	for _, p := range perms {
		permMap[p] = true
	}
	userCtx = context.WithValue(userCtx, contextkeys.UserPermissionsMapKey, permMap)
	return user, userCtx, nil
}

func (c *TelegramController) getStatusMap(ctx context.Context) map[uint64]*entities.Status {
	c.statusCacheMutex.RLock()
	if time.Since(c.statusCacheTime) < 5*time.Minute && len(c.statusCache) > 0 {
		defer c.statusCacheMutex.RUnlock()
		return c.statusCache
	}
	c.statusCacheMutex.RUnlock()
	c.statusCacheMutex.Lock()
	defer c.statusCacheMutex.Unlock()
	statusMap := make(map[uint64]*entities.Status)
	statuses, err := c.statusRepo.FindAll(ctx)
	if err == nil {
		for i := range statuses {
			statusMap[statuses[i].ID] = &statuses[i]
		}
		c.statusCache = statusMap
		c.statusCacheTime = time.Now()
	}
	return statusMap
}

func (c *TelegramController) getAllowedStatuses(ctx context.Context, currentStatus *entities.Status, currentStatusID uint64) []entities.Status {
	allStatuses, err := c.statusRepo.FindAll(ctx)
	if err != nil {
		return nil
	}
	blockedCodes := map[string]bool{
		"ACTIVE":   true,
		"INACTIVE": true,
		"OPEN":     true,
	}
	var allowed []entities.Status
	if currentStatus != nil && currentStatus.Code != nil {
		switch *currentStatus.Code {
		case "COMPLETED":
			for _, s := range allStatuses {
				if s.Code != nil && (*s.Code == "CLOSED" || *s.Code == "REFINEMENT") {
					allowed = append(allowed, s)
				}
			}
		case "CLOSED":
			return nil
		default:
			for _, s := range allStatuses {
				if s.ID == currentStatusID {
					continue
				}
				if s.Code != nil && blockedCodes[*s.Code] {
					continue
				}
				if s.Code != nil && *s.Code == "CLOSED" {
					continue
				}
				allowed = append(allowed, s)
			}
		}
	}
	return allowed
}

// ==================== API ENDPOINTS ====================
func (c *TelegramController) HandleGenerateLinkToken(ctx echo.Context) error {
	token, err := c.userService.GenerateTelegramLinkToken(ctx.Request().Context())
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, map[string]string{"token": token},
		"Токен сгенерирован", http.StatusOK)
}

func (c *TelegramController) RegisterWebhook(baseURL string) error {
	cleanBaseURL := strings.TrimSuffix(baseURL, "/")
	webhookURL := fmt.Sprintf("%s/api/webhooks/telegram", cleanBaseURL)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook?url=%s",
		c.botToken, webhookURL)
	c.logger.Info("Регистрация вебхука Telegram", zap.String("url", webhookURL))
	tr := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr, Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %v", err)
	}
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка запроса (возможно, блокировка прокси): %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("отказ сервера (код: %d). Ответ: %s",
			resp.StatusCode, string(body))
	}
	c.logger.Info("✅ TELEGRAM BOT УСПЕШНО ПОДКЛЮЧЕН")
	return nil
}

func (c *TelegramController) StartCleanup(ctx context.Context) {
	if c.deduplicator != nil {
		c.logger.Info("Запуск фоновой очистки дедупликатора")
		c.deduplicator.Cleanup(ctx, 1*time.Minute)
		c.logger.Info("Фоновая очистка остановлена")
	}
}

// ==================== ТИПЫ ====================
type TelegramUpdate struct {
	UpdateID      int                      `json:"update_id"`
	Message       *TelegramMessage         `json:"message"`
	CallbackQuery *TelegramCallbackQuery   `json:"callback_query"`
}

type TelegramMessage struct {
	MessageID int            `json:"message_id"`
	From      TelegramUser   `json:"from"`
	Chat      TelegramChat   `json:"chat"`
	Text      string         `json:"text"`
	Date      int64          `json:"date"`
}

type TelegramUser struct {
	ID int64 `json:"id"`
}

type TelegramChat struct {
	ID int64 `json:"id"`
}

type TelegramCallbackQuery struct {
	ID      string            `json:"id"`
	From    TelegramUser      `json:"from"`
	Message *TelegramMessage  `json:"message"`
	Data    string            `json:"data"`
}

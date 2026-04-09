// internal/controllers/telegram/controller.go
package telegram

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

	"github.com/jackc/pgx/v5"
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
	"request-system/pkg/utils"
)

const (
	telegramStateKey     = "tg_user_state:%d"
	maxMessageAgeSeconds = 600
	commandCooldown      = 1000 * time.Millisecond // 1 СЃРµРє РјРµР¶РґСѓ РєРѕРјР°РЅРґР°РјРё
	callbackCooldown     = 500 * time.Millisecond  // 0.5 СЃРµРє РјРµР¶РґСѓ РєР»РёРєР°РјРё
	menuCooldown         = 2000 * time.Millisecond // 2 СЃРµРє РґР»СЏ СЃС‚Р°С‚РёСЃС‚РёРєРё Рё РјРµРЅСЋ
	stateExpiration      = 60 * time.Minute        // СЃРѕСЃС‚РѕСЏРЅРёРµ Р¶РёРІС‘С‚ 60 РјРёРЅСѓС‚
	goroutineTimeout     = 45 * time.Second

	maxCommentLength      = 500
	maxSearchQueryLength  = 100
	maxDateInFutureDays   = 365
	maxOrdersPerPage      = 10
	maxConcurrentRequests = 50
)

type telegramContextKey string

const callbackQueryIDContextKey telegramContextKey = "telegram_callback_query_id"
const callbackAnswerStateContextKey telegramContextKey = "telegram_callback_answer_state"

var errTelegramAccountNotLinked = errors.New("telegram account not linked")

type callbackAnswerState struct {
	mu       sync.Mutex
	answered bool
}

type TelegramController struct {
	userService           services.UserServiceInterface
	orderService          services.OrderServiceInterface
	integrationService    services.TelegramIntegrationServiceInterface
	statusRepo            repositories.StatusRepositoryInterface
	userRepo              repositories.UserRepositoryInterface
	orderHistoryRepo      repositories.OrderHistoryRepositoryInterface
	tgService             telegram.ServiceInterface
	cacheRepo             repositories.CacheRepositoryInterface
	authPermissionService services.AuthPermissionServiceInterface
	deduplicator          *RequestDeduplicator
	logger                *zap.Logger
	orderTypeRepo         repositories.OrderTypeRepositoryInterface
	cfg                   config.TelegramConfig
	loc                   *time.Location

	statusCache      map[uint64]*entities.Status
	statusCacheMutex sync.RWMutex
	statusCacheTime  time.Time

	sem chan struct{}
}

func NewTelegramController(
	userService services.UserServiceInterface,
	orderService services.OrderServiceInterface,
	integrationService services.TelegramIntegrationServiceInterface,
	tgService telegram.ServiceInterface,
	cacheRepo repositories.CacheRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	orderHistoryRepo repositories.OrderHistoryRepositoryInterface,
	authPermissionService services.AuthPermissionServiceInterface,
	logger *zap.Logger,
	orderTypeRepo repositories.OrderTypeRepositoryInterface,
	cfg config.TelegramConfig,
) *TelegramController {
	return &TelegramController{
		userService:           userService,
		orderService:          orderService,
		integrationService:    integrationService,
		tgService:             tgService,
		cacheRepo:             cacheRepo,
		statusRepo:            statusRepo,
		userRepo:              userRepo,
		orderHistoryRepo:      orderHistoryRepo,
		authPermissionService: authPermissionService,
		deduplicator:          NewRequestDeduplicator(),
		logger:                logger,
		orderTypeRepo:         orderTypeRepo,
		cfg:                   cfg,
		loc:                   time.Local,
		statusCache:           make(map[uint64]*entities.Status),
		sem:                   make(chan struct{}, maxConcurrentRequests),
	}
}

func (c *TelegramController) HandleTelegramWebhook(ctx echo.Context) error {
	c.logger.Info("Telegram webhook request received",
		zap.String("method", ctx.Request().Method),
		zap.String("remote_addr", ctx.Request().RemoteAddr),
		zap.String("user_agent", ctx.Request().UserAgent()))

	if err := c.integrationService.ValidateWebhookRequest(ctx.Request()); err != nil {
		c.logger.Warn("Telegram webhook rejected", zap.Error(err))
		return ctx.NoContent(http.StatusUnauthorized)
	}

	rawBody, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		c.logger.Error("Telegram webhook read failed", zap.Error(err))
		return ctx.NoContent(http.StatusOK)
	}

	var update TelegramUpdate
	if err := json.Unmarshal(rawBody, &update); err != nil {
		rawSample := string(rawBody)
		if len(rawSample) > 512 {
			rawSample = rawSample[:512]
		}
		c.logger.Warn("Telegram webhook payload decode failed",
			zap.Error(err),
			zap.String("raw_body", rawSample))
		return ctx.NoContent(http.StatusOK)
	}

	if !c.isMessageRecent(&update) {
		if update.Message != nil {
			c.logger.Info("Telegram stale message dropped",
				zap.Int("message_id", update.Message.MessageID),
				zap.Int64("chat_id", update.Message.Chat.ID),
				zap.Int64("message_date", update.Message.Date))
		}
		return ctx.NoContent(http.StatusOK)
	}

	if update.CallbackQuery != nil {
		c.logger.Debug("Telegram callback received",
			zap.String("callback_id", update.CallbackQuery.ID),
			zap.Bool("has_message", update.CallbackQuery.Message != nil))

		if !c.cfg.AdvancedMode {
			return ctx.NoContent(http.StatusOK)
		}

		if update.CallbackQuery.Message == nil {
			go c.tgService.AnswerCallbackQuery(context.Background(), update.CallbackQuery.ID, "")
			return ctx.NoContent(http.StatusOK)
		}

		chatID := update.CallbackQuery.Message.Chat.ID
		if !c.deduplicator.TryAcquire(chatID, "cb", callbackCooldown) {
			go c.tgService.AnswerCallbackQuery(context.Background(), update.CallbackQuery.ID, "")
			return ctx.NoContent(http.StatusOK)
		}

		go c.handleCallbackQueryAsync(update.CallbackQuery)
		return ctx.NoContent(http.StatusOK)
	}

	if update.Message != nil {
		c.logger.Debug("Telegram message received",
			zap.Int("message_id", update.Message.MessageID),
			zap.Int64("chat_id", update.Message.Chat.ID),
			zap.Bool("is_command", strings.HasPrefix(strings.TrimSpace(update.Message.Text), "/")))
		go c.handleMessageAsync(update.Message)
		return ctx.NoContent(http.StatusOK)
	}

	c.logger.Info("Telegram update ignored: unsupported type",
		zap.Int("update_id", update.UpdateID),
		zap.Bool("has_message", update.Message != nil),
		zap.Bool("has_callback_query", update.CallbackQuery != nil))
	return ctx.NoContent(http.StatusOK)
}

// ==================== РЎР›РЈР–Р•Р‘РќР«Р• Р¤РЈРќРљР¦РР ====================
func (c *TelegramController) handleCallbackQueryAsync(query *TelegramCallbackQuery) {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	defer c.recoverPanic("handleCallbackQueryAsync")
	bgCtx := withCallbackQueryState(context.Background(), query.ID)
	bgCtx, cancel := context.WithTimeout(bgCtx, goroutineTimeout)
	defer cancel()

	go c.ensureCallbackAnswered(bgCtx, 1200*time.Millisecond)
	defer func() {
		_ = c.answerCallback(bgCtx, "")
	}()

	if err := c.handleCallbackQuery(bgCtx, query); err != nil {
		if isTelegramAccountNotLinkedError(err) {
			if renderErr := c.renderNotLinkedScreen(bgCtx, query.Message.Chat.ID); renderErr != nil {
				c.logger.Error("Telegram not-linked screen render failed", zap.Error(renderErr))
			}
			return
		}
		c.logger.Error("Callback error", zap.Error(err))
	}
}

func (c *TelegramController) handleMessageAsync(msg *TelegramMessage) {
	defer c.recoverPanic("handleMessageAsync")

	chatID := msg.Chat.ID
	msgID := msg.MessageID
	text := strings.TrimSpace(msg.Text)

	isCommand := strings.HasPrefix(text, "/")
	isMenu := isTelegramMenuButton(text)

	if isCommand {
		if !c.deduplicator.TryAcquire(chatID, "cmd", commandCooldown) {
			go c.tgService.DeleteMessage(context.Background(), chatID, msgID)
			return
		}
	} else if c.cfg.AdvancedMode && isMenu {
		if !c.deduplicator.TryAcquire(chatID, "menu", menuCooldown) {
			go c.tgService.DeleteMessage(context.Background(), chatID, msgID)
			return
		}
	}

	// РЈРґР°Р»СЏРµРј СЃРѕРѕР±С‰РµРЅРёРµ РїРѕР»СЊР·РѕРІР°С‚РµР»СЏ РІ РѕС‚РґРµР»СЊРЅРѕР№ РіРѕСЂСѓС‚РёРЅРµ.
	go func() {
		time.Sleep(500 * time.Millisecond)
		_ = c.tgService.DeleteMessage(context.Background(), chatID, msgID)
	}()

	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	bgCtx, cancel := context.WithTimeout(context.Background(), goroutineTimeout)
	defer cancel()

	if isCommand {
		if err := c.handleCommand(bgCtx, chatID, text); err != nil {
			if isTelegramAccountNotLinkedError(err) {
				if renderErr := c.renderNotLinkedScreen(bgCtx, chatID); renderErr != nil {
					c.logger.Error("Telegram not-linked screen render failed",
						zap.Int64("chat_id", chatID),
						zap.Error(renderErr))
				}
				return
			}
			c.logger.Error("Telegram command failed",
				zap.Int64("chat_id", chatID),
				zap.String("text", text),
				zap.Error(err))
		}
		return
	}

	if isUUIDFormat(text) || isTelegramShortCodeFormat(text) {
		if _, err := c.userService.FindUserByTelegramChatID(bgCtx, chatID); err != nil {
			if err := c.handleTokenLink(bgCtx, chatID, text); err != nil {
				c.logger.Error("Telegram link by plain code failed",
					zap.Int64("chat_id", chatID),
					zap.String("text", text),
					zap.Error(err))
			}
			return
		}
	}

	if c.cfg.AdvancedMode {
		if err := c.handleTextMessage(bgCtx, chatID, text); err != nil {
			if isTelegramAccountNotLinkedError(err) {
				if renderErr := c.renderNotLinkedScreen(bgCtx, chatID); renderErr != nil {
					c.logger.Error("Telegram not-linked screen render failed",
						zap.Int64("chat_id", chatID),
						zap.Error(renderErr))
				}
				return
			}
			c.logger.Error("Text error", zap.Error(err))
		}
	}
}

// ==================== РЎР›РЈР–Р•Р‘РќР«Р• Р¤РЈРќРљР¦РР ====================
func (c *TelegramController) getUserState(ctx context.Context, chatID int64) (*dto.TelegramState, error) {
	stateJSON, err := c.cacheRepo.Get(ctx, fmt.Sprintf(telegramStateKey, chatID))
	if err != nil || stateJSON == "" {
		return nil, errors.New("no state")
	}
	return dto.FromJSON(stateJSON)
}

func (c *TelegramController) setUserState(ctx context.Context, chatID int64, state *dto.TelegramState) error {
	js, err := state.ToJSON()
	if err != nil {
		c.logger.Error("РћС€РёР±РєР° СЃРµСЂРёР°Р»РёР·Р°С†РёРё СЃРѕСЃС‚РѕСЏРЅРёСЏ", zap.Error(err))
		return err
	}
	return c.cacheRepo.Set(ctx, fmt.Sprintf(telegramStateKey, chatID), js, stateExpiration)
}

func (c *TelegramController) isMessageRecent(update *TelegramUpdate) bool {
	// Callback СЃС‡РёС‚Р°РµРј Р°РєС‚СѓР°Р»СЊРЅС‹Рј: РґР°С‚Р° Сѓ РЅРµРіРѕ РѕС‚РЅРѕСЃРёС‚СЃСЏ Рє РёСЃС…РѕРґРЅРѕРјСѓ СЃРѕРѕР±С‰РµРЅРёСЋ Р±РѕС‚Р°.
	if update.CallbackQuery != nil {
		return true
	}

	// Р”Р»СЏ С‚РµРєСЃС‚РѕРІС‹С… СЃРѕРѕР±С‰РµРЅРёР№ РїСЂРѕРІРµСЂСЏРµРј СЃСЂРѕРє РґР°РІРЅРѕСЃС‚Рё.
	if update.Message != nil {
		msgDate := update.Message.Date
		if msgDate > 0 {
			msgTime := time.Unix(msgDate, 0)
			if time.Since(msgTime) > time.Duration(maxMessageAgeSeconds)*time.Second {
				return false
			}
		}
	}

	return true
}

func (c *TelegramController) isMenuButton(text string) bool {
	return isTelegramMenuButton(text)
}
func (c *TelegramController) recoverPanic(funcName string) {
	if r := recover(); r != nil {
		c.logger.Error("PANIC РІ РіРѕСЂСѓС‚РёРЅРµ",
			zap.String("function", funcName),
			zap.Any("panic", r),
			zap.Stack("stacktrace"))
	}
}

func (c *TelegramController) sendInternalError(ctx context.Context, chatID int64) error {
	return c.renderHomeScreen(ctx, chatID, 0,
		"вќЊ Р’РЅСѓС‚СЂРµРЅРЅСЏСЏ РѕС€РёР±РєР°.\nРџРѕРїСЂРѕР±СѓР№С‚Рµ РїРѕР·Р¶Рµ РёР»Рё РѕР±СЂР°С‚РёС‚РµСЃСЊ РІ РїРѕРґРґРµСЂР¶РєСѓ.")
}

func (c *TelegramController) sendStaleStateError(ctx context.Context, chatID int64, messageID int) error {
	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
	return c.renderHomeScreen(ctx, chatID, messageID,
		"вљ пёЏ РЎСЂРѕРє РґРµР№СЃС‚РІРёСЏ РјРµРЅСЋ РёСЃС‚С‘Рє.\nРћС‚РєСЂРѕР№С‚Рµ СЃРїРёСЃРѕРє Р·Р°РЅРѕРІРѕ С‡РµСЂРµР· /menu РёР»Рё РєРЅРѕРїРєРё РЅРёР¶Рµ.")
}

func (c *TelegramController) answerCallback(ctx context.Context, text string) error {
	callbackQueryID := callbackQueryIDFromContext(ctx)
	if callbackQueryID == "" {
		return nil
	}

	if state := callbackAnswerStateFromContext(ctx); state != nil {
		state.mu.Lock()
		if state.answered {
			state.mu.Unlock()
			return nil
		}
		state.answered = true
		state.mu.Unlock()
	}

	return c.tgService.AnswerCallbackQuery(ctx, callbackQueryID, text)
}

func getStatusEmoji(status *entities.Status) string {
	if status == nil || status.Code == nil {
		return "рџ”·"
	}
	switch *status.Code {
	case "OPEN":
		return "вќ—"
	case "IN_PROGRESS":
		return "вЏі"
	case "REFINEMENT":
		return "рџ”Ѓ"
	case "CLARIFICATION":
		return "вќ“"
	case "COMPLETED":
		return "вњ…"
	case "CLOSED":
		return "вњ”пёЏ"
	case "REJECTED":
		return "вќЊ"
	case "CONFIRMED":
		return "рџ”„"
	case "SERVICE":
		return "рџ› пёЏ"
	default:
		return "рџ”·"
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

func isTelegramShortCodeFormat(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) != 6 {
		return false
	}
	for _, ch := range text {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func (c *TelegramController) prepareUserContext(ctx context.Context, chatID int64) (*entities.User, context.Context, error) {
	user, err := c.userService.FindUserByTelegramChatID(ctx, chatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, errTelegramAccountNotLinked
		}
		return nil, nil, err
	}
	userCtx := context.WithValue(ctx, contextkeys.UserIDKey, user.ID)
	perms, _ := c.authPermissionService.GetAllUserPermissions(userCtx, user.ID)
	permMap := make(map[string]bool)
	for _, p := range perms {
		permMap[p] = true
	}
	userCtx = context.WithValue(userCtx, contextkeys.UserPermissionsMapKey, permMap)
	userCtx = context.WithValue(userCtx, contextkeys.UserEntityKey, user)
	return user, userCtx, nil
}

func isTelegramAccountNotLinkedError(err error) bool {
	return errors.Is(err, errTelegramAccountNotLinked)
}

func (c *TelegramController) renderNotLinkedScreen(ctx context.Context, chatID int64) error {
	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))
	return c.renderScreen(
		ctx,
		chatID,
		0,
		"❌ *Аккаунт не привязан*\n\nИспользуйте /start для получения инструкций\\.",
		telegram.WithMarkdownV2(),
	)
}

func (c *TelegramController) handlePrepareUserContextError(ctx context.Context, chatID int64, err error) error {
	if isTelegramAccountNotLinkedError(err) {
		return err
	}
	return c.sendInternalError(ctx, chatID)
}

func (c *TelegramController) getStatusMap(ctx context.Context) map[uint64]*entities.Status {
	c.statusCacheMutex.RLock()
	// РћРїС‚РёРјРёР·Р°С†РёСЏ: РєРµС€ СЃС‚Р°С‚СѓСЃРѕРІ РЅР° 10 РјРёРЅСѓС‚.
	if time.Since(c.statusCacheTime) < 10*time.Minute && len(c.statusCache) > 0 {
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

// ==================== РЎР›РЈР–Р•Р‘РќР«Р• Р¤РЈРќРљР¦РР ====================
func (c *TelegramController) HandleGenerateLinkToken(ctx echo.Context) error {
	if !c.integrationService.Enabled() {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusServiceUnavailable, "Telegram Р±РѕС‚ РЅРµ РЅР°СЃС‚СЂРѕРµРЅ", nil, nil), c.logger)
	}

	linkData, err := c.userService.GenerateTelegramLinkToken(ctx.Request().Context())
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	response := map[string]interface{}{
		"short_code":         linkData.ShortCode,
		"expires_in_seconds": linkData.ExpiresInSeconds,
	}
	if botLink := c.integrationService.BuildBotStartLink(linkData.Token); botLink != "" {
		response["bot_link"] = botLink
	}

	return utils.SuccessResponse(ctx, response,
		"РўРѕРєРµРЅ СЃРіРµРЅРµСЂРёСЂРѕРІР°РЅ", http.StatusOK)
}

func (c *TelegramController) RegisterWebhook(baseURL string) error {
	_, err := c.integrationService.RegisterWebhook(context.Background(), baseURL)
	return err
}

func (c *TelegramController) StartCleanup(ctx context.Context) {
	if c.deduplicator != nil {
		c.logger.Info("Р—Р°РїСѓСЃРє С„РѕРЅРѕРІРѕР№ РѕС‡РёСЃС‚РєРё РґРµРґСѓРїР»РёРєР°С‚РѕСЂР°")
		c.deduplicator.Cleanup(ctx, 2*time.Minute)
		c.logger.Info("Р¤РѕРЅРѕРІР°СЏ РѕС‡РёСЃС‚РєР° РѕСЃС‚Р°РЅРѕРІР»РµРЅР°")
	}
}

// ==================== РЎР›РЈР–Р•Р‘РќР«Р• Р¤РЈРќРљР¦РР ====================
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

func withCallbackQueryState(ctx context.Context, callbackQueryID string) context.Context {
	ctx = context.WithValue(ctx, callbackQueryIDContextKey, callbackQueryID)
	return context.WithValue(ctx, callbackAnswerStateContextKey, &callbackAnswerState{})
}

func callbackQueryIDFromContext(ctx context.Context) string {
	callbackQueryID, _ := ctx.Value(callbackQueryIDContextKey).(string)
	return strings.TrimSpace(callbackQueryID)
}

func callbackAnswerStateFromContext(ctx context.Context) *callbackAnswerState {
	state, _ := ctx.Value(callbackAnswerStateContextKey).(*callbackAnswerState)
	return state
}

func (c *TelegramController) ensureCallbackAnswered(ctx context.Context, wait time.Duration) {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		_ = c.answerCallback(ctx, "")
	}
}

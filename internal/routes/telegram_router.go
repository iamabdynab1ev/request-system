package routes

import (
	"context"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	tgCtrl "request-system/internal/controllers/telegram"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/middleware"
	"request-system/pkg/telegram"
)

func runTelegramRouter(
	e *echo.Echo,
	userService services.UserServiceInterface,
	orderService services.OrderServiceInterface,
	tgService telegram.ServiceInterface,
	cacheRepo repositories.CacheRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	historyRepo repositories.OrderHistoryRepositoryInterface,

	authPermissionService services.AuthPermissionServiceInterface,
	orderTypeRepo repositories.OrderTypeRepositoryInterface,
	authMW *middleware.AuthMiddleware,
	cfg *config.Config,
	logger *zap.Logger,
	appCtx context.Context,
) {
	tgIntegrationService := services.NewTelegramIntegrationService(cfg.Telegram, logger)

	tgController := tgCtrl.NewTelegramController(
		userService,
		orderService,
		tgIntegrationService,
		tgService,
		cacheRepo,
		statusRepo,
		userRepo,
		historyRepo,
		authPermissionService,
		logger,
		orderTypeRepo,
		cfg.Telegram,
	)

	go tgController.StartCleanup(appCtx)

	api := e.Group("/api")
	secureGroup := api.Group("", authMW.Auth)

	secureGroup.GET("/profile/telegram", tgController.HandleTelegramLinkStatus)
	secureGroup.DELETE("/profile/telegram", tgController.HandleUnlinkTelegram)
	secureGroup.POST("/profile/telegram/generate-token", tgController.HandleGenerateLinkToken)

	if !tgIntegrationService.Enabled() {
		logger.Warn("Telegram integration disabled: TELEGRAM_BOT_TOKEN is empty")
		return
	}

	api.POST("/webhooks/telegram", tgController.HandleTelegramWebhook)

	// Регистрация webhook
	go func() {
		err := tgController.RegisterWebhook(cfg.Server.BaseURL)
		if err != nil {
			logger.Error("Не удалось зарегистрировать Telegram Webhook", zap.Error(err))
		}
	}()
}

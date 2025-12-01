package routes

import (
	"context"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/controllers"
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
	authMW *middleware.AuthMiddleware,
	cfg *config.Config,
	logger *zap.Logger,
	appCtx context.Context,
) {
	tgController := controllers.NewTelegramController(
		userService,
		orderService,
		tgService,
		cacheRepo,
		statusRepo,
		userRepo,
		historyRepo,
		authPermissionService,
		cfg.Telegram.BotToken,
		logger,
		cfg.Telegram,
	)

	go tgController.StartCleanup(appCtx)

	api := e.Group("/api")
	secureGroup := api.Group("", authMW.Auth)

	secureGroup.POST("/profile/telegram/generate-token", tgController.HandleGenerateLinkToken)
	api.POST("/webhooks/telegram", tgController.HandleTelegramWebhook)

	// Регистрация webhook
	go func() {
		err := tgController.RegisterWebhook(cfg.Server.BaseURL)
		if err != nil {
			logger.Error("Не удалось зарегистрировать Telegram Webhook", zap.Error(err))
		}
	}()
}

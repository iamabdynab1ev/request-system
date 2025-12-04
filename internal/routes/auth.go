package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"
	"request-system/pkg/service"
	"request-system/pkg/telegram"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"go.uber.org/zap"
)

func runAuthRouter(
	api *echo.Group,
	dbConn *pgxpool.Pool,
	redisClient *redis.Client,
	jwtSvc service.JWTService,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
	fileStorage filestorage.FileStorageInterface,
	authPermissionService services.AuthPermissionServiceInterface,
	cfg *config.Config,

	positionService services.PositionServiceInterface,
	branchService services.BranchServiceInterface,
	departmentService services.DepartmentServiceInterface,
	otdelService services.OtdelServiceInterface,
	officeService services.OfficeServiceInterface,
) {
	userRepository := repositories.NewUserRepository(dbConn, logger)
	cacheRepository := repositories.NewRedisCacheRepository(redisClient)

	// 2. Включаем реализацию через Telegram
	tgService := telegram.NewService(cfg.Telegram.BotToken)
	notificationService := services.NewTelegramNotificationService(tgService, logger)
	txManager := repositories.NewTxManager(dbConn, logger)

	authService := services.NewAuthService(
		txManager,
		userRepository,
		cacheRepository,
		logger,
		&cfg.Auth,
		&cfg.LDAP,
		notificationService,
		positionService,
		branchService,
		departmentService,
		otdelService,
		officeService,
	)

	authCtrl := controllers.NewAuthController(
		authService,
		authPermissionService,
		jwtSvc,
		fileStorage,
		logger,
	)

	authGroup := api.Group("/auth")
	secureAuthGroup := authGroup.Group("", authMW.Auth)
	authGroup.POST("/login", authCtrl.Login)
	authGroup.POST("/refresh_token", authCtrl.RefreshToken)

	passwordGroup := authGroup.Group("/password")
	passwordGroup.POST("/request", authCtrl.RequestPasswordReset)
	passwordGroup.POST("/verify_phone", authCtrl.VerifyCode)
	passwordGroup.POST("/reset", authCtrl.ResetPassword)

	secureAuthGroup.GET("/me", authCtrl.Me)
	secureAuthGroup.POST("/logout", authCtrl.Logout)
	secureAuthGroup.PUT("/me", authCtrl.UpdateMe, authMW.Auth)
}

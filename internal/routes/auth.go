// Файл: internal/routes/auth_router.go
package routes

import (
	// <<< ДОБАВЛЯЕМ ИМПОРТ

	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/middleware"
	"request-system/pkg/service"

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
	authPermissionService services.AuthPermissionServiceInterface,
	cfg *config.Config,
) {
	userRepository := repositories.NewUserRepository(dbConn, logger)
	cacheRepository := repositories.NewRedisCacheRepository(redisClient)

	authService := services.NewAuthService(userRepository, cacheRepository, logger, &cfg.Auth)

	authCtrl := controllers.NewAuthController(authService, authPermissionService, jwtSvc, logger)

	authGroup := api.Group("/auth")
	{
		authGroup.POST("/login", authCtrl.Login)
		authGroup.POST("/refresh_token", authCtrl.RefreshToken)
		authGroup.GET("/me", authCtrl.Me, authMW.Auth)

		passwordGroup := authGroup.Group("/password")
		passwordGroup.POST("/request", authCtrl.RequestPasswordReset)
		passwordGroup.POST("/verify_phone", authCtrl.VerifyCode)
		passwordGroup.POST("/reset", authCtrl.ResetPassword)
	}
}

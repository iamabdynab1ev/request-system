package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/service"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runAuthRouter(api *echo.Group, dbConn *pgxpool.Pool, redisClient *redis.Client, jwtSvc service.JWTService, logger *zap.Logger) {
	userRepository := repositories.NewUserRepository(dbConn)
	cacheRepository := repositories.NewRedisCacheRepository(redisClient)
	authService := services.NewAuthService(userRepository, cacheRepository, logger)
	authCtrl := controllers.NewAuthController(authService, jwtSvc, logger)

	authGroup := api.Group("/auth")
	{
		authGroup.POST("/login", authCtrl.Login)
		authGroup.POST("/send-code", authCtrl.SendCode)
		authGroup.POST("/verify-code", authCtrl.VerifyCode)
		authGroup.POST("/refresh-token", authCtrl.RefreshToken)
		authGroup.POST("/recovery-options", authCtrl.CheckRecoveryOptions)
		authGroup.POST("/recovery-send", authCtrl.SendRecoveryInstructions)
		authGroup.POST("/reset-password/email", authCtrl.ResetPasswordWithEmail)
		authGroup.POST("/reset-password/phone", authCtrl.ResetPasswordWithPhone)
	}
}

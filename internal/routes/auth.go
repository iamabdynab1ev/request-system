package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/middleware"
	"request-system/pkg/service"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runAuthRouter(api *echo.Group, dbConn *pgxpool.Pool, redisClient *redis.Client, jwtSvc service.JWTService, logger *zap.Logger, authMW *middleware.AuthMiddleware) {
	userRepository := repositories.NewUserRepository(dbConn, logger)
	cacheRepository := repositories.NewRedisCacheRepository(redisClient)
	authService := services.NewAuthService(userRepository, cacheRepository, logger)
	authCtrl := controllers.NewAuthController(authService, jwtSvc, logger)

	authGroup := api.Group("/auth")
	{
		authGroup.POST("/login", authCtrl.Login)
		authGroup.POST("/send_code", authCtrl.SendCode)
		authGroup.POST("/verify_code", authCtrl.VerifyCode)
		authGroup.POST("/refresh_token", authCtrl.RefreshToken)
		authGroup.POST("/recovery_options", authCtrl.CheckRecoveryOptions)
		authGroup.POST("/recovery_send", authCtrl.SendRecoveryInstructions)
		authGroup.POST("/reset_password/email", authCtrl.ResetPasswordWithEmail)
		authGroup.POST("/reset_password/phone_number", authCtrl.ResetPasswordWithPhone)
		authGroup.GET("/me", authCtrl.Me, authMW.Auth)
	}
}

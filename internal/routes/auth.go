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

func RUN_AUTH_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool, redisClient *redis.Client, jwtSvc service.JWTService, logger *zap.Logger) {
	// Сборка зависимостей
	userRepository := repositories.NewUserRepository(dbConn)
	cacheRepository := repositories.NewRedisCacheRepository(redisClient) // ИСПРАВЛЕНО: NewRedisCacheRepository
	authService := services.NewAuthService(userRepository, cacheRepository, logger)
	authCtrl := controllers.NewAuthController(authService, jwtSvc, logger)

	authGroup := e.Group("/api/auth")
	{
		authGroup.POST("/login", authCtrl.Login)
		authGroup.POST("/send-code", authCtrl.SendCode)
		authGroup.POST("/verify-code", authCtrl.VerifyCode)
		
	}
}

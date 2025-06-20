
package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/service" 

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap" 
)

func RUN_AUTH_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool, jwtSvc service.JWTService, logger *zap.Logger) {
	userRepository := repositories.NewUserRepository(dbConn)
	authService := services.NewAuthService(userRepository, logger)         
	authCtrl := controllers.NewAuthController(authService, jwtSvc, logger) 

	authGroup := e.Group("/api/auth")
	authGroup.POST("/login", authCtrl.Login)
	authGroup.POST("/refresh-token", authCtrl.RefreshToken)
}

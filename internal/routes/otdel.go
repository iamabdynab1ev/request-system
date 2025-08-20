package routes

import (
	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runOtdelRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware) {
	otdelRepository := repositories.NewOtdelRepository(dbConn, logger)
	userRepo := repositories.NewUserRepository(dbConn, logger) // Нужен для проверки прав

	otdelService := services.NewOtdelService(otdelRepository, userRepo, logger)
	otdelCtrl := controllers.NewOtdelController(otdelService, logger)

	otdelGroup := secureGroup.Group("/otdel")

	// Добавляем проверку прав для каждого роута
	otdelGroup.GET("", otdelCtrl.GetOtdels, authMW.AuthorizeAny(authz.OtdelsView))
	otdelGroup.GET("/:id", otdelCtrl.FindOtdel, authMW.AuthorizeAny(authz.OtdelsView))
	otdelGroup.POST("", otdelCtrl.CreateOtdel, authMW.AuthorizeAny(authz.OtdelsCreate))
	otdelGroup.PUT("/:id", otdelCtrl.UpdateOtdel, authMW.AuthorizeAny(authz.OtdelsUpdate))
	otdelGroup.DELETE("/:id", otdelCtrl.DeleteOtdel, authMW.AuthorizeAny(authz.OtdelsDelete))
}

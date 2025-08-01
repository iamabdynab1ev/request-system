package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runOtdelRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {

	otdelRepository := repositories.NewOtdelRepository(dbConn)
	otdelService := services.NewOtdelService(otdelRepository, logger)
	otdelCtrl := controllers.NewOtdelController(otdelService, logger)

	secureGroup.GET("/otdel", otdelCtrl.GetOtdels)
	secureGroup.GET("/otdel/:id", otdelCtrl.FindOtdel)
	secureGroup.POST("/otdel", otdelCtrl.CreateOtdel)
	secureGroup.PUT("/otdel/:id", otdelCtrl.UpdateOtdel)
	secureGroup.DELETE("/otdel/:id", otdelCtrl.DeleteOtdel)
}

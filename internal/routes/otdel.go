package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_OTDEL_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		logger = logger.NewLogger()

		otdelRepository = repositories.NewOtdelRepository(dbConn)
		otdelService    = services.NewOtdelService(otdelRepository, logger)
		otdelCtrl       = controllers.NewOtdelController(otdelService, logger)
	)

	e.GET("/otdels", otdelCtrl.GetOtdels)
	e.GET("/otdel/:id", otdelCtrl.FindOtdel)
	e.POST("/otdel", otdelCtrl.CreateOtdel)
	e.PUT("/otdel/:id", otdelCtrl.UpdateOtdel)
	e.DELETE("/otdel/:id", otdelCtrl.DeleteOtdel)
}

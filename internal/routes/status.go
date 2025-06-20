package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_STATUS_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		logger = logger.NewLogger()

		statusRepository = repositories.NewStatusRepository(dbConn)
		statusService    = services.NewStatusService(statusRepository, logger)
		statusCtrl       = controllers.NewStatusController(statusService, logger)
	)

	e.GET("status", statusCtrl.GetStatuses)
	e.GET("status/:id", statusCtrl.FindStatus)
	e.POST("status", statusCtrl.CreateStatus)
	e.PUT("status/:id", statusCtrl.UpdateStatus)
	e.DELETE("status/:id", statusCtrl.DeleteStatus)
}

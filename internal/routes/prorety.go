package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_PRORETY_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		logger = logger.NewLogger()

		proretyRepository = repositories.NewProretyRepository(dbConn)
		proretyService    = services.NewProretyService(proretyRepository, logger)
		proretyCtrl       = controllers.NewProretyController(proretyService, logger)
	)

	e.GET("/proreties", proretyCtrl.GetProreties)
	e.GET("/prorety/:id", proretyCtrl.FindProrety)
	e.POST("/prorety", proretyCtrl.CreateProrety)
	e.PUT("/prorety/:id", proretyCtrl.UpdateProrety)
	e.DELETE("/prorety/:id", proretyCtrl.DeleteProrety)
}

package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_POSITION_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		logger = logger.NewLogger()

		positionRepository = repositories.NewPositionRepository(dbConn)
		positionService    = services.NewPositionService(positionRepository, logger)
		positionCtrl       = controllers.NewPositionController(positionService, logger)
	)

	e.GET("/positions", positionCtrl.GetPositions)
	e.GET("/position/:id", positionCtrl.FindPosition)
	e.POST("/position", positionCtrl.CreatePosition)
	e.PUT("/position/:id", positionCtrl.UpdatePosition)
	e.DELETE("/position/:id", positionCtrl.DeletePosition)
}

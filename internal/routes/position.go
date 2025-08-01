package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runPositionRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	positionRepository := repositories.NewPositionRepository(dbConn)
	positionService := services.NewPositionService(positionRepository, logger)
	positionCtrl := controllers.NewPositionController(positionService, logger)

	secureGroup.GET("/position", positionCtrl.GetPositions)
	secureGroup.GET("/position/:id", positionCtrl.FindPosition)
	secureGroup.POST("/position", positionCtrl.CreatePosition)
	secureGroup.PUT("/position/:id", positionCtrl.UpdatePosition)
	secureGroup.DELETE("/position/:id", positionCtrl.DeletePosition)
}

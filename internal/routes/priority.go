package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func RunProretyRouter(e *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	priorityRepository := repositories.NewPriorityRepository(dbConn)
	priorityService := services.NewPriorityService(priorityRepository, logger)
	priorityCtrl := controllers.NewPriorityController(priorityService, logger)

	e.GET("/priority", priorityCtrl.GetPriorities)
	e.GET("/priority/:id", priorityCtrl.FindPriority)
	e.POST("/priority", priorityCtrl.CreatePriority)
	e.PUT("/priority/:id", priorityCtrl.UpdatePriority)
	e.DELETE("/priority/:id", priorityCtrl.DeletePriority)
}

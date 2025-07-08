package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runEquipmentRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	equipmentRepository := repositories.NewEquipmentRepository(dbConn)
	equipmentService := services.NewEquipmentService(equipmentRepository, logger)
	equipmentCtrl := controllers.NewEquipmentController(equipmentService, logger)

	secureGroup.GET("/equipment", equipmentCtrl.GetEquipments)
	secureGroup.GET("/equipment/:id", equipmentCtrl.FindEquipment)
	secureGroup.POST("/equipment", equipmentCtrl.CreateEquipment)
	secureGroup.PUT("/equipment/:id", equipmentCtrl.UpdateEquipment)
	secureGroup.DELETE("/equipment/:id", equipmentCtrl.DeleteEquipment)
}

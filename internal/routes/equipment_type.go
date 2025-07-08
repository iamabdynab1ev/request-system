package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runEquipmentTypeRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	equipmentTypeRepository := repositories.NewEquipmentTypeRepository(dbConn)
	equipmentTypeService := services.NewEquipmentTypeService(equipmentTypeRepository, logger)
	equipmentTypeCtrl := controllers.NewEquipmentTypeController(equipmentTypeService, logger)

	secureGroup.GET("/equipment-types", equipmentTypeCtrl.GetEquipmentTypes)
	secureGroup.GET("/equipment-type/:id", equipmentTypeCtrl.FindEquipmentType)
	secureGroup.POST("/equipment-type", equipmentTypeCtrl.CreateEquipmentType)
	secureGroup.PUT("/equipment-type/:id", equipmentTypeCtrl.UpdateEquipmentType)
	secureGroup.DELETE("/equipment-type/:id", equipmentTypeCtrl.DeleteEquipmentType)
}

package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	
	
)

func RUN_EQUIPMENT_TYPE_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		logger  = logger.NewLogger()

		equipmentTypeRepository = repositories.NewEquipmentTypeRepository(dbConn)
		equipmentTypeService    = services.NewEquipmentTypeService(equipmentTypeRepository, logger)
		equipmentTypeCtrl       = controllers.NewEquipmentTypeController(equipmentTypeService, logger)
	)
	e.GET("/equipment-types", equipmentTypeCtrl.GetEquipmentTypes)
	e.GET("/equipment-type/:id", equipmentTypeCtrl.FindEquipmentType)
	e.POST("/equipment-type", equipmentTypeCtrl.CreateEquipmentType)
	e.PUT("/equipment-type/:id", equipmentTypeCtrl.UpdateEquipmentType)
	e.DELETE("/equipment-type/:id", equipmentTypeCtrl.DeleteEquipmentType)
}
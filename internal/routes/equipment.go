package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/pkg/logger"
	"request-system/internal/services"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_EQUIPMENT_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {

	var (
		logger = logger.NewLogger()

		equipmentRepository = repositories.NewEquipmentRepository(dbConn)
		equipmentService    = services.NewEquipmentService(equipmentRepository, logger)
		equipmentCtrl       = controllers.NewEquipmentController(equipmentService, logger)
	)
	e.GET("/equipment", equipmentCtrl.GetEquipments)
	e.GET("/equipment/:id", equipmentCtrl.FindEquipment)
	e.POST("/equipment", equipmentCtrl.CreateEquipment)
	e.PUT("/equipment/:id", equipmentCtrl.UpdateEquipment)
	e.DELETE("/equipment/:id", equipmentCtrl.DeleteEquipment)
}
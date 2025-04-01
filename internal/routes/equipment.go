package routes

import (
	"request-system/internal/controllers"
	"github.com/labstack/echo/v4"
)

var equipmentCtrl = controllers.NewEquipmentController()

func RUN_EQUIPMENT_ROUTER(e *echo.Echo) {
	e.GET("equipment", equipmentCtrl.GetEquipments)
	e.GET("equipment/:id", equipmentCtrl.FindEquipments)
	e.POST("equipment", equipmentCtrl.CreateEquipments)
	e.PUT("equipment/:id", equipmentCtrl.UpdateEquipments)
	e.DELETE("equipment/:id", equipmentCtrl.DeleteEquipments)
}
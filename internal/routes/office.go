package routes

import (
	"request-system/internal/controllers"
	"github.com/labstack/echo/v4"
)

var officeCtrl = controllers.NewOfficeController()

func RUN_OFFICE_ROUTER(e *echo.Echo) {
	e.GET("office", officeCtrl.GetOffices)
	e.GET("office/:id", officeCtrl.FindOffices)
	e.POST("office", officeCtrl.CreateOffices)
	e.PUT("office/:id", officeCtrl.UpdateOffices)
	e.DELETE("office/:id", officeCtrl.DeleteOffices)
}
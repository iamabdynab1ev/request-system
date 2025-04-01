package routes

import (
	"request-system/internal/controllers"
	"github.com/labstack/echo/v4"
)

var otdelCtrl = controllers.NewOtdelController()

func RUN_OTDEL_ROUTER(e *echo.Echo) {
	e.GET("otdel", otdelCtrl.GetOtdels)
	e.GET("otdel/:id", otdelCtrl.FindOtdels)
	e.POST("otdel", otdelCtrl.CreateOtdels)
	e.PUT("otdel/:id", otdelCtrl.UpdateOtdels)
	e.DELETE("otdel/:id", otdelCtrl.DeleteOtdels)
}
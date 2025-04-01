package routes

import (
	"request-system/internal/controllers"
	"github.com/labstack/echo/v4"
)

var proretyCtrl = controllers.NewProretyController()

func RUN_PRORETY_ROUTER(e *echo.Echo) {
	e.GET("prorety", proretyCtrl.GetProreties)
	e.GET("prorety/:id", proretyCtrl.FindProreties)
	e.POST("prorety", proretyCtrl.CreateProreties)
	e.PUT("prorety/:id", proretyCtrl.UpdateProreties)
	e.DELETE("prorety/:id", proretyCtrl.DeleteProreties)
}
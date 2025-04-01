package routes

import (
	"request-system/internal/controllers"
	"github.com/labstack/echo/v4"
)

var orderCtrl = controllers.NewOrderController()

func RUN_ORDER_ROUTER(e *echo.Echo) {
	e.GET("order", orderCtrl.GetOrders)
	e.GET("order/:id", orderCtrl.FindOrders)
	e.POST("order", orderCtrl.CreateOrders)
	e.PUT("order/:id", orderCtrl.UpdateOrders)
	e.DELETE("order/:id", orderCtrl.DeleteOrders)
}

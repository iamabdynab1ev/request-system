// Файл: internal/routes/order_type.go

package routes

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/controllers"
	"request-system/internal/services"
	"request-system/pkg/middleware"
)

func runOrderTypeRouter(
	secureGroup *echo.Group,
	orderTypeService services.OrderTypeServiceInterface,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	orderTypeCtrl := controllers.NewOrderTypeController(orderTypeService, logger)

	orderType := secureGroup.Group("/order_type")
	{
		orderType.POST("", orderTypeCtrl.Create, authMW.AuthorizeAny("order_type:create"))
		orderType.GET("", orderTypeCtrl.GetAll, authMW.AuthorizeAny("order_type:view"))
		orderType.GET("/:id", orderTypeCtrl.GetByID, authMW.AuthorizeAny("order_type:view"))
		orderType.PUT("/:id", orderTypeCtrl.Update, authMW.AuthorizeAny("order_type:update"))
		orderType.DELETE("/:id", orderTypeCtrl.Delete, authMW.AuthorizeAny("order_type:delete"))

		orderType.GET("/:id/config", orderTypeCtrl.GetConfig, authMW.AuthorizeAny("order:create"))
	}
}

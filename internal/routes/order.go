// Файл: internal/routes/order.go
package routes

import (
	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/services"
	"request-system/pkg/middleware"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// runOrderRouter принимает ГОТОВЫЙ сервис и регистрирует маршруты.
func runOrderRouter(
	secureGroup *echo.Group,
	orderService services.OrderServiceInterface, // <-- Главное изменение!
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	// Создаем контроллер, передавая ему только сервис
	orderController := controllers.NewOrderController(orderService, logger)

	orders := secureGroup.Group("/order")
	{
		orders.POST("", orderController.CreateOrder, authMW.AuthorizeAny(authz.OrdersCreate))
		orders.GET("", orderController.GetOrders, authMW.AuthorizeAny(authz.OrdersView))
		orders.GET("/:id", orderController.FindOrder, authMW.AuthorizeAny(authz.OrdersView))
		orders.PUT("/:id", orderController.UpdateOrder, authMW.AuthorizeAny(authz.OrdersUpdate))
		orders.DELETE("/:id", orderController.DeleteOrder, authMW.AuthorizeAny(authz.OrdersDelete))
	}
}


package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runOrderRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, jwtSvc service.JWTService, logger *zap.Logger) {
	orderRepo := repositories.NewOrderRepository(dbConn)
	orderService := services.NewOrderService(orderRepo, logger)
	orderCtrl := controllers.NewOrderController(orderService, logger)
	{
		secureGroup.GET("/orders", orderCtrl.GetOrders)
		secureGroup.POST("/orders", orderCtrl.CreateOrder)
		secureGroup.GET("/orders/:id", orderCtrl.FindOrder)
		secureGroup.PUT("/orders/:id", orderCtrl.UpdateOrder)
		secureGroup.DELETE("/orders/:id", orderCtrl.DeleteOrder)
	}
}

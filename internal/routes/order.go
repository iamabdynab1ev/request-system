package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	mid "request-system/pkg/middleware"
	"request-system/pkg/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func RUN_ORDER_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool, jwtSvc service.JWTService, logger *zap.Logger) {
	// Создаем только те зависимости, которые нужны для ЗАЯВОК
	orderRepo := repositories.NewOrderRepository(dbConn)
	orderService := services.NewOrderService(orderRepo, logger) 
	orderCtrl := controllers.NewOrderController(orderService, logger)
	authMW := mid.NewAuthMiddleware(jwtSvc)

	ordersGroup := e.Group("/api/orders", authMW.Auth)

	ordersGroup.GET("", orderCtrl.GetOrders)
	ordersGroup.POST("", orderCtrl.CreateOrder)
	ordersGroup.GET("/:id", orderCtrl.FindOrder)
	ordersGroup.PUT("/:id", orderCtrl.UpdateOrder)
	ordersGroup.DELETE("/:id", orderCtrl.DeleteOrder)
}
package routes

import (
	"request-system/internal/controllers" // <-- Импортируем middleware (можно с псевдонимом)
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/middleware"
	"request-system/pkg/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Убедитесь, что эта функция вызывается в INIT_ROUTER
func RUN_ORDER_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool, jwtSvc service.JWTService, logger *zap.Logger) {
	// Сборка зависимостей для заявок (Order)
	orderRepo := repositories.NewOrderRepository(dbConn)
	orderService := services.NewOrderService(orderRepo, logger)
	orderCtrl := controllers.NewOrderController(orderService, logger)

	// Создаем экземпляр AuthMiddleware со всеми зависимостями
	authMW := middleware.NewAuthMiddleware(jwtSvc, logger) // <-- ИСПРАВЛЕНО

	// Создаем группу роутов и сразу применяем к ней middleware
	// Этот способ применения middleware полностью корректен.
	ordersGroup := e.Group("/api/orders", authMW.Auth)

	// Все роуты внутри этой группы теперь защищены
	{
		ordersGroup.GET("", orderCtrl.GetOrders)
		ordersGroup.POST("", orderCtrl.CreateOrder)
		ordersGroup.GET("/:id", orderCtrl.FindOrder)
		ordersGroup.PUT("/:id", orderCtrl.UpdateOrder)
		ordersGroup.DELETE("/:id", orderCtrl.DeleteOrder)
	}
}

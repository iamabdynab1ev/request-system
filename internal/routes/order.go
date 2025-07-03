package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	mid "request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func RUN_ORDER_ROUTER(
	e *echo.Echo,
	dbConn *pgxpool.Pool,
	commentRepo repositories.OrderCommentRepositoryInterface,
	delegationRepo repositories.OrderDelegationRepositoryInterface,
	userRepo repositories.UserRepositoryInterface,
	statusRepo repositories.StatusRepositoryInterface,
	priorityRepo repositories.ProretyRepositoryInterface,
	authMW *mid.AuthMiddleware, 
	logger *zap.Logger,
) {
	orderRepo := repositories.NewOrderRepository(dbConn)
	orderService := services.NewOrderService(dbConn, orderRepo, commentRepo, delegationRepo, logger, userRepo, statusRepo, priorityRepo)
	orderCtrl := controllers.NewOrderController(orderService, logger)

	ordersGroup := e.Group("/api/orders", authMW.Auth)

	ordersGroup.POST("", orderCtrl.CreateOrder, authMW.CheckPermission("Create: order"))

	ordersGroup.GET("", orderCtrl.GetOrders)
	ordersGroup.GET("/:id", orderCtrl.FindOrder)
	ordersGroup.PUT("/:id", orderCtrl.UpdateOrder)
	ordersGroup.DELETE("/:id", orderCtrl.SoftDeleteOrder)
}

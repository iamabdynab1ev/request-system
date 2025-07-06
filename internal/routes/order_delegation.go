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

func RUN_ORDER_DELEGATION_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool, jwtSvc service.JWTService, logger *zap.Logger) {
	repo := repositories.NewOrderDelegationRepository(dbConn)
	service := services.NewOrderDelegationService(repo, logger)
	ctrl := controllers.NewOrderDelegationController(service, logger)
	authMW := mid.NewAuthMiddleware(jwtSvc, logger)

	apiGroup := e.Group("/api", authMW.Auth)
	apiGroup.GET("/order-delegations", ctrl.GetOrderDelegations)
	apiGroup.GET("/order-delegation/:id", ctrl.FindOrderDelegation)
	apiGroup.POST("/order-delegation", ctrl.CreateOrderDelegation)
	apiGroup.DELETE("/order-delegation/:id", ctrl.DeleteOrderDelegation)
}

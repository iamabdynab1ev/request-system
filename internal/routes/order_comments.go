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

func RUN_ORDER_COMMENT_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool, jwtSvc service.JWTService, logger *zap.Logger) {
	repo := repositories.NewOrderCommentRepository(dbConn)
	service := services.NewOrderCommentService(repo, logger)
	ctrl := controllers.NewOrderCommentController(service)
	authMW := mid.NewAuthMiddleware(jwtSvc)

	apiGroup := e.Group("/api", authMW.Auth)
	apiGroup.GET("/order-comments", ctrl.GetOrderComments)
	apiGroup.GET("/order-comment/:id", ctrl.FindOrderComment)
	apiGroup.POST("/order-comment", ctrl.CreateOrderComment)
	apiGroup.PUT("/order-comment/:id", ctrl.UpdateOrderComment)
	apiGroup.DELETE("/order-comment/:id", ctrl.DeleteOrderComment)
}

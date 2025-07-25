package routes
/*
import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runOrderCommentRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, jwtSvc service.JWTService, logger *zap.Logger) {
	repo := repositories.NewOrderCommentRepository(dbConn)
	service := services.NewOrderCommentService(repo, logger)
	ctrl := controllers.NewOrderCommentController(service)

	secureGroup.GET("/order-comments", ctrl.GetOrderComments)
	secureGroup.GET("/order-comment/:id", ctrl.FindOrderComment)
	secureGroup.POST("/order-comment", ctrl.CreateOrderComment)
	secureGroup.PUT("/order-comment/:id", ctrl.UpdateOrderComment)
	secureGroup.DELETE("/order-comment/:id", ctrl.DeleteOrderComment)
}
*/
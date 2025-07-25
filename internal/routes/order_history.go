// internal/routes/order_history.go
package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runOrderHistoryRouter(group *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	repo := repositories.NewOrderHistoryRepository(dbConn)
	service := services.NewOrderHistoryService(repo, logger)
	controller := controllers.NewOrderHistoryController(service, logger)

	group.GET("/orders/:orderID/history", controller.GetHistoryForOrder)
}

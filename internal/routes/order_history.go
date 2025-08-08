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
	// Инициализируем все необходимые репозитории
	repo := repositories.NewOrderHistoryRepository(dbConn)
	userRepo := repositories.NewUserRepository(dbConn, logger)
	orderRepo := repositories.NewOrderRepository(dbConn)

	// Передаем все необходимые зависимости в конструктор сервиса
	service := services.NewOrderHistoryService(repo, userRepo, orderRepo, logger)
	controller := controllers.NewOrderHistoryController(service, logger)

	group.GET("/order/:orderID/history", controller.GetHistoryForOrder)
}

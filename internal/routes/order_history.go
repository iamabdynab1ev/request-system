// internal/routes/order_history_router.go

package routes

import (
	"request-system/internal/authz" // <-- Импорт констант
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/middleware" // <-- Импорт мидлвара

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Сигнатура теперь принимает authMW, как и другие "защищенные" роутеры
func runOrderHistoryRouter(
	secureGroup *echo.Group, // Используем secureGroup, который уже под authMW.Auth
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	// Инициализируем все необходимые репозитории
	repo := repositories.NewOrderHistoryRepository(dbConn)
	userRepo := repositories.NewUserRepository(dbConn, logger)
	orderRepo := repositories.NewOrderRepository(dbConn, logger) // Добавляем logger

	service := services.NewOrderHistoryService(repo, userRepo, orderRepo, logger)

	controller := controllers.NewOrderHistoryController(service, logger)

	// Регистрируем роут ВНУТРИ `secureGroup` и добавляем ПРОВЕРКУ ПРАВ
	// Право на просмотр истории заявки = право на просмотр самой заявки
	secureGroup.GET("/order/:orderID/history", controller.GetHistoryForOrder, authMW.AuthorizeAny(authz.OrdersView))
}

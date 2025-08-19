// internal/routes/order_routes.go
package routes

import (
	"request-system/internal/authz" // <-- ШАГ 1: Импортируем наши константы
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runOrderRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware) {

	// Инициализация зависимостей (твой код здесь идеален)
	txManager := repositories.NewTxManager(dbConn)
	orderRepo := repositories.NewOrderRepository(dbConn, logger)
	userRepo := repositories.NewUserRepository(dbConn, logger)
	statusRepo := repositories.NewStatusRepository(dbConn)
	priorityRepo := repositories.NewPriorityRepository(dbConn)
	attachRepo := repositories.NewAttachmentRepository(dbConn)
	historyRepo := repositories.NewOrderHistoryRepository(dbConn)
	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		logger.Fatal("не удалось создать файловое хранилище для OrderRouter", zap.Error(err))
	}
	orderService := services.NewOrderService(
		txManager, orderRepo, userRepo, statusRepo, priorityRepo,
		attachRepo, historyRepo, fileStorage, logger,
	)
	orderController := controllers.NewOrderController(orderService, logger)

	// --- ШАГ 2: Заменяем строки на константы ---
	orders := secureGroup.Group("/order") // Группируем роуты для чистоты
	orders.POST("", orderController.CreateOrder, authMW.AuthorizeAny(authz.OrdersCreate))
	orders.GET("", orderController.GetOrders, authMW.AuthorizeAny(authz.OrdersView))
	orders.GET("/:id", orderController.FindOrder, authMW.AuthorizeAny(authz.OrdersView))
	orders.PUT("/:id", orderController.UpdateOrder, authMW.AuthorizeAny(authz.OrdersUpdate, authz.OrdersDelegate))
	orders.DELETE("/:id", orderController.DeleteOrder, authMW.AuthorizeAny(authz.OrdersDelete))
}

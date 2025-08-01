package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runOrderRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware, authPermissionService services.AuthPermissionServiceInterface) {

	txManager := repositories.NewTxManager(dbConn)
	orderRepo := repositories.NewOrderRepository(dbConn)
	userRepo := repositories.NewUserRepository(dbConn)
	statusRepo := repositories.NewStatusRepository(dbConn)
	priorityRepo := repositories.NewPriorityRepository(dbConn)
	attachRepo := repositories.NewAttachmentRepository(dbConn)
	historyRepo := repositories.NewOrderHistoryRepository(dbConn)

	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		logger.Fatal("не удалось создать файловое хранилище", zap.Error(err))
	}

	orderService := services.NewOrderService(
		txManager,
		orderRepo,
		userRepo,
		statusRepo,
		priorityRepo,
		attachRepo,
		historyRepo,
		fileStorage,
		logger,
	)

	orderController := controllers.NewOrderController(orderService, logger)

	secureGroup.POST("/order", orderController.CreateOrder, authMW.AuthorizeAny("orders:create", "orders:manage:all"))
	secureGroup.GET("/order", orderController.GetOrders, authMW.AuthorizeAny("orders:view:all", "orders:view:department", "orders:view:own"))
	secureGroup.POST("/order/:id/delegate", orderController.DelegateOrder, authMW.AuthorizeAny("orders:delegate:department", "orders:manage:all"))
	secureGroup.GET("/order", orderController.GetOrders, authMW.AuthorizeAny("orders:view:all", "orders:view:department", "orders:view:own"))

	// group.GET("/orders/:id", orderController.FindOrder, authMW.AuthorizeAny("orders:view:all", "orders:view:department", "orders:view:own")) // Просмотр конкретной заявки
	// group.PUT("/orders/:id", orderController.UpdateOrder, authMW.AuthorizeAny("orders:update:own", "orders:manage:all")) // Обновление своей заявки
}

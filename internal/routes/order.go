package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/filestorage"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runOrderRouter(group *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) /* ...другие зависимости... */ {

	// Инициализируем ВСЕ нужные репозитории
	txManager := repositories.NewTxManager(dbConn)
	orderRepo := repositories.NewOrderRepository(dbConn)
	userRepo := repositories.NewUserRepository(dbConn)         // <-- предполагаемое имя конструктора
	statusRepo := repositories.NewStatusRepository(dbConn)     // <-- предполагаемое имя конструктора
	priorityRepo := repositories.NewPriorityRepository(dbConn) // <-- предполагаемое имя конструктора
	attachRepo := repositories.NewAttachmentRepository(dbConn)
	historyRepo := repositories.NewOrderHistoryRepository(dbConn)

	// Создаем файловое хранилище
	// ВАЖНО: "uploads" - это папка в корне проекта. Создайте ее или укажите другой путь.
	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		logger.Fatal("не удалось создать файловое хранилище", zap.Error(err))
	}

	// Передаем ВСЕ зависимости в сервис
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

	group.POST("/orders", orderController.CreateOrder)
	group.GET("/orders", orderController.GetOrders)
	group.POST("/orders/:id/delegate", orderController.DelegateOrder)

}

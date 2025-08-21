package routes

import (
	"log"

	"request-system/internal/authz" // <-- Импорт констант
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware" // <-- Импорт мидлвара

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Сигнатура теперь принимает authMW, как и все защищенные роутеры
func runStatusRouter(
	secureGroup *echo.Group, // Используем secureGroup
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	// Инициализация зависимостей
	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		log.Fatalf("не удалось инициализировать хранилище файлов для Status: %v", err)
	}

	statusRepository := repositories.NewStatusRepository(dbConn)
	userRepository := repositories.NewUserRepository(dbConn, logger)

	statusService := services.NewStatusService(statusRepository, userRepository, fileStorage, logger)

	statusCtrl := controllers.NewStatusController(statusService, logger)

	statuses := secureGroup.Group("/status")
	statuses.GET("", statusCtrl.GetStatuses, authMW.AuthorizeAny(authz.StatusesView))
	statuses.GET("/:id", statusCtrl.FindStatus, authMW.AuthorizeAny(authz.StatusesView))
	statuses.POST("", statusCtrl.CreateStatus, authMW.AuthorizeAny(authz.StatusesCreate))
	statuses.PUT("/:id", statusCtrl.UpdateStatus, authMW.AuthorizeAny(authz.StatusesUpdate))
	statuses.DELETE("/:id", statusCtrl.DeleteStatus, authMW.AuthorizeAny(authz.StatusesDelete))
}

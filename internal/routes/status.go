package routes

import (
	"log"
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/filestorage"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runStatusRouter(api *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {

	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		log.Fatalf("не удалось инициализировать хранилище файлов: %v", err)
	}
	statusRepository := repositories.NewStatusRepository(dbConn)
	statusService := services.NewStatusService(statusRepository, fileStorage, logger)
	statusCtrl := controllers.NewStatusController(statusService, logger)

	api.GET("/status", statusCtrl.GetStatuses)
	api.GET("/status/:id", statusCtrl.FindStatus)
	api.POST("/status", statusCtrl.CreateStatus)
	api.PUT("/status/:id", statusCtrl.UpdateStatus)
	api.DELETE("/status/:id", statusCtrl.DeleteStatus)
}

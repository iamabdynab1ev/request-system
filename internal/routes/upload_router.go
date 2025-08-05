package routes

import (
	"request-system/internal/controllers"
	"request-system/pkg/filestorage"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runUploadRouter(
	group *echo.Group,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) {
	uploadController := controllers.NewUploadController(fileStorage, logger)

	group.POST("/upload/:type", uploadController.Upload)
}

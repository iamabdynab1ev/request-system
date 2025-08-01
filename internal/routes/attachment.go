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

func runAttachmentRouter(
	group *echo.Group,
	dbConn *pgxpool.Pool,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
) {
	attachmentRepo := repositories.NewAttachmentRepository(dbConn)

	attachmentService := services.NewAttachmentService(
		attachmentRepo,
		fileStorage,
		logger,
	)

	attachmentController := controllers.NewAttachmentController(
		attachmentService,
		logger,
	)

	group.GET("/attachment", attachmentController.GetAttachmentsByOrder)
	group.DELETE("/attachment/:id", attachmentController.DeleteAttachment)
}

package routes

import (
	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runAttachmentRouter(
	group *echo.Group,
	dbConn *pgxpool.Pool,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	attachmentRepo := repositories.NewAttachmentRepository(dbConn)
	orderRepo := repositories.NewOrderRepository(dbConn, logger)
	userRepo := repositories.NewUserRepository(dbConn, logger)
	historyRepo := repositories.NewOrderHistoryRepository(dbConn, logger)

	attachmentService := services.NewAttachmentService(
		attachmentRepo,
		orderRepo,
		userRepo,
		historyRepo,
		fileStorage,
		logger,
	)

	attachmentController := controllers.NewAttachmentController(
		attachmentService,
		logger,
	)

	group.GET("/attachment", attachmentController.GetAttachmentsByOrder, authMW.AuthorizeAny(authz.OrdersView))
	group.DELETE(
		"/attachment/:id",
		attachmentController.DeleteAttachment,
		authMW.AuthorizeAny(
			authz.OrdersUpdate,
			authz.OrdersUpdateInOtdelScope,
			authz.OrdersUpdateInOfficeScope,
			authz.OrdersUpdateInBranchScope,
			authz.OrdersUpdateInDepartmentScope,
		),
	)
}

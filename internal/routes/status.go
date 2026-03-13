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


func runStatusRouter(
	secureGroup *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
	fileStorage filestorage.FileStorageInterface,
) {
	
	statusRepository := repositories.NewStatusRepository(dbConn)
	userRepository := repositories.NewUserRepository(dbConn, logger) 

	statusService := services.NewStatusService(statusRepository, userRepository, fileStorage, logger)

	statusCtrl := controllers.NewStatusController(statusService, logger)

	statuses := secureGroup.Group("/status")
	{
		statuses.GET("", statusCtrl.GetStatuses, authMW.AuthorizeAny(authz.StatusesView))
		statuses.GET("/:id", statusCtrl.FindStatus, authMW.AuthorizeAny(authz.StatusesView))
		statuses.POST("", statusCtrl.CreateStatus, authMW.AuthorizeAny(authz.StatusesCreate))
		statuses.PUT("/:id", statusCtrl.UpdateStatus, authMW.AuthorizeAny(authz.StatusesUpdate))
		statuses.DELETE("/:id", statusCtrl.DeleteStatus, authMW.AuthorizeAny(authz.StatusesDelete))
	}
}

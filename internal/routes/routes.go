package routes

import (
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"
	"request-system/pkg/service"

	"request-system/internal/services"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func InitRouter(e *echo.Echo, dbConn *pgxpool.Pool, redisClient *redis.Client, jwtSvc service.JWTService, logger *zap.Logger, authPermissionService services.AuthPermissionServiceInterface) {
	logger.Info("InitRouter: Начало создания маршрутов")

	api := e.Group("/api")

	authMW := middleware.NewAuthMiddleware(jwtSvc, authPermissionService, logger)
	runAuthRouter(api, dbConn, redisClient, jwtSvc, logger, authMW)
	runStatusRouter(api, dbConn, logger)

	secureGroup := api.Group("", authMW.Auth)

	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		logger.Fatal("не удалось создать файловое хранилище", zap.Error(err))
	}
	runUploadRouter(secureGroup, fileStorage, logger)
	RunProretyRouter(secureGroup, dbConn, logger)
	runDepartmentRouter(secureGroup, dbConn, logger)
	runOtdelRouter(secureGroup, dbConn, logger)
	runEquipmentTypeRouter(secureGroup, dbConn, logger)
	runBranchRouter(secureGroup, dbConn, logger)
	runOfficeRouter(secureGroup, dbConn, logger)

	runPermissionRouter(secureGroup, dbConn, logger, authMW, authPermissionService)
	runRoleRouter(secureGroup, dbConn, logger, authMW, authPermissionService)
	runRolePermissionRouter(secureGroup, dbConn, logger, authMW, authPermissionService)
	runUserRouter(secureGroup, dbConn, logger, authMW, authPermissionService, fileStorage)
	runOrderRouter(secureGroup, dbConn, logger, authMW, authPermissionService)
	runEquipmentRouter(secureGroup, dbConn, logger)

	RunOrderDocumentRouter(secureGroup, dbConn, logger)
	runPositionRouter(secureGroup, dbConn, logger)
	runOrderHistoryRouter(secureGroup, dbConn, logger)
	runAttachmentRouter(secureGroup, dbConn, fileStorage, logger)

	logger.Info("INIT_ROUTER: Создание маршрутов завершено")
}

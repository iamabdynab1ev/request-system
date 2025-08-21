// routes/main_router.go

package routes

import (
	// Убедись, что есть импорт

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"
	"request-system/pkg/service"
)

func InitRouter(e *echo.Echo, dbConn *pgxpool.Pool, redisClient *redis.Client, jwtSvc service.JWTService, logger *zap.Logger, authPermissionService services.AuthPermissionServiceInterface, cfg *config.Config) {
	logger.Info("InitRouter: Начало создания маршрутов")

	api := e.Group("/api")

	authMW := middleware.NewAuthMiddleware(jwtSvc, authPermissionService, logger)

	runAuthRouter(api, dbConn, redisClient, jwtSvc, logger, authMW, authPermissionService, cfg)

	secureGroup := api.Group("", authMW.Auth)

	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		logger.Fatal("не удалось создать файловое хранилище", zap.Error(err))
	}

	runStatusRouter(secureGroup, dbConn, logger, authMW)

	runUploadRouter(secureGroup, fileStorage, logger)
	RunPriorityRouter(secureGroup, dbConn, logger, authMW)
	runDepartmentRouter(secureGroup, dbConn, logger, authMW)
	runOtdelRouter(secureGroup, dbConn, logger, authMW)
	runEquipmentTypeRouter(secureGroup, dbConn, logger, authMW)
	runBranchRouter(secureGroup, dbConn, logger, authMW)
	runOfficeRouter(secureGroup, dbConn, logger, authMW)

	runPermissionRouter(secureGroup, dbConn, logger, authMW)
	runRoleRouter(secureGroup, dbConn, logger, authMW, authPermissionService)

	runUserRouter(secureGroup, dbConn, logger, authMW, fileStorage)
	runOrderRouter(secureGroup, dbConn, logger, authMW)

	runEquipmentRouter(secureGroup, dbConn, logger, authMW)
	RunOrderDocumentRouter(secureGroup, dbConn, logger)
	runPositionRouter(secureGroup, dbConn, logger)
	runOrderHistoryRouter(secureGroup, dbConn, logger, authMW)
	runAttachmentRouter(secureGroup, dbConn, fileStorage, logger)

	logger.Info("INIT_ROUTER: Создание маршрутов завершено")
}

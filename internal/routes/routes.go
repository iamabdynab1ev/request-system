package routes

import (
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"
	"request-system/pkg/service"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func InitRouter(e *echo.Echo, dbConn *pgxpool.Pool, redisClient *redis.Client, jwtSvc service.JWTService, logger *zap.Logger) {
	logger.Info("InitRouter: Начало создания маршрутов")

	api := e.Group("/api")

	runAuthRouter(api, dbConn, redisClient, jwtSvc, logger)
	runStatusRouter(api, dbConn, logger)
	authMW := middleware.NewAuthMiddleware(jwtSvc, logger)
	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		logger.Fatal("не удалось создать файловое хранилище", zap.Error(err))
	}
	protectedGroup := api.Group("", authMW.Auth)

	RunProretyRouter(protectedGroup, dbConn, logger)
	runDepartmentRouter(protectedGroup, dbConn, logger)
	runOtdelRouter(protectedGroup, dbConn, logger)
	runEquipmentTypeRouter(protectedGroup, dbConn, logger)
	runBranchRouter(protectedGroup, dbConn, logger)
	runOfficeRouter(protectedGroup, dbConn, logger)
	runPermissionRouter(protectedGroup, dbConn, logger)
	runRoleRouter(protectedGroup, dbConn, logger)
	runEquipmentRouter(protectedGroup, dbConn, logger)
	runUserRouter(protectedGroup, dbConn, logger)
	RunProretyRouter(protectedGroup, dbConn, logger)
	//RunOrderDelegrationRouter(protectedGroup, dbConn, jwtSvc, logger)
	//runOrderCommentRouter(protectedGroup, dbConn, jwtSvc, logger)
	RunOrderDocumentRouter(protectedGroup, dbConn, logger)
	runPositionRouter(protectedGroup, dbConn, logger)

	runOrderRouter(protectedGroup, dbConn, logger)
	runOrderHistoryRouter(protectedGroup, dbConn, logger)
	runOrderHistoryRouter(protectedGroup, dbConn, logger)

	runAttachmentRouter(protectedGroup, dbConn, fileStorage, logger)

	logger.Info("INIT_ROUTER: Создание маршрутов завершено")
}

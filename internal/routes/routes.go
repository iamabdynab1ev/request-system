package routes

import (
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

// Структура для передачи логгеров
type Loggers struct {
	Main         *zap.Logger
	Auth         *zap.Logger
	Order        *zap.Logger
	User         *zap.Logger
	OrderHistory *zap.Logger
}

func InitRouter(
	e *echo.Echo,
	dbConn *pgxpool.Pool,
	redisClient *redis.Client,
	jwtSvc service.JWTService,
	loggers *Loggers,
	authPermissionService services.AuthPermissionServiceInterface,
	cfg *config.Config,
) {
	loggers.Main.Info("InitRouter: Начало создания маршрутов")

	api := e.Group("/api")
	authMW := middleware.NewAuthMiddleware(jwtSvc, authPermissionService, loggers.Auth)

	runAuthRouter(api, dbConn, redisClient, jwtSvc, loggers.Auth, authMW, authPermissionService, cfg)

	secureGroup := api.Group("", authMW.Auth)

	// Создаем FileStorage один раз и передаем, кому нужно
	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		loggers.Main.Fatal("не удалось создать файловое хранилище", zap.Error(err))
	}

	// Раздаем логгеры и зависимости

	// ЭТИМ НУЖЕН FILESTORAGE
	runUserRouter(secureGroup, dbConn, loggers.User, authMW, fileStorage)
	runAttachmentRouter(secureGroup, dbConn, fileStorage, loggers.Main)
	runStatusRouter(secureGroup, dbConn, loggers.Main, authMW, fileStorage)

	// ЭТИМ FILESTORAGE НЕ НУЖЕН
	runOrderRouter(secureGroup, dbConn, loggers.Order, authMW)
	runOrderHistoryRouter(secureGroup, dbConn, loggers.OrderHistory, authMW)
	RunPriorityRouter(secureGroup, dbConn, loggers.Main, authMW) // Priority иконки мы удалили
	runDepartmentRouter(secureGroup, dbConn, loggers.Main, authMW)
	runOtdelRouter(secureGroup, dbConn, loggers.Main, authMW)
	runEquipmentTypeRouter(secureGroup, dbConn, loggers.Main, authMW)
	runBranchRouter(secureGroup, dbConn, loggers.Main, authMW)
	runOfficeRouter(secureGroup, dbConn, loggers.Main, authMW)
	runEquipmentRouter(secureGroup, dbConn, loggers.Main, authMW)
	runPermissionRouter(secureGroup, dbConn, loggers.Main, authMW)
	runRoleRouter(secureGroup, dbConn, loggers.Main, authMW, authPermissionService)
	runPositionRouter(secureGroup, dbConn, loggers.Main) // У position вряд ли есть иконки

	loggers.Main.Info("INIT_ROUTER: Создание маршрутов завершено")
}

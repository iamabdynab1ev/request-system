package routes

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/eventbus"
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"
	"request-system/pkg/service"
	"request-system/pkg/websocket"
)

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
	bus *eventbus.Bus,
	wsHub *websocket.Hub,
	adService services.ADServiceInterface,
	appCtx context.Context,
) {
	loggers.Main.Info("InitRouter: start route registration")

	api := e.Group("/api")
	authMW := middleware.NewAuthMiddleware(jwtSvc, authPermissionService, loggers.Auth)
	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		loggers.Main.Fatal("failed to create local file storage", zap.Error(err))
	}
	txManager := repositories.NewTxManager(dbConn, loggers.Main)

	repos := buildRouteRepositories(dbConn, redisClient, loggers)
	servicesBundle := buildRouteServices(repos, txManager, fileStorage, bus, authPermissionService, cfg, loggers)
	controllersBundle := buildRouteControllers(servicesBundle, adService, fileStorage, wsHub, jwtSvc, cfg, loggers)

	secureGroup := api.Group("", authMW.Auth)

	runEquipImportRouter(secureGroup, dbConn, loggers.Main, authMW)
	runEquipmentRouter(secureGroup, dbConn, loggers.Main, authMW)
	runAuthRouter(api, dbConn, redisClient, jwtSvc, loggers.Auth, authMW, fileStorage, authPermissionService, cfg,
		servicesBundle.position, servicesBundle.branch, servicesBundle.department, servicesBundle.otdel, servicesBundle.office)

	api.GET("/ws", controllersBundle.websocket.ServeWs)

	runUserRouter(secureGroup, controllersBundle.user, authMW)
	runRoleRouter(secureGroup, servicesBundle.role, loggers.Main, authMW)
	runPermissionRouter(secureGroup, servicesBundle.permission, loggers.Main, authMW)
	runRolePermissionRouter(secureGroup, servicesBundle.rolePermission, loggers.Main, authMW)
	runOrderRouter(secureGroup, servicesBundle.order, loggers.Order, authMW)
	runOrderTypeRouter(secureGroup, servicesBundle.orderType, loggers.Main, authMW)
	runPositionRouter(secureGroup, servicesBundle.position, loggers.Main, authMW)
	runOrderRoutingRuleRouter(secureGroup, servicesBundle.orderRule, loggers.Main, authMW)
	runAttachmentRouter(secureGroup, dbConn, fileStorage, loggers.Main, authMW)
	runStatusRouter(secureGroup, dbConn, loggers.Main, authMW, fileStorage)
	runOrderHistoryRouter(secureGroup, controllersBundle.history, authMW)
	RunPriorityRouter(secureGroup, dbConn, loggers.Main, authMW)
	runDepartmentRouter(secureGroup, dbConn, loggers.Main, authMW, txManager)
	runOtdelRouter(secureGroup, dbConn, loggers.Main, authMW, txManager)
	runEquipmentTypeRouter(secureGroup, dbConn, loggers.Main, authMW)
	runBranchRouter(secureGroup, dbConn, loggers.Main, txManager, authMW)
	runOfficeRouter(secureGroup, servicesBundle.office, loggers.Main, authMW)
	runTelegramRouter(e, servicesBundle.user, servicesBundle.order, servicesBundle.telegram, repos.cache, repos.status, repos.user, repos.history, authPermissionService, repos.orderType, authMW, cfg, loggers.Main, appCtx)
	runSyncRouter(api, dbConn, cfg, loggers)

	secureGroup.GET("/dashboard", controllersBundle.dashboard.GetDashboardStats, authMW.AuthorizeAny(authz.DashboardView))

	loggers.Main.Info("InitRouter: route registration completed")
}

// Файл: internal/routes/routes.go

package routes

import (
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"
	"request-system/pkg/service"
)

type Loggers struct {
	Main         *zap.Logger
	Auth         *zap.Logger
	Order        *zap.Logger
	User         *zap.Logger
	OrderHistory *zap.Logger
}

func InitRouter(e *echo.Echo, dbConn *pgxpool.Pool, redisClient *redis.Client, jwtSvc service.JWTService, loggers *Loggers, authPermissionService services.AuthPermissionServiceInterface, cfg *config.Config) {
	loggers.Main.Info("InitRouter: Начало создания маршрутов")

	// --- 0. ОБЩИЕ КОМПОНЕНТЫ ---
	api := e.Group("/api")
	authMW := middleware.NewAuthMiddleware(jwtSvc, authPermissionService, loggers.Auth)
	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		loggers.Main.Fatal("не удалось создать файловое хранилище", zap.Error(err))
	}
	txManager := repositories.NewTxManager(dbConn)

	// --- 1. РЕПОЗИТОРИИ ---
	userRepo := repositories.NewUserRepository(dbConn, loggers.User)
	roleRepo := repositories.NewRoleRepository(dbConn, loggers.Main)
	permissionRepo := repositories.NewPermissionRepository(dbConn, loggers.Main)
	statusRepo := repositories.NewStatusRepository(dbConn)
	rpRepo := repositories.NewRolePermissionRepository(dbConn)
	orderRepo := repositories.NewOrderRepository(dbConn, loggers.Order)
	priorityRepo := repositories.NewPriorityRepository(dbConn, loggers.Main)
	attachRepo := repositories.NewAttachmentRepository(dbConn)
	historyRepo := repositories.NewOrderHistoryRepository(dbConn)
	positionRepo := repositories.NewPositionRepository(dbConn)
	orderTypeRepo := repositories.NewOrderTypeRepository(dbConn)
	ruleRepo := repositories.NewOrderRoutingRuleRepository(dbConn)
	// ДОБАВЛЯЕМ РЕПОЗИТОРИЙ ОТЧЕТОВ
	reportRepo := repositories.NewReportRepository(dbConn)

	// --- 2. СЕРВИСЫ ---
	ruleEngineService := services.NewRuleEngineService(ruleRepo, userRepo, positionRepo, loggers.Main)

	roleService := services.NewRoleService(roleRepo, userRepo, statusRepo, authPermissionService, loggers.Main)
	permissionService := services.NewPermissionService(permissionRepo, userRepo, loggers.Main)
	userService := services.NewUserService(userRepo, roleRepo, permissionRepo, statusRepo, authPermissionService, loggers.User)
	rpService := services.NewRolePermissionService(rpRepo, userRepo, authPermissionService, loggers.Main)
	orderTypeService := services.NewOrderTypeService(orderTypeRepo, userRepo, txManager, ruleEngineService, loggers.Main)
	positionService := services.NewPositionService(positionRepo, userRepo, txManager, loggers.Main)
	orderRuleService := services.NewOrderRoutingRuleService(ruleRepo, userRepo, txManager, loggers.Main, orderTypeRepo)

	// !!! ИСПРАВЛЕН ВЫЗОВ `NewOrderService` !!!
	orderService := services.NewOrderService(
		// ПОРЯДОК ОЧЕНЬ ВАЖЕН!
		txManager,         // 1
		orderRepo,         // 2
		userRepo,          // 3
		statusRepo,        // 4
		priorityRepo,      // 5
		attachRepo,        // 6
		ruleEngineService, // 7
		historyRepo,       // 8
		fileStorage,       // 9
		loggers.Order,     // 10
		orderTypeRepo,     // 11
	)

	// ДОБАВЛЯЕМ СЕРВИС ОТЧЕТОВ
	reportService := services.NewReportService(reportRepo, userRepo)

	// --- 3. РОУТЕРЫ ---
	secureGroup := api.Group("", authMW.Auth)

	// ДОБАВЛЯЕМ ВЫЗОВ РОУТЕРА ОТЧЕТОВ
	runReportRouter(secureGroup, reportService, loggers.Main, authMW)

	runAuthRouter(api, dbConn, redisClient, jwtSvc, loggers.Auth, authMW, authPermissionService, cfg)

	// ... остальной код ...
	runUserRouter(secureGroup, userService, fileStorage, loggers.User, authMW)
	runRoleRouter(secureGroup, roleService, loggers.Main, authMW)
	runPermissionRouter(secureGroup, permissionService, loggers.Main, authMW)
	runRolePermissionRouter(secureGroup, rpService, loggers.Main, authMW)
	runOrderRouter(secureGroup, orderService, loggers.Order, authMW)
	runOrderTypeRouter(secureGroup, orderTypeService, loggers.Main, authMW)
	runPositionRouter(secureGroup, positionService, loggers.Main, authMW)
	runOrderRoutingRuleRouter(secureGroup, orderRuleService, loggers.Main, authMW)
	runAttachmentRouter(secureGroup, dbConn, fileStorage, loggers.Main)
	runStatusRouter(secureGroup, dbConn, loggers.Main, authMW, fileStorage)
	runOrderHistoryRouter(secureGroup, dbConn, loggers.OrderHistory, authMW)
	RunPriorityRouter(secureGroup, dbConn, loggers.Main, authMW)
	runDepartmentRouter(secureGroup, dbConn, loggers.Main, authMW)
	runOtdelRouter(secureGroup, dbConn, loggers.Main, authMW)
	runEquipmentTypeRouter(secureGroup, dbConn, loggers.Main, authMW)
	runBranchRouter(secureGroup, dbConn, loggers.Main, authMW)
	runOfficeRouter(secureGroup, dbConn, loggers.Main, authMW)
	runEquipmentRouter(secureGroup, dbConn, loggers.Main, authMW)

	loggers.Main.Info("INIT_ROUTER: Создание маршрутов завершено")
}

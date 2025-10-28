package routes

import (
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/controllers"
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
	historyRepo := repositories.NewOrderHistoryRepository(dbConn, loggers.OrderHistory)
	positionRepo := repositories.NewPositionRepository(dbConn)
	orderTypeRepo := repositories.NewOrderTypeRepository(dbConn)
	ruleRepo := repositories.NewOrderRoutingRuleRepository(dbConn)
	reportRepo := repositories.NewReportRepository(dbConn)

	// <<<--- ДОБАВЛЕНИЕ НЕДОСТАЮЩИХ РЕПОЗИТОРИЕВ
	branchRepo := repositories.NewBranchRepository(dbConn, loggers.Main)
	departmentRepo := repositories.NewDepartmentRepository(dbConn, loggers.Main)
	otdelRepo := repositories.NewOtdelRepository(dbConn, loggers.Main)
	officeRepo := repositories.NewOfficeRepository(dbConn)

	// --- 2. СЕРВИСЫ ---
	ruleEngineService := services.NewRuleEngineService(ruleRepo, userRepo, positionRepo, loggers.Main)
	roleService := services.NewRoleService(roleRepo, userRepo, statusRepo, authPermissionService, loggers.Main)
	permissionService := services.NewPermissionService(permissionRepo, userRepo, loggers.Main)
	userService := services.NewUserService(userRepo, roleRepo, permissionRepo, statusRepo, authPermissionService, loggers.User)
	rpService := services.NewRolePermissionService(rpRepo, userRepo, authPermissionService, loggers.Main)
	orderTypeService := services.NewOrderTypeService(orderTypeRepo, userRepo, txManager, ruleEngineService, loggers.Main)
	positionService := services.NewPositionService(positionRepo, userRepo, txManager, loggers.Main)
	orderRuleService := services.NewOrderRoutingRuleService(
		ruleRepo,
		userRepo,
		positionRepo,
		txManager,
		loggers.Main,
		orderTypeRepo,
	)
	orderService := services.NewOrderService(
		txManager, orderRepo, userRepo, statusRepo, priorityRepo, attachRepo,
		ruleEngineService, historyRepo, fileStorage, loggers.Order, orderTypeRepo,
	)
	historyService := services.NewOrderHistoryService(historyRepo, orderService, userRepo, loggers.OrderHistory)
	reportService := services.NewReportService(reportRepo, userRepo)

	// === 3. КОНТРОЛЛЕРЫ === (НОВЫЙ БЛОК)
	// Создаем ВСЕ контроллеры здесь, в одном месте.
	userController := controllers.NewUserController(userService, fileStorage, loggers.User)
	historyController := controllers.NewOrderHistoryController(historyService, orderService, loggers.OrderHistory)

	// <<<--- ДОБАВЛЕНИЕ НЕДОСТАЮЩИХ СЕРВИСОВ
	branchService := services.NewBranchService(branchRepo, userRepo, loggers.Main)
	departmentService := services.NewDepartmentService(departmentRepo, otdelRepo, userRepo, loggers.Main)
	otdelService := services.NewOtdelService(otdelRepo, userRepo, loggers.Main)
	officeService := services.NewOfficeService(officeRepo, loggers.Main)

	// --- 3. РОУТЕРЫ ---
	secureGroup := api.Group("", authMW.Auth)

	runReportRouter(secureGroup, reportService, loggers.Main, authMW)

	// <<<--- ИСПРАВЛЕННЫЙ ВЫЗОВ runAuthRouter
	runAuthRouter(api, dbConn, redisClient, jwtSvc, loggers.Auth, authMW, fileStorage, authPermissionService, cfg,
		positionService, branchService, departmentService, otdelService, officeService)

	runUserRouter(secureGroup, userController, authMW)
	runRoleRouter(secureGroup, roleService, loggers.Main, authMW)
	runPermissionRouter(secureGroup, permissionService, loggers.Main, authMW)
	runRolePermissionRouter(secureGroup, rpService, loggers.Main, authMW)
	runOrderRouter(secureGroup, orderService, loggers.Order, authMW)
	runOrderTypeRouter(secureGroup, orderTypeService, loggers.Main, authMW)
	runPositionRouter(secureGroup, positionService, loggers.Main, authMW)
	runOrderRoutingRuleRouter(secureGroup, orderRuleService, loggers.Main, authMW)
	runAttachmentRouter(secureGroup, dbConn, fileStorage, loggers.Main)
	runStatusRouter(secureGroup, dbConn, loggers.Main, authMW, fileStorage)
	runOrderHistoryRouter(secureGroup, historyController, authMW)
	RunPriorityRouter(secureGroup, dbConn, loggers.Main, authMW)
	runDepartmentRouter(secureGroup, dbConn, loggers.Main, authMW)
	runOtdelRouter(secureGroup, dbConn, loggers.Main, authMW)
	runEquipmentTypeRouter(secureGroup, dbConn, loggers.Main, authMW)
	runBranchRouter(secureGroup, dbConn, loggers.Main, authMW)

	runOfficeRouter(secureGroup, officeService, loggers.Main, authMW)

	runEquipmentRouter(secureGroup, dbConn, loggers.Main, authMW)

	loggers.Main.Info("INIT_ROUTER: Создание маршрутов завершено")
}

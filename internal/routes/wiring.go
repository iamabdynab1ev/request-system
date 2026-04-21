package routes

import (
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"

	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/eventbus"
	"request-system/pkg/filestorage"
	"request-system/pkg/service"
	"request-system/pkg/telegram"
	"request-system/pkg/websocket"
)

type routeRepositories struct {
	user           repositories.UserRepositoryInterface
	cache          repositories.CacheRepositoryInterface
	role           repositories.RoleRepositoryInterface
	permission     repositories.PermissionRepositoryInterface
	status         repositories.StatusRepositoryInterface
	rolePermission repositories.RolePermissionRepositoryInterface
	order          repositories.OrderRepositoryInterface
	priority       repositories.PriorityRepositoryInterface
	attachment     repositories.AttachmentRepositoryInterface
	history        repositories.OrderHistoryRepositoryInterface
	position       repositories.PositionRepositoryInterface
	orderType      repositories.OrderTypeRepositoryInterface
	rule           repositories.OrderRoutingRuleRepositoryInterface
	report         repositories.ReportRepositoryInterface
	branch         repositories.BranchRepositoryInterface
	department     repositories.DepartmentRepositoryInterface
	otdel          repositories.OtdelRepositoryInterface
	office         repositories.OfficeRepositoryInterface
	dashboard      repositories.DashboardRepositoryInterface
}

type routeServices struct {
	role           services.RoleServiceInterface
	permission     services.PermissionServiceInterface
	rolePermission services.RolePermissionServiceInterface
	orderType      services.OrderTypeServiceInterface
	position       services.PositionServiceInterface
	user           services.UserServiceInterface
	department     services.DepartmentServiceInterface
	otdel          services.OtdelServiceInterface
	orderRule      services.OrderRoutingRuleServiceInterface
	telegram       telegram.ServiceInterface
	order          services.OrderServiceInterface
	history        services.OrderHistoryServiceInterface
	branch         services.BranchServiceInterface
	office         services.OfficeServiceInterface
	dashboard      *services.DashboardService
}

type routeControllers struct {
	user      *controllers.UserController
	history   *controllers.OrderHistoryController
	websocket *controllers.WebSocketController
	dashboard *controllers.DashboardController
}

func buildRouteRepositories(
	dbConn *pgxpool.Pool,
	redisClient *redis.Client,
	loggers *Loggers,
) *routeRepositories {
	return &routeRepositories{
		user:           repositories.NewUserRepository(dbConn, loggers.User),
		cache:          repositories.NewRedisCacheRepository(redisClient),
		role:           repositories.NewRoleRepository(dbConn, loggers.Main),
		permission:     repositories.NewPermissionRepository(dbConn, loggers.Main),
		status:         repositories.NewStatusRepository(dbConn),
		rolePermission: repositories.NewRolePermissionRepository(dbConn),
		order:          repositories.NewOrderRepository(dbConn, loggers.Order),
		priority:       repositories.NewPriorityRepository(dbConn, loggers.Main),
		attachment:     repositories.NewAttachmentRepository(dbConn),
		history:        repositories.NewOrderHistoryRepository(dbConn, loggers.OrderHistory),
		position:       repositories.NewPositionRepository(dbConn, loggers.Main),
		orderType:      repositories.NewOrderTypeRepository(dbConn),
		rule:           repositories.NewOrderRoutingRuleRepository(dbConn),
		report:         repositories.NewReportRepository(dbConn, loggers.Main),
		branch:         repositories.NewBranchRepository(dbConn, loggers.Main),
		department:     repositories.NewDepartmentRepository(dbConn, loggers.Main),
		otdel:          repositories.NewOtdelRepository(dbConn, loggers.Main),
		office:         repositories.NewOfficeRepository(dbConn, loggers.Main),
		dashboard:      repositories.NewDashboardRepository(dbConn, loggers.Main),
	}
}

func buildRouteServices(
	repos *routeRepositories,
	txManager repositories.TxManagerInterface,
	fileStorage filestorage.FileStorageInterface,
	bus *eventbus.Bus,
	authPermissionService services.AuthPermissionServiceInterface,
	cfg *config.Config,
	loggers *Loggers,
) *routeServices {
	ruleEngineService := services.NewRuleEngineService(repos.rule, repos.user, loggers.Main)
	tgService := telegram.NewService(cfg.Telegram.BotToken)
	notificationService := services.NewTelegramNotificationService(tgService, loggers.Main)
	_ = services.NewReportService(repos.report, repos.user, loggers.Main)

	return &routeServices{
		role:           services.NewRoleService(repos.role, repos.user, repos.status, authPermissionService, loggers.Main),
		permission:     services.NewPermissionService(repos.permission, repos.user, loggers.Main),
		rolePermission: services.NewRolePermissionService(repos.rolePermission, repos.user, authPermissionService, loggers.Main),
		orderType:      services.NewOrderTypeService(repos.orderType, repos.user, txManager, ruleEngineService, loggers.Main),
		position:       services.NewPositionService(repos.position, repos.user, txManager, loggers.Main),
		user:           services.NewUserService(txManager, repos.user, repos.otdel, repos.role, repos.permission, repos.status, repos.cache, authPermissionService, loggers.User),
		department:     services.NewDepartmentService(txManager, repos.department, repos.user, loggers.Main),
		otdel:          services.NewOtdelService(txManager, repos.otdel, repos.user, loggers.Main),
		orderRule:      services.NewOrderRoutingRuleService(repos.rule, repos.user, repos.position, txManager, loggers.Main, repos.orderType),
		telegram:       tgService,
		order: services.NewOrderService(
			txManager,
			repos.order,
			repos.user,
			repos.status,
			repos.priority,
			repos.attachment,
			ruleEngineService,
			repos.history,
			fileStorage,
			bus,
			loggers.Order,
			repos.orderType,
			authPermissionService,
			notificationService,
			repos.cache,
		),
		history: services.NewOrderHistoryService(
			repos.history,
			repos.user,
			repos.department,
			repos.otdel,
			repos.branch,
			repos.office,
			repos.status,
			repos.priority,
			loggers.OrderHistory,
		),
		branch:    services.NewBranchService(txManager, repos.branch, repos.user, loggers.Main),
		office:    services.NewOfficeService(repos.office, repos.user, txManager, loggers.Main),
		dashboard: services.NewDashboardService(repos.dashboard, repos.user, repos.cache, loggers.Main),
	}
}

func buildRouteControllers(
	servicesBundle *routeServices,
	adService services.ADServiceInterface,
	fileStorage filestorage.FileStorageInterface,
	wsHub *websocket.Hub,
	jwtSvc service.JWTService,
	cfg *config.Config,
	loggers *Loggers,
) *routeControllers {
	return &routeControllers{
		user:      controllers.NewUserController(servicesBundle.user, adService, fileStorage, loggers.User),
		history:   controllers.NewOrderHistoryController(servicesBundle.history, servicesBundle.order, loggers.OrderHistory),
		websocket: controllers.NewWebSocketController(wsHub, jwtSvc, loggers.Main, cfg.Server.AllowedOrigins),
		dashboard: controllers.NewDashboardController(
			servicesBundle.dashboard,
			loggers.Main.Named("Dashboard"),
		),
	}
}

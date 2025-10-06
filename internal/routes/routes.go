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

	// --- 1. СОЗДАЕМ ВСЕ РЕПОЗИТОРИИ В ОДНОМ МЕСТЕ ---
	userRepository := repositories.NewUserRepository(dbConn, loggers.User)
	roleRepository := repositories.NewRoleRepository(dbConn, loggers.Main)
	permissionRepository := repositories.NewPermissionRepository(dbConn, loggers.Main)
	statusRepository := repositories.NewStatusRepository(dbConn)
	rpRepository := repositories.NewRolePermissionRepository(dbConn)

	// --- 2. СОЗДАЕМ ВСЕ СЕРВИСЫ В ОДНОМ МЕСТЕ ---
	roleService := services.NewRoleService(roleRepository, userRepository, statusRepository, authPermissionService, loggers.Main)
	permissionService := services.NewPermissionService(permissionRepository, userRepository, loggers.Main)
	userService := services.NewUserService(userRepository, roleRepository, permissionRepository, statusRepository, authPermissionService, loggers.User)
	rpService := services.NewRolePermissionService(rpRepository, userRepository, authPermissionService, loggers.Main)

	// --- 3. ИНИЦИАЛИЗИРУЕМ ОСТАЛЬНЫЕ КОМПОНЕНТЫ И ВЫЗЫВАЕМ РОУТЕРЫ ---
	runAuthRouter(api, dbConn, redisClient, jwtSvc, loggers.Auth, authMW, authPermissionService, cfg)
	secureGroup := api.Group("", authMW.Auth)
	fileStorage, err := filestorage.NewLocalFileStorage("uploads")
	if err != nil {
		loggers.Main.Fatal("не удалось создать файловое хранилище", zap.Error(err))
	}

	// ----- ИСПРАВЛЕННЫЙ ВЫЗОВ ЗДЕСЬ -----
	runUserRouter(secureGroup, userService, fileStorage, loggers.User, authMW)
	// ----- КОНЕЦ ИСПРАВЛЕНИЙ -----

	runRoleRouter(secureGroup, roleService, loggers.Main, authMW)
	runPermissionRouter(secureGroup, permissionService, loggers.Main, authMW)
	runRolePermissionRouter(secureGroup, rpService, loggers.Main, authMW)

	// --- 4. ОСТАЛЬНЫЕ РОУТЕРЫ (без изменений, но их можно перевести на DI позже) ---
	runAttachmentRouter(secureGroup, dbConn, fileStorage, loggers.Main)
	runStatusRouter(secureGroup, dbConn, loggers.Main, authMW, fileStorage)
	runOrderRouter(secureGroup, dbConn, loggers.Order, authMW)
	runOrderHistoryRouter(secureGroup, dbConn, loggers.OrderHistory, authMW)
	RunPriorityRouter(secureGroup, dbConn, loggers.Main, authMW)
	runDepartmentRouter(secureGroup, dbConn, loggers.Main, authMW)
	runOtdelRouter(secureGroup, dbConn, loggers.Main, authMW)
	runEquipmentTypeRouter(secureGroup, dbConn, loggers.Main, authMW)
	runBranchRouter(secureGroup, dbConn, loggers.Main, authMW)
	runOfficeRouter(secureGroup, dbConn, loggers.Main, authMW)
	runEquipmentRouter(secureGroup, dbConn, loggers.Main, authMW)
	runPositionRouter(secureGroup, dbConn, loggers.Main)

	loggers.Main.Info("INIT_ROUTER: Создание маршрутов завершено")
}

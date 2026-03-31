package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/internal/sync"
	"request-system/pkg/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func runSyncRouter(
	apiGroup *echo.Group,
	dbConn *pgxpool.Pool,
	cfg *config.Config,
	loggers *Loggers,
) {
	loggers.Main.Info("Инициализация роутера для синхронизации c 1С...")

	txManager := repositories.NewTxManager(dbConn, loggers.Main)
	branchRepo := repositories.NewBranchRepository(dbConn, loggers.Main)
	officeRepo := repositories.NewOfficeRepository(dbConn, loggers.Main)
	statusRepo := repositories.NewStatusRepository(dbConn)
	departmentRepo := repositories.NewDepartmentRepository(dbConn, loggers.Main)
	otdelRepo := repositories.NewOtdelRepository(dbConn, loggers.Main)
	positionRepo := repositories.NewPositionRepository(dbConn, loggers.Main)
	userRepo := repositories.NewUserRepository(dbConn, loggers.User)
	roleRepo := repositories.NewRoleRepository(dbConn, loggers.Main)

	dbHandler := sync.NewDBHandler(
		txManager,
		branchRepo,
		officeRepo,
		statusRepo,
		departmentRepo,
		otdelRepo,
		positionRepo,
		userRepo,
		roleRepo,
		&cfg.Integrations,
		loggers.Main,
	)

	syncService := services.NewSyncService(dbHandler, loggers.Main)
	syncController := controllers.NewSyncController(syncService, loggers.Main)

	syncGroup := apiGroup.Group("/sync")

	apiKey := cfg.Integrations.OneCApiKey
	if apiKey == "" {
		loggers.Main.Warn("API-ключ для синхронизации с 1С (ONE_C_API_KEY) не установлен! Эндпоинт не защищен.")
	} else {
		syncGroup.Use(middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
			return key == apiKey, nil
		}))
	}

	syncGroup.POST("/1c", syncController.HandleSyncFrom1C)
}

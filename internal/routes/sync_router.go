package routes

import (
	"strings"

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
	if strings.TrimSpace(cfg.Integrations.OneCApiKey) == "" {
		loggers.Main.Error("ONE_C_API_KEY не установлен: роут /api/sync/1c отключен")
		return
	}

	apiKey := strings.TrimSpace(cfg.Integrations.OneCApiKey)
	syncGroup.Use(middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
		return key == apiKey, nil
	}))

	syncGroup.POST("/1c", syncController.HandleSyncFrom1C)
}

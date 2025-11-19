// Файл: internal/routes/sync_router.go
package routes

import (
	// Больше не нужен: "request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/internal/sync"
	"request-system/pkg/config"

	// Больше не нужен: "request-system/pkg/middleware"
	// Больше не нужен: "request-system/internal/integrations"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func runSyncRouter(
	apiGroup *echo.Group, // Теперь эндпоинт будет в общей группе /api, а не в защищенной
	dbConn *pgxpool.Pool,
	cfg *config.Config, // Принимаем весь конфиг для доступа к API-ключу
	loggers *Loggers,
) {
	loggers.Main.Info("Инициализация роутера для синхронизации c 1С...")

	// --- 1. Инициализация всех репозиториев для справочников ---
	// Вам нужно будет создать конструкторы New...Repository для недостающих.
	txManager := repositories.NewTxManager(dbConn, loggers.Main)
	branchRepo := repositories.NewBranchRepository(dbConn, loggers.Main)
	officeRepo := repositories.NewOfficeRepository(dbConn, loggers.Main)
	statusRepo := repositories.NewStatusRepository(dbConn)
	departmentRepo := repositories.NewDepartmentRepository(dbConn, loggers.Main)
	otdelRepo := repositories.NewOtdelRepository(dbConn, loggers.Main)
	positionRepo := repositories.NewPositionRepository(dbConn, loggers.Main)
	userRepo := repositories.NewUserRepository(dbConn, loggers.User)
	roleRepo := repositories.NewRoleRepository(dbConn, loggers.Main)

	// --- 2. Собираем наши компоненты в правильном порядке ---

	// DBHandler теперь принимает полный набор репозиториев.
	dbHandler := sync.NewDBHandler(
		txManager,
		branchRepo,
		officeRepo,
		statusRepo,
		departmentRepo,
		otdelRepo,
		positionRepo,
		userRepo, roleRepo,
		&cfg.Integrations,
		loggers.Main,
	)

	// SyncService стал проще, ему больше не нужен registry от onlinebank.
	syncService := services.NewSyncService(dbHandler, loggers.Main)
	syncController := controllers.NewSyncController(syncService, loggers.Main)

	// --- 3. Создаем группу для вебхуков и защищаем ее ---
	// Новый эндпоинт будет доступен по POST /api/sync/1c
	syncGroup := apiGroup.Group("/sync")

	// Защита с помощью API-ключа, который должен быть в вашем .env и config.go
	apiKey := cfg.Integrations.OneCApiKey // Предполагаем, что поле будет здесь
	if apiKey == "" {
		loggers.Main.Warn("API-ключ для синхронизации с 1С (ONE_C_API_KEY) не установлен! Эндпоинт не защищен.")
	} else {
		syncGroup.Use(middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
			return key == apiKey, nil
		}))
	}

	// --- 4. Регистрируем единственный маршрут для вебхука 1С ---
	syncGroup.POST("/1c", syncController.HandleSyncFrom1C)

	// Старый маршрут /sync/run полностью удален.
}

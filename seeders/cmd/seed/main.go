package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // Нужен для Goose
	"github.com/pressly/goose/v3"

	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"request-system/internal/listeners"
	"request-system/internal/repositories"
	"request-system/internal/routes"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/database/postgresql"
	"request-system/pkg/eventbus"
	"request-system/pkg/logger"
	"request-system/pkg/service"
	"request-system/pkg/telegram"
	"request-system/pkg/validation"
	"request-system/pkg/websocket"
	"request-system/seeders"
)

func main() {
	// =========================================================================
	// 1. ПАРСИНГ ФЛАГОВ (Режим наполнения/Сидеры)
	// =========================================================================
	runCore := flag.Bool("core", false, "Запустить наполнение базовых справочников (статусы, права и т.д.)")
	runRoles := flag.Bool("roles", false, "Запустить создание ролей и Супер-Администратора")
	runEquipment := flag.Bool("equipment", false, "Запустить наполнение справочников оборудования")
	runAll := flag.Bool("all", false, "Запустить все сидеры (эквивалентно -core -roles -equipment)")

	flag.Parse()

	// Загружаем конфиг (он нужен и для сидеров, и для сервера)
	cfg := config.New()

	// Проверяем, запущен ли режим сидеров
	isSeederMode := *runCore || *runRoles || *runEquipment || *runAll

	if isSeederMode {
		log.Println("🛠️ ЗАПУСК В РЕЖИМЕ НАПОЛНЕНИЯ (SEEDERS MODE)...")
		log.Println("Используется DSN для сидера:", cfg.Postgres.DSN)

		dbPool := postgresql.ConnectDB(cfg.Postgres.DSN)
		defer dbPool.Close()

		log.Println("======================================================")

		if *runAll || *runCore {
			seeders.SeedCoreDictionaries(dbPool)
			log.Println("======================================================")
		}

		if *runAll || *runEquipment {
			seeders.SeedEquipmentData(dbPool)
			log.Println("======================================================")
		}

		if *runAll || *runRoles {
			// Передаем и Pool, и Config (чтобы достать пароль Рута)
			seeders.SeedRolesAndAdmin(dbPool, cfg)
			log.Println("======================================================")
		}

		log.Println("✅ Все операции сидирования успешно завершены.")
		log.Println("Программа завершает работу (сервер не запускается в этом режиме).")
		return // <--- ВАЖНО: Выходим, не запуская сервер
	}

	// =========================================================================
	// 2. ОБЫЧНЫЙ ЗАПУСК СЕРВЕРА (если флагов нет)
	// =========================================================================

	// --- Логгеры ---
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	mainLogger, err := logger.CreateLogger(logLevel, "system")
	if err != nil {
		panic("Не удалось создать главный логгер")
	}

	// --- БЛОК МИГРАЦИЙ (Goose) ---
	mainLogger.Info("Запуск проверки и применения миграций...")
	dbForGoose, err := sql.Open("pgx", cfg.Postgres.DSN)
	if err != nil {
		mainLogger.Fatal("Не удалось создать подключение к БД для миграций", zap.Error(err))
	}
	defer dbForGoose.Close()

	if err := dbForGoose.Ping(); err != nil {
		mainLogger.Fatal("Не удалось проверить соединение с БД для миграций", zap.Error(err))
	}

	if err := goose.SetDialect("postgres"); err != nil {
		mainLogger.Fatal("Ошибка установки диалекта для goose", zap.Error(err))
	}

	// Запускаем миграции, чтобы таблицы точно существовали
	if err := goose.Up(dbForGoose, "./database/migrations"); err != nil {
		mainLogger.Fatal("Ошибка применения миграций", zap.Error(err))
	}
	mainLogger.Info("Миграции успешно проверены/применены.")

	// --- Логгеры сервисов ---
	authLogger, _ := logger.CreateLogger(logLevel, "auth")
	orderLogger, _ := logger.CreateLogger(logLevel, "orders")
	userLogger, _ := logger.CreateLogger(logLevel, "users")
	orderHistoryLogger, _ := logger.CreateLogger(logLevel, "order_history")

	appLoggers := &routes.Loggers{
		Main:         mainLogger,
		Auth:         authLogger,
		Order:        orderLogger,
		User:         userLogger,
		OrderHistory: orderHistoryLogger,
	}

	// --- Инициализация Echo ---
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "pong"})
	})
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		DisableStackAll: false, StackSize: 8 << 10,
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			mainLogger.Error("!!! ПАНИКА В ПРИЛОЖЕНИИ !!!", zap.Error(err), zap.String("stack", string(stack)))
			return err
		},
	}))

	// --- Middlewares ---
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.Server.AllowedOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "ngrok-skip-browser-warning"},
		AllowCredentials: true,
	}))

	absPath, err := filepath.Abs("./uploads")
	if err != nil {
		mainLogger.Fatal("Не удалось получить путь к ./uploads", zap.Error(err))
	}
	e.Static("/uploads", absPath)
	e.Validator = validation.New()

	// --- Подключение к базам данных ---
	dbConn := postgresql.ConnectDB(cfg.Postgres.DSN)
	defer dbConn.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       0,
	})
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		mainLogger.Fatal("Не удалось подключиться к Redis", zap.Error(err))
	}

	// --- Инициализация слоев (Repo, Service, Handler) ---
	jwtSvc := service.NewJWTService(cfg.JWT.SecretKey, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL, authLogger)
	permissionRepo := repositories.NewPermissionRepository(dbConn, mainLogger)
	cacheRepo := repositories.NewRedisCacheRepository(redisClient)
	authPermissionService := services.NewAuthPermissionService(permissionRepo, cacheRepo, authLogger, 10*time.Minute)

	// Шина событий
	bus := eventbus.New(mainLogger)

	// WebSocket
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Уведомления
	tgService := telegram.NewService(cfg.Telegram.BotToken)
	notificationService := services.NewTelegramNotificationService(tgService, mainLogger)
	wsNotificationService := services.NewWebSocketNotificationService(wsHub, mainLogger.Named("WebSocketNotifier"))

	// Слушатель событий (Notification Listener)
	userRepoForListener := repositories.NewUserRepository(dbConn, userLogger)
	statusRepoForListener := repositories.NewStatusRepository(dbConn)
	priorityRepoForListener := repositories.NewPriorityRepository(dbConn, mainLogger)

	notificationListener := listeners.NewNotificationListener(
		notificationService,
		wsNotificationService,
		userRepoForListener,
		statusRepoForListener,
		priorityRepoForListener,
		cfg.Frontend,
		cfg.Server,
		mainLogger.Named("NotificationListener"),
	)
	notificationListener.Register(bus)

	// Active Directory Service
	adLogger, _ := logger.CreateLogger(logLevel, "ad_service")
	adService := services.NewADService(&cfg.LDAP, adLogger)

	// --- Запуск маршрутизатора ---
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	routes.InitRouter(e, dbConn, redisClient, jwtSvc, appLoggers, authPermissionService, cfg, bus, wsHub, adService, appCtx)
	serverAddress := ":" + cfg.Server.Port

	mainLogger.Info("🚀 Сервер запущен на " + serverAddress)

	// --- Graceful Shutdown ---
	go func() {
		if err := e.Start(serverAddress); err != nil && err != http.ErrServerClosed {
			mainLogger.Fatal("Не удалось запустить сервер", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	mainLogger.Info("🛑 Получен сигнал завершения, начинаем graceful shutdown...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		mainLogger.Error("Ошибка при остановке сервера", zap.Error(err))
	}

	mainLogger.Info("✅ Сервер корректно остановлен")
}
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
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
	"request-system/seeders" // Важно!
)

func main() {
	// ==========================================================
	// 1. ИНИЦИАЛИЗАЦИЯ И НАСТРОЙКА СРЕДЫ
	// ==========================================================

	// Настройка прокси банка (в коде)
	os.Setenv("HTTP_PROXY", "http://192.168.10.42:3128")
	os.Setenv("HTTPS_PROXY", "http://192.168.10.42:3128")
	// Важные исключения: база данных и внутренние сервера идут без прокси!
	os.Setenv("NO_PROXY", "localhost,127.0.0.1,192.168.10.79,arvand.local,192.168.10.42,192.168.8.106")

	// Флаги для режима сидеров
	runCore := flag.Bool("core", false, "Наполнение базовых справочников")
	runRoles := flag.Bool("roles", false, "Создание ролей и Рут-Админа")
	runEquipment := flag.Bool("equipment", false, "Наполнение оборудования")
	runAll := flag.Bool("all", false, "Запустить все сидеры сразу")
	flag.Parse()

	// Загружаем настройки (.env)
	cfg := config.New()

	// ==========================================================
	// 2. БЛОК СИДЕРОВ (Если запущены с флагом, сервер НЕ стартуем)
	// ==========================================================
	if *runCore || *runRoles || *runEquipment || *runAll {
		log.Println("🛠️ ЗАПУСК СИДЕРОВ (Наполнение базы)...")

		// Подключаемся к базе для сидов
		dbPool := postgresql.ConnectDB(cfg.Postgres.DSN)
		defer dbPool.Close()

		if *runAll || *runCore {
			seeders.SeedCoreDictionaries(dbPool)
		}

		if *runAll || *runRoles {
			// Передаем и конфиг, чтобы знать пароль Root!
			seeders.SeedRolesAndAdmin(dbPool, cfg)
		}

		log.Println("✅ Все сидеры выполнены успешно. Выход.")
		return // ЗАВЕРШАЕМ ПРОГРАММУ
	}

	// ==========================================================
	// 3. ОБЫЧНЫЙ ЗАПУСК СЕРВЕРА
	// ==========================================================

	// Логгеры
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	mainLogger, err := logger.CreateLogger(logLevel, "system")
	if err != nil {
		panic("Не удалось создать логгер")
	}

	// Миграции (Goose)
	mainLogger.Info("Запуск миграций Goose...")
	dbGoose, err := sql.Open("pgx", cfg.Postgres.DSN)
	if err != nil {
		mainLogger.Fatal("Ошибка соединения для миграций", zap.Error(err))
	}
	defer dbGoose.Close()

	if err := goose.SetDialect("postgres"); err == nil {
		// Миграции будут искать папку "database/migrations" относительно запускаемого .exe
		if err := goose.Up(dbGoose, "./database/migrations"); err != nil {
			mainLogger.Error("Внимание: ошибка миграций (возможно они уже накатаны)", zap.Error(err))
		}
	}

	// Остальные логгеры
	authLogger, _ := logger.CreateLogger(logLevel, "auth")
	orderLogger, _ := logger.CreateLogger(logLevel, "orders")
	userLogger, _ := logger.CreateLogger(logLevel, "users")
	orderHistoryLogger, _ := logger.CreateLogger(logLevel, "order_history")

	appLoggers := &routes.Loggers{
		Main: mainLogger, Auth: authLogger, Order: orderLogger, User: userLogger, OrderHistory: orderHistoryLogger,
	}

	// Настройка Echo
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	// Важно для фронта: CORS
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.Server.AllowedOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodHead},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			echo.HeaderXRequestedWith,
			"ngrok-skip-browser-warning",
		},
		AllowCredentials: true,
	}))

	e.Validator = validation.New()

	// Подключение к основной БД (Pool)
	dbConn := postgresql.ConnectDB(cfg.Postgres.DSN)
	defer dbConn.Close()
	e.Static("/uploads", "uploads")
	// Подключение к Redis
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.Redis.Address, Password: cfg.Redis.Password})

	// Сервисы
	jwtSvc := service.NewJWTService(cfg.JWT.SecretKey, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL, authLogger)
	permissionRepo := repositories.NewPermissionRepository(dbConn, mainLogger)
	cacheRepo := repositories.NewRedisCacheRepository(redisClient)
	authPermissionService := services.NewAuthPermissionService(permissionRepo, cacheRepo, authLogger, 10*time.Minute)

	bus := eventbus.New(mainLogger)
	wsHub := websocket.NewHub()
	go wsHub.Run()

	tgService := telegram.NewService(cfg.Telegram.BotToken)
	notificationService := services.NewTelegramNotificationService(tgService, mainLogger)
	wsNotificationService := services.NewWebSocketNotificationService(wsHub, mainLogger.Named("WebSocketNotifier"))

	notificationListener := listeners.NewNotificationListener(
		notificationService, wsNotificationService,
		repositories.NewUserRepository(dbConn, userLogger),
		repositories.NewStatusRepository(dbConn),
		repositories.NewPriorityRepository(dbConn, mainLogger),
		cfg.Frontend, cfg.Server, mainLogger.Named("NotificationListener"),
	)
	notificationListener.Register(bus)

	adService := services.NewADService(&cfg.LDAP, mainLogger)

	// Роутинг
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	routes.InitRouter(e, dbConn, redisClient, jwtSvc, appLoggers, authPermissionService, cfg, bus, wsHub, adService, appCtx)

	// ==========================================================
	// 4. ЗАПУСК СЕРВЕРА HTTPS (StartTLS)
	// ==========================================================

	serverAddress := ":" + cfg.Server.Port

	// Проверяем наличие сертификатов
	certPath := cfg.Server.CertFile
	keyPath := cfg.Server.KeyFile

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		mainLogger.Fatal("Не найден файл сертификата! Проверьте SSL_CERT_PATH", zap.String("path", certPath))
	}

	go func() {
		// Запуск в безопасном режиме
		if err := e.StartTLS(serverAddress, certPath, keyPath); err != nil && err != http.ErrServerClosed {
			mainLogger.Fatal("🔴 Ошибка запуска HTTPS", zap.Error(err))
		}
	}()

	mainLogger.Info("🚀 HTTPS СЕРВЕР ЗАПУЩЕН УСПЕШНО")
	mainLogger.Info("🔗 Адрес (Локально): https://localhost" + serverAddress + "/ping")
	mainLogger.Info("🔗 Адрес (На сервере): https://192.168.10.79" + serverAddress + "/ping")

	// Graceful Shutdown (Красивое выключение при Ctrl+C)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	mainLogger.Info("🛑 Получен сигнал завершения...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		mainLogger.Error("Ошибка при остановке сервера", zap.Error(err))
	}

	mainLogger.Info("✅ Сервер корректно остановлен")
}

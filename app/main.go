package main

import (
	"context"
	"crypto/tls" 
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
	"request-system/seeders"
)

func main() {
 loc, err := time.LoadLocation("Asia/Tashkent")
if err != nil {
    log.Printf("⚠️ Не удалось загрузить Asia/Tashkent: %v", err)
    loc = time.Local
}
time.Local = loc
log.Printf("✅ Временная зона установлена: %s", time.Local.String())

	os.Setenv("HTTP_PROXY", "http://192.168.10.42:3128")
	os.Setenv("HTTPS_PROXY", "http://192.168.10.42:3128")

	os.Setenv("NO_PROXY", "localhost,127.0.0.1,192.168.10.79,arvand.local,192.168.10.42,192.168.10.15,192.168.10.14")
	
	runCore := flag.Bool("core", false, "Наполнение базовых справочников")
	runRoles := flag.Bool("roles", false, "Создание ролей и Рут-Админа")
	runAll := flag.Bool("all", false, "Запустить все сидеры сразу")

	importAtms := flag.String("import-atms", "", "Путь к файлу банкоматов .xlsx")
    importTerms := flag.String("import-terms", "", "Путь к файлу терминалов .xlsx")
    importPos := flag.String("import-pos", "", "Путь к файлу ПОС-терминалов .xlsx")


	flag.Parse()

	// Загружаем настройки (.env)
	cfg := config.New()

	// 3. БЛОК СИДЕРОВ И ИМПОРТА (Работает как сидер, если есть хоть один флаг)
    if *runCore || *runRoles || *runAll || *importAtms != "" || *importTerms != "" || *importPos != "" {
        log.Println("🛠️ ЗАПУСК ОПЕРАЦИИ СИДИРОВАНИЯ/ИМПОРТА...")
        dbPool := postgresql.ConnectDB(cfg.Postgres.DSN)
        defer dbPool.Close()

        // Сидеры (Базовые данные)
        if *runAll || *runCore { seeders.SeedCoreDictionaries(dbPool) }
        if *runAll || *runRoles { seeders.SeedRolesAndAdmin(dbPool, cfg) }

      // --- ЛОГИКА ИМПОРТА ИЗ EXCEL ---
        if *importAtms != "" || *importTerms != "" || *importPos != "" {
            log.Println("📥 Запуск процесса импорта оборудования...")
            svc := services.NewEquipImportService(dbPool)

            if *importAtms != ""  { 
                log.Printf("📄 Файл АТМ: %s", *importAtms)
                if err := svc.ImportAtms(*importAtms); err != nil {
                    log.Printf("❌ Ошибка при импорте АТМ: %v", err)
                }
            }
            if *importTerms != "" { 
                log.Printf("📄 Файл Терминалы: %s", *importTerms)
                if err := svc.ImportTerminals(*importTerms); err != nil {
                    log.Printf("❌ Ошибка при импорте терминалов: %v", err)
                }
            }
            if *importPos != ""   { 
                log.Printf("📄 Файл ПОС-терминалы: %s", *importPos)
                if err := svc.ImportPos(*importPos); err != nil {
                    log.Printf("❌ Ошибка при импорте ПОС-терминалов: %v", err)
                }
            }
        }

        log.Println("✅ Все операции выполнены успешно.")
        return
    }

	
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" { logLevel = "info" }
	
	mainLogger, err := logger.CreateLogger(logLevel, "system")
	if err != nil { panic("Не удалось создать логгер") }

	// Миграции (Goose)
	mainLogger.Info("Запуск миграций Goose...")
	dbGoose, err := sql.Open("pgx", cfg.Postgres.DSN)
	if err != nil {
		mainLogger.Fatal("Ошибка соединения для миграций", zap.Error(err))
	}
	defer dbGoose.Close()

	if err := goose.SetDialect("postgres"); err == nil {
		if err := goose.Up(dbGoose, "./database/migrations"); err != nil {
			mainLogger.Error("Внимание: ошибка миграций (возможно они уже накатаны)", zap.Error(err))
		}
	}

	authLogger, _ := logger.CreateLogger(logLevel, "auth")
	orderLogger, _ := logger.CreateLogger(logLevel, "orders")
	userLogger, _ := logger.CreateLogger(logLevel, "users")
	orderHistoryLogger, _ := logger.CreateLogger(logLevel, "order_history")

	appLoggers := &routes.Loggers{Main: mainLogger, Auth: authLogger, Order: orderLogger, User: userLogger, OrderHistory: orderHistoryLogger}

	// Настройка Echo
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	
	// CORS: Разрешаем куки и заголовки
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.Server.AllowedOrigins, // Берется из .env (исправленного на Шаге 1)
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodHead},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-Requested-With", "ngrok-skip-browser-warning"},
		AllowCredentials: true,
	}))

	e.Validator = validation.New()

	dbConn := postgresql.ConnectDB(cfg.Postgres.DSN)
	defer dbConn.Close()
	e.Static("/uploads", "uploads")
	
	redisClient := redis.NewClient(&redis.Options{Addr: cfg.Redis.Address, Password: cfg.Redis.Password})

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

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	routes.InitRouter(e, dbConn, redisClient, jwtSvc, appLoggers, authPermissionService, cfg, bus, wsHub, adService, appCtx)


	serverAddress := ":" + cfg.Server.Port
	certPath := cfg.Server.CertFile
	keyPath := cfg.Server.KeyFile


	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		
	}

	s := &http.Server{
		Addr:      serverAddress,
		Handler:   e,
		TLSConfig: tlsConfig,
	}

	go func() {
		// Запуск сервера вручную через http.Server
		if err := s.ListenAndServeTLS(certPath, keyPath); err != nil && err != http.ErrServerClosed {
			mainLogger.Fatal("🔴 Ошибка запуска HTTPS", zap.Error(err))
		}
	}()

	mainLogger.Info("🚀 HTTPS СЕРВЕР ЗАПУЩЕН (ПОРТ "+cfg.Server.Port+")")
	mainLogger.Info("🔗 Local: https://localhost" + serverAddress + "/ping")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	mainLogger.Info("🛑 Остановка сервера...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		mainLogger.Error("Error shutdown", zap.Error(err))
	}
}

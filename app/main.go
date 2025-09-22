package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"request-system/internal/repositories"
	"request-system/internal/routes"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/customvalidator"
	"request-system/pkg/database/postgresql"
	"request-system/pkg/logger" // <<<--- ИСПОЛЬЗУЕМ ИМПОРТ БЕЗ ПСЕВДОНИМА
	"request-system/pkg/service"
	"request-system/pkg/utils"
)

func main() {
	// 1. КОНФИГ
	cfg := config.New()

	// 2. ЛОГГЕРЫ
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	mainLogger, err := logger.CreateLogger(logLevel, "system") // <-- Вызываем через имя пакета
	if err != nil {
		panic("Не удалось создать главный логгер")
	}

	authLogger, err := logger.CreateLogger(logLevel, "auth")
	if err != nil {
		panic("Не удалось создать логгер 'auth'")
	}

	orderLogger, err := logger.CreateLogger(logLevel, "orders")
	if err != nil {
		panic("Не удалось создать логгер 'orders'")
	}

	userLogger, err := logger.CreateLogger(logLevel, "users")
	if err != nil {
		panic("Не удалось создать логгер 'users'")
	}

	orderHistoryLogger, err := logger.CreateLogger(logLevel, "order_history")
	if err != nil {
		panic("Не удалось создать логгер 'order_history'")
	}

	appLoggers := &routes.Loggers{
		Main:         mainLogger,
		Auth:         authLogger,
		Order:        orderLogger,
		User:         userLogger,
		OrderHistory: orderHistoryLogger,
	}

	// 3. НАСТРОЙКА ECHO
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

	// 4. НАСТРОЙКА MIDDLEWARES
	allowedOrigins := []string{
		"http://localhost:4040", "http://10.98.102.66:4040", "http://10.98.102.66",
		"http://helpdesk.local", "https://a089b2344e17.ngrok-free.app",
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc:  func(origin string) (bool, error) { return slices.Contains(allowedOrigins, origin), nil },
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "ngrok-skip-browser-warning"},
		AllowCredentials: true,
	}))

	absPath, err := filepath.Abs("./uploads")
	if err != nil {
		mainLogger.Fatal("Не удалось получить путь к ./uploads", zap.Error(err))
	}
	e.Static("/uploads", absPath)

	v := validator.New()
	if err := customvalidator.RegisterCustomValidations(v); err != nil {
		mainLogger.Fatal("Не удалось зарегистрировать валидаторы", zap.Error(err))
	}
	e.Validator = utils.NewValidator(v)

	// 5. ПОДКЛЮЧЕНИЯ
	dbConn := postgresql.ConnectDB()
	defer dbConn.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       0,
	})
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		mainLogger.Fatal("Не удалось подключиться к Redis", zap.Error(err))
	}

	// 6. ИНИЦИАЛИЗАЦИЯ СЕРВИСОВ И ЗАПУСК РОУТЕРОВ
	jwtSvc := service.NewJWTService(cfg.JWT.SecretKey, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL, authLogger)
	permissionRepo := repositories.NewPermissionRepository(dbConn, mainLogger)
	cacheRepo := repositories.NewRedisCacheRepository(redisClient)
	authPermissionService := services.NewAuthPermissionService(permissionRepo, cacheRepo, authLogger, 10*time.Minute)

	routes.InitRouter(e, dbConn, redisClient, jwtSvc, appLoggers, authPermissionService, cfg)

	// 7. ЗАПУСК СЕРВЕРЕРА
	mainLogger.Info("🚀 Сервер запущен на :8080")
	if err := e.Start(":8080"); err != nil {
		mainLogger.Fatal("Не удалось запустить сервер", zap.Error(err))
	}
}

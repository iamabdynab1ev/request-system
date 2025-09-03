// Файл: main.go

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"request-system/internal/repositories"
	"request-system/internal/routes"
	"request-system/internal/services"
	"request-system/pkg/config"
	"request-system/pkg/customvalidator" // Убедитесь, что этот импорт есть
	"request-system/pkg/database/postgresql"
	apperrors "request-system/pkg/errors"
	applogger "request-system/pkg/logger"
	"request-system/pkg/service"
	"request-system/pkg/utils"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

// !!! ВСЕ ФУНКЦИИ-ВАЛИДАТОРЫ (isGoodEmailFormat, isTajikPhoneNumber и т.д.) ОТСЮДА УДАЛЕНЫ !!!
// Теперь они живут в pkg/customvalidator/validators.go

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found or could not be loaded.")
	}

	// 1. СНАЧАЛА создаем базовые экземпляры Echo и логгера
	e := echo.New()
	logger := applogger.NewLogger()

	// 2. Инициализируем конфиг
	cfg := config.New()

	// 3. ПОСЛЕ этого настраиваем middleware, так как они используют logger и echo
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		DisableStackAll: true,
		StackSize:       1 << 10,
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			logger.Error("!!! ОБНАРУЖЕНА ПАНИКА (PANIC) !!!",
				zap.String("method", c.Request().Method),
				zap.String("uri", c.Request().RequestURI),
				zap.Error(err),
				zap.String("stack", string(stack)),
			)
			if !c.Response().Committed {
				httpErr := apperrors.NewHttpError(http.StatusInternalServerError, "Внутренняя ошибка сервера", err, nil)
				utils.ErrorResponse(c, httpErr, logger)
			}
			return err
		},
	}))

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("ngrok-skip-browser-warning", "true")
			return next(c)
		}
	})

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: func(origin string) (bool, error) {
			allowedOrigins := []string{
				"https://d41fdadc8416.ngrok-free.app",
				"http://localhost:5173",
			}
			for _, o := range allowedOrigins {
				if origin == o {
					return true, nil
				}
			}
			return false, nil
		},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "ngrok-skip-browser-warning"},
		AllowCredentials: true,
		ExposeHeaders:    []string{"Content-Disposition"},
	}))

	// 4. Настраиваем статические файлы
	absPath, err := filepath.Abs("./uploads")
	if err != nil {
		logger.Fatal("не удалось получить абсолютный путь к uploads", zap.Error(err))
	}
	e.Static("/uploads", absPath)

	// <<<--- 5. ЭТО ПРАВИЛЬНОЕ МЕСТО ДЛЯ ИНИЦИАЛИЗАЦИИ ВАЛИДАТОРА ---
	// Он использует logger для вывода ошибок и `e` для присваивания.
	v := validator.New()
	// Вызываем нашу единую функцию из нового пакета
	if err := customvalidator.RegisterCustomValidations(v); err != nil {
		logger.Fatal("Ошибка регистрации кастомных правил валидации", zap.Error(err))
	}
	e.Validator = utils.NewValidator(v)
	// <<<--- КОНЕЦ БЛОКА ВАЛИДАТОРА ---

	// 6. Подключаемся к базам данных и другим сервисам
	dbConn := postgresql.ConnectDB()
	defer dbConn.Close()

	redisAddr := os.Getenv("REDIS_ADDRESS")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
		logger.Fatal("не удалось подключиться к Redis", zap.Error(err), zap.String("address", redisAddr))
	}

	// 7. Инициализируем сервисы
	jwtSecretKey := os.Getenv("JWT_SECRET_KEY")
	accessTokenTTL := time.Hour * 24
	refreshTokenTTL := time.Hour * 24 * 7 // Примечание: в вашем коде это значение может быть переопределено из .env в config.New()
	jwtSvc := service.NewJWTService(jwtSecretKey, accessTokenTTL, refreshTokenTTL, logger)

	permissionRepo := repositories.NewPermissionRepository(dbConn, logger)
	cacheRepo := repositories.NewRedisCacheRepository(redisClient)

	rolePermissionsCacheTTL := time.Minute * 10
	authPermissionService := services.NewAuthPermissionService(permissionRepo, cacheRepo, logger, rolePermissionsCacheTTL)

	// 8. Инициализируем роуты
	routes.InitRouter(e, dbConn, redisClient, jwtSvc, logger, authPermissionService, cfg)

	// 9. Запускаем сервер
	logger.Info("🚀 Сервер запущен на :8080")
	if err := e.Start(":8080"); err != nil {
		logger.Fatal("Ошибка запуска сервера", zap.Error(err))
	}
}

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"request-system/internal/routes"
	"request-system/pkg/database/postgresql"
	applogger "request-system/pkg/logger"

	"request-system/pkg/service"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"request-system/internal/repositories"
	"request-system/internal/services"
)

type CustomValidator struct {
	validator *validator.Validate
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		return err
	}
	return nil
}

func isTajikPhoneNumber(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(`^\+992\d{9}$`)
	return re.MatchString(fl.Field().String())
}

func isDurationValid(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(`^(\d+h)?(\d+m)?$`)
	s := fl.Field().String()
	return re.MatchString(s) && (strings.Contains(s, "h") || strings.Contains(s, "m"))
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found or could not be loaded.")
	}

	e := echo.New()
	logger := applogger.NewLogger()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("ngrok-skip-browser-warning", "true")
			return next(c)
		}
	})

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{
			"http://localhost:5173",
			"https://65006e9a1844.ngrok-free.app",
			"https://eec4e17c3a10.ngrok-free.app",
		},
		AllowMethods: []string{
			http.MethodGet, http.MethodPost, http.MethodPut,
			http.MethodDelete, http.MethodOptions,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization, // вот эта строка критична!
			"ngrok-skip-browser-warning",
		},
		AllowCredentials: true, // тоже критично
		ExposeHeaders: []string{
			"Content-Disposition",
		},
	}))

	absPath, err := filepath.Abs("./uploads")
	if err != nil {
		logger.Fatal("не удалось получить абсолютный путь к uploads", zap.Error(err))
	}
	logger.Info("Абсолютный путь к uploads", zap.String("path", absPath))

	e.Static("/uploads", absPath)

	e.GET("/testphoto", func(c echo.Context) error {
		return c.File("./uploads/2025/08/05/2025-08-05-80286516-eaff-472e-8379-22d3e51bb236.jpg")
	})
	v := validator.New()
	if err := v.RegisterValidation("e164_TJ", isTajikPhoneNumber); err != nil {
		logger.Fatal("Ошибка регистрации валидации e164_TJ", zap.Error(err))
	}
	if err := v.RegisterValidation("duration_format", isDurationValid); err != nil {
		logger.Fatal("Ошибка регистрации валидации duration_format", zap.Error(err))
	}

	e.Validator = &CustomValidator{validator: v}

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
	logger.Info("main: Успешное подключение к Redis")

	jwtSecretKey := os.Getenv("JWT_SECRET_KEY")
	if jwtSecretKey == "" {
		logger.Warn("main: JWT_SECRET_KEY не найден в .env. Используется небезопасный запасной ключ.")
		jwtSecretKey = "your_default_super_secret_key_for_testing"
	}
	if os.Getenv("ENV") == "production" && jwtSecretKey == "your_default_super_secret_key_for_testing" {
		logger.Fatal("В продакшене необходимо задать безопасный JWT_SECRET_KEY")
	}

	accessTokenTTL := time.Hour * 24
	refreshTokenTTL := time.Hour * 24 * 7
	jwtSvc := service.NewJWTService(jwtSecretKey, accessTokenTTL, refreshTokenTTL, logger)
	logger.Info("main: JWTService успешно создан")

	permissionRepo := repositories.NewPermissionRepository(dbConn, logger)

	cacheRepo := repositories.NewRedisCacheRepository(redisClient)

	rolePermissionsCacheTTL := time.Minute * 10
	authPermissionService := services.NewAuthPermissionService(permissionRepo, cacheRepo, logger, rolePermissionsCacheTTL)
	logger.Info("main: AuthPermissionService успешно создан")

	routes.InitRouter(e, dbConn, redisClient, jwtSvc, logger, authPermissionService)
	logger.Info("🚀 Сервер запущен на :8080")
	if err := e.Start(":8080"); err != nil {
		logger.Fatal("Ошибка запуска сервера", zap.Error(err))
	}
}

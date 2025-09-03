package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"request-system/internal/repositories"
	"request-system/internal/routes"
	"request-system/internal/services"
	"request-system/pkg/config"
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

func isGoodEmailFormat(fl validator.FieldLevel) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(fl.Field().String())
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

	cfg := config.New()

	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		DisableStackAll: true,
		StackSize:       1 << 10,
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			// Используем новую функцию с логером
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
			allowedOrigin := "https://d41fdadc8416.ngrok-free.app"
			if origin == allowedOrigin {
				return true, nil
			}
			return false, nil
		},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderAuthorization,
			"ngrok-skip-browser-warning",
		},
		AllowCredentials: true,
		ExposeHeaders:    []string{"Content-Disposition"},
	}))
	absPath, err := filepath.Abs("./uploads")
	if err != nil {
		logger.Fatal("не удалось получить абсолютный путь к uploads", zap.Error(err))
	}
	e.Static("/uploads", absPath)

	v := validator.New()
	v.RegisterValidation("e164_TJ", isTajikPhoneNumber)
	v.RegisterValidation("duration_format", isDurationValid)
	if err := v.RegisterValidation("email", isGoodEmailFormat); err != nil {
		logger.Fatal("Ошибка переопределения валидации 'email'", zap.Error(err))
	}
	e.Validator = utils.NewValidator(v)

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

	jwtSecretKey := os.Getenv("JWT_SECRET_KEY")

	accessTokenTTL := time.Hour * 24
	refreshTokenTTL := time.Hour * 24 * 7
	jwtSvc := service.NewJWTService(jwtSecretKey, accessTokenTTL, refreshTokenTTL, logger)

	permissionRepo := repositories.NewPermissionRepository(dbConn, logger)
	cacheRepo := repositories.NewRedisCacheRepository(redisClient)

	rolePermissionsCacheTTL := time.Minute * 10
	authPermissionService := services.NewAuthPermissionService(permissionRepo, cacheRepo, logger, rolePermissionsCacheTTL)

	routes.InitRouter(e, dbConn, redisClient, jwtSvc, logger, authPermissionService, cfg)

	logger.Info("🚀 Сервер запущен на :8080")
	if err := e.Start(":8080"); err != nil {
		logger.Fatal("Ошибка запуска сервера", zap.Error(err))
	}
}

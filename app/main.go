package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"request-system/internal/routes"
	"request-system/pkg/database/postgresql"
	applogger "request-system/pkg/logger"
	"request-system/pkg/service"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
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

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found or could not be loaded.")
	}

	e := echo.New()

	v := validator.New()
	v.RegisterValidation("e164_TJ", isTajikPhoneNumber)
	e.Validator = &CustomValidator{validator: v}

	logger := applogger.NewLogger()

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
	accessTokenTTL := time.Hour * 1
	refreshTokenTTL := time.Hour * 24 * 7
	jwtSvc := service.NewJWTService(jwtSecretKey, accessTokenTTL, refreshTokenTTL)
	logger.Info("main: JWTService успешно создан")

	routes.InitRouter(e, dbConn, redisClient, jwtSvc, logger)

	fmt.Println("\n\n==================== Зарегистрированные маршруты ====================")
	for _, r := range e.Routes() {
		fmt.Printf("МЕТОД: %-7s | ПУТЬ: %-50s | ОБРАБОТЧИК: %s\n", r.Method, r.Path, r.Name)
	}
	fmt.Print("===================================================================\n\n")

	logger.Info("🚀 Сервер запущен на :8080")
	if err := e.Start(":8080"); err != nil {
		logger.Fatal("Ошибка запуска сервера", zap.Error(err))
	}
}


package main

import (
	"log"
	"os"
	"request-system/internal/routes"
	"request-system/pkg/database/postgresql"
	applogger "request-system/pkg/logger"
	"request-system/pkg/service"
	"time"

	"github.com/go-playground/validator/v10"
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

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, переменные окружения не загружены из файла")
	}
	e := echo.New()
	e.Validator = &CustomValidator{validator: validator.New()}

	logger := applogger.NewLogger()

	dbConn := postgresql.ConnectDB()
	if dbConn == nil {
		logger.Fatal("main: Ошибка подключения к базе данных")
	}
	logger.Info("main: Успешное подключение к базе данных")

	jwtSecretKey := os.Getenv("JWT_SECRET_KEY")
	if jwtSecretKey == "" {
		logger.Warn("main: Переменная окружения JWT_SECRET_KEY не установлена! ИСПОЛЬЗУЕТСЯ ДЕФОЛТНЫЙ (НЕБЕЗОПАСНЫЙ) КЛЮЧ.")
		jwtSecretKey = "s8D3f9LqPwX2vM0zNzG7RkH1TcJb5YxVUaQWmEoZlIfSgCnBtKhDrLjPeUyOxFa"
	}

	if len(jwtSecretKey) < 64 && os.Getenv("APP_ENV") != "development" {

		logger.Info("main: Длина JWT_SECRET_KEY", zap.Int("length", len(jwtSecretKey)))

		logger.Warn("main: JWT_SECRET_KEY слишком короткий (рекомендуется минимум 64 символа для HS512), безопасность может быть низкой!")
	}

	accessTokenTTL := time.Hour * 1
	refreshTokenTTL := time.Hour * 24 * 7

	jwtSvc := service.NewJWTService(jwtSecretKey, accessTokenTTL, refreshTokenTTL)
	logger.Info("main: JWTService успешно создан")

	routes.INIT_ROUTER(e, dbConn, jwtSvc, logger)

	serverStartTime := time.Now()
	log.Printf("DEBUG: Server current time at start is %s in location %s", serverStartTime.Format(time.RFC3339), serverStartTime.Location())
	logger.Info("🚀 Сервер запущен на :8080")

	if err := e.Start(":8080"); err != nil {
		logger.Fatal("Ошибка запуска сервера", zap.Error(err))
	}
}

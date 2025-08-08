package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool" // Убедись, что эти импорты используются
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runUserRouter(
	secureGroup *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
	authPermissionService services.AuthPermissionServiceInterface,
	fileStorage filestorage.FileStorageInterface,
) {
	userRepository := repositories.NewUserRepository(dbConn, logger)
	statusRepository := repositories.NewStatusRepository(dbConn)

	userService := services.NewUserService(userRepository, statusRepository, logger)

	userCtrl := controllers.NewUserController(userService, fileStorage, logger)

	secureGroup.POST("/user", userCtrl.CreateUser, authMW.AuthorizeAny("users:create")) // Базовое право на создание

	secureGroup.GET("/user", userCtrl.GetUsers, authMW.AuthorizeAny("users:view"))     // Базовое право на просмотр списка
	secureGroup.GET("/user/:id", userCtrl.FindUser, authMW.AuthorizeAny("users:view")) // Базовое право на просмотр одного

	secureGroup.PUT("/user/:id", userCtrl.UpdateUser, authMW.AuthorizeAny("users:update", "profile:update")) // Базовое право на обновление + право на обновление своего профиля
	secureGroup.DELETE("/user/:id", userCtrl.DeleteUser, authMW.AuthorizeAny("users:delete"))                // Базовое право на удаление
}

package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/middleware" // <-- ДОБАВЛЕНО

	// request-system/pkg/logger" // Если здесь используется logger.NewLogger, это неправильно, logger должен быть передан

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4" // <-- Добавлен импорт echo
	"go.uber.org/zap"             // <-- Добавлен импорт zap
)

// runUserRouter теперь принимает AuthMiddleware и AuthPermissionServiceInterface
func runUserRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware, authPermissionService services.AuthPermissionServiceInterface) { // <-- ИЗМЕНЕНО
	userRepository := repositories.NewUserRepository(dbConn)
	statusRepository := repositories.NewStatusRepository(dbConn)             // <-- Возможно, NewStatusRepository тоже нуждается в dbConn
	userService := services.NewUserService(userRepository, statusRepository) // Если UserService изменится, он также может нуждаться в AuthPermissionService.

	userCtrl := controllers.NewUserController(userService, logger) // Если NewUserController также изменялся, обновите здесь

	secureGroup.GET("/user", userCtrl.GetUsers, authMW.AuthorizeAny("users:view:all", "users:view:department"))                         // Просмотр всех/своего департамента
	secureGroup.GET("/user/:id", userCtrl.FindUser, authMW.AuthorizeAny("users:view:all", "users:view:department", "profile:view:own")) // FindUser: могут ли все видеть кого-либо, или только своего по ID.
	secureGroup.POST("/user", userCtrl.CreateUser, authMW.AuthorizeAny("users:create", "users:manage"))                                 // Создание пользователей
	secureGroup.PUT("/user/:id", userCtrl.UpdateUser, authMW.AuthorizeAny("users:update", "users:manage", "profile:update:own"))        // Обновление пользователей
	secureGroup.DELETE("/user/:id", userCtrl.DeleteUser, authMW.AuthorizeAny("users:delete", "users:manage"))                           // Удаление пользователей
}

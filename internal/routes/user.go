package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runUserRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware, authPermissionService services.AuthPermissionServiceInterface, fileStorage filestorage.FileStorageInterface) {
	userRepository := repositories.NewUserRepository(dbConn)
	statusRepository := repositories.NewStatusRepository(dbConn)
	userService := services.NewUserService(userRepository, statusRepository)
	userCtrl := controllers.NewUserController(userService, fileStorage, logger)

	secureGroup.POST("/user", userCtrl.CreateUser, authMW.AuthorizeAny("users:create", "users:manage"))

	secureGroup.GET("/user", userCtrl.GetUsers, authMW.AuthorizeAny("users:view:all", "users:view:department"))
	secureGroup.GET("/user/:id", userCtrl.FindUser, authMW.AuthorizeAny("users:view:all", "users:view:department", "profile:view:own"))
	secureGroup.POST("/user", userCtrl.CreateUser, authMW.AuthorizeAny("users:create", "users:manage"))
	secureGroup.PUT("/user/:id", userCtrl.UpdateUser, authMW.AuthorizeAny("users:update", "users:manage", "profile:update:own"))
	secureGroup.DELETE("/user/:id", userCtrl.DeleteUser, authMW.AuthorizeAny("users:delete", "users:manage"))
}

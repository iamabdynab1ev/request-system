package routes

import (
	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/filestorage"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runUserRouter(
	secureGroup *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
	fileStorage filestorage.FileStorageInterface,
) {
	userRepository := repositories.NewUserRepository(dbConn, logger)
	statusRepository := repositories.NewStatusRepository(dbConn)
	userService := services.NewUserService(userRepository, statusRepository, logger)
	userCtrl := controllers.NewUserController(userService, fileStorage, logger)

	users := secureGroup.Group("/user")

	users.POST("", userCtrl.CreateUser, authMW.AuthorizeAny(authz.UsersCreate))
	users.GET("", userCtrl.GetUsers, authMW.AuthorizeAny(authz.UsersView))
	users.GET("/:id", userCtrl.FindUser, authMW.AuthorizeAny(authz.UsersView))
	users.PUT("/:id", userCtrl.UpdateUser, authMW.AuthorizeAny(authz.UsersUpdate, authz.ProfileUpdate))
	users.DELETE("/:id", userCtrl.DeleteUser, authMW.AuthorizeAny(authz.UsersDelete))
}

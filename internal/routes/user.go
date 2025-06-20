package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func RUN_USER_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		logger           *zap.Logger
		userRepository   = repositories.NewUserRepository(dbConn)
		statusRepository = repositories.NewStatusRepository(dbConn)

		userService = services.NewUserService(userRepository, statusRepository)
		userCtrl    = controllers.NewUserController(userService, logger)
	)
	e.GET("/users", userCtrl.GetUsers)
	e.GET("/user/:id", userCtrl.FindUser)
	e.POST("/user", userCtrl.CreateUser)
	e.PUT("/user/:id", userCtrl.UpdateUser)
	e.DELETE("/user/:id", userCtrl.DeleteUser)
}

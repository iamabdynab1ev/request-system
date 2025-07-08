package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runUserRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	userRepository := repositories.NewUserRepository(dbConn)
	statusRepository := repositories.NewStatusRepository(dbConn)
	userService := services.NewUserService(userRepository, statusRepository)
	userCtrl := controllers.NewUserController(userService, logger)

	secureGroup.GET("/users", userCtrl.GetUsers)
	secureGroup.GET("/user/:id", userCtrl.FindUser)
	secureGroup.POST("/user", userCtrl.CreateUser)
	secureGroup.PUT("/user/:id", userCtrl.UpdateUser)
	secureGroup.DELETE("/user/:id", userCtrl.DeleteUser)
}

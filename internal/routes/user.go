package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// ИСПРАВЛЕНО: Добавлен logger в аргументы
func RUN_USER_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool, logger *zap.Logger) {
	var (
		// logger здесь больше не объявляется
		userRepository   = repositories.NewUserRepository(dbConn)
		statusRepository = repositories.NewStatusRepository(dbConn)

		userService = services.NewUserService(userRepository, statusRepository)
		// ИСПРАВЛЕНО: Передаем logger в контроллер
		userCtrl = controllers.NewUserController(userService, logger)
	)

	// Группируем роуты для пользователей
	userGroup := e.Group("/api")
	// Здесь можно добавить middleware для проверки авторизации

	userGroup.GET("/users", userCtrl.GetUsers)
	userGroup.GET("/user/:id", userCtrl.FindUser)
	userGroup.POST("/user", userCtrl.CreateUser)
	userGroup.PUT("/user/:id", userCtrl.UpdateUser)
	userGroup.DELETE("/user/:id", userCtrl.DeleteUser)
}

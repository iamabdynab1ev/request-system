package routes

import (
	"request-system/internal/authz" // <-- Импорт констант
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/middleware" // <-- Импорт мидлвара

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Название функции лучше сделать с маленькой буквы, если она не экспортируется
// Я оставил как у тебя - RunPriorityRouter, но правильно было бы runPriorityRouter
func RunPriorityRouter(
	secureGroup *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	priorityRepository := repositories.NewPriorityRepository(dbConn)

	userRepository := repositories.NewUserRepository(dbConn, logger)

	priorityService := services.NewPriorityService(priorityRepository, userRepository, logger)

	priorityCtrl := controllers.NewPriorityController(priorityService, logger)

	priorities := secureGroup.Group("/priority")

	// Просмотр доступен тем, у кого есть 'catalogs:view'
	priorities.GET("", priorityCtrl.GetPriorities, authMW.AuthorizeAny(authz.PrioritiesView))
	priorities.GET("/:id", priorityCtrl.FindPriority, authMW.AuthorizeAny(authz.PrioritiesView))

	// Управление доступно тем, у кого есть права на управление приоритет
	priorities.POST("", priorityCtrl.CreatePriority, authMW.AuthorizeAny(authz.PrioritiesCreate))
	priorities.PUT("/:id", priorityCtrl.UpdatePriority, authMW.AuthorizeAny(authz.PrioritiesUpdate))
	priorities.DELETE("/:id", priorityCtrl.DeletePriority, authMW.AuthorizeAny(authz.PrioritiesDelete))
}

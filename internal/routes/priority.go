// Файл: internal/routes/priority_router.go
package routes

import (
	"log"

	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func RunPriorityRouter(
	secureGroup *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	priorityRepository := repositories.NewPriorityRepository(dbConn, logger)
	userRepository := repositories.NewUserRepository(dbConn, logger) // Предполагаем, что он уже есть

	// Внедряем FileStorage в сервис
	priorityService := services.NewPriorityService(priorityRepository, userRepository, logger)
	if priorityService == nil {
		log.Fatal("Не удалось создать PriorityService")
	}
	priorityCtrl := controllers.NewPriorityController(priorityService, logger)

	priorities := secureGroup.Group("/priority")
	priorities.GET("", priorityCtrl.GetPriorities, authMW.AuthorizeAny(authz.PrioritiesView))
	priorities.GET("/:id", priorityCtrl.FindPriority, authMW.AuthorizeAny(authz.PrioritiesView))
	priorities.POST("", priorityCtrl.CreatePriority, authMW.AuthorizeAny(authz.PrioritiesCreate))
	priorities.PUT("/:id", priorityCtrl.UpdatePriority, authMW.AuthorizeAny(authz.PrioritiesUpdate))
	priorities.DELETE("/:id", priorityCtrl.DeletePriority, authMW.AuthorizeAny(authz.PrioritiesDelete))
}

// routes/permission_router.go
package routes

/*import (
	"request-system/internal/controllers"
	"request-system/internal/middleware"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

var authMW *middleware.AuthMiddleware

func runPermissionRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, userRepo repositories.UserRepositoryInterface) {
	// Инициализация репозитория и сервиса для привилегий
	permRepo := repositories.NewPermissionRepository(dbConn, logger)

	// ИСПРАВЛЕНИЕ: Используем интерфейсный тип для сервиса
	permService := services.NewPermissionService(permRepo, repositories.UserRepository) // <-- НОВЫЙ КОНСТРУКТОР

	// Создаем контроллер с интерфейсным типом сервиса
	permCtrl := controllers.NewPermissionController(permService, logger)

	perms := secureGroup.Group("/permissions")

	// Просмотр доступен всем, у кого есть 'permissions:view'
	perms.GET("", permCtrl.GetPermissions, authMW.AuthorizeAny("permissions:view"))
	perms.GET("/:id", permCtrl.FindPermission, authMW.AuthorizeAny("permissions:view"))

	// Управление - только тем, у кого есть конкретные права (т.е. Super Admin)
	perms.POST("", permCtrl.CreatePermission, authMW.AuthorizeAny("permissions:create"))
	perms.PUT("/:id", permCtrl.UpdatePermission, authMW.AuthorizeAny("permissions:update"))
	perms.DELETE("/:id", permCtrl.DeletePermission, authMW.AuthorizeAny("permissions:delete"))
}
*/

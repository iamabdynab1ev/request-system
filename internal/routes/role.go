package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runRoleRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware, authPermissionService services.AuthPermissionServiceInterface) { // <-- ИЗМЕНЕНО
	roleRepository := repositories.NewRoleRepository(dbConn)
	roleService := services.NewRoleService(roleRepository, authPermissionService, logger)
	roleCtrl := controllers.NewRoleController(roleService, logger)

	// Применяем AuthMiddleware.Authorize к маршрутам
	secureGroup.GET("/role", roleCtrl.GetRoles, authMW.AuthorizeAny("roles:view:all", "roles:manage"))
	secureGroup.POST("/role", roleCtrl.CreateRole, authMW.AuthorizeAny("roles:create", "roles:manage"))
	secureGroup.GET("/role/:id", roleCtrl.FindRole, authMW.AuthorizeAny("roles:view:all", "roles:manage")) // Или "roles:view:own"
	secureGroup.PUT("/role/:id", roleCtrl.UpdateRole, authMW.AuthorizeAny("roles:update", "roles:manage"))
	secureGroup.DELETE("/role/:id", roleCtrl.DeleteRole, authMW.AuthorizeAny("roles:delete", "roles:manage"))
}

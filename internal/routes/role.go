package routes

import (
	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runRoleRouter(
	secureGroup *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
	authPermissionService services.AuthPermissionServiceInterface,
) {
	roleRepository := repositories.NewRoleRepository(dbConn, logger)
	userRepository := repositories.NewUserRepository(dbConn, logger)

	roleService := services.NewRoleService(roleRepository, userRepository, authPermissionService, logger)
	roleCtrl := controllers.NewRoleController(roleService, logger)

	roles := secureGroup.Group("/role")

	roles.GET("", roleCtrl.GetRoles, authMW.AuthorizeAny(authz.RolesView))
	roles.POST("", roleCtrl.CreateRole, authMW.AuthorizeAny(authz.RolesCreate))
	roles.GET("/:id", roleCtrl.FindRole, authMW.AuthorizeAny(authz.RolesView))
	roles.PUT("/:id", roleCtrl.UpdateRole, authMW.AuthorizeAny(authz.RolesUpdate))
	roles.DELETE("/:id", roleCtrl.DeleteRole, authMW.AuthorizeAny(authz.RolesDelete))
}

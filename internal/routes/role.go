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

func runRoleRouter(
	secureGroup *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
	authPermissionService services.AuthPermissionServiceInterface,
) {
	roleRepository := repositories.NewRoleRepository(dbConn)
	userRepository := repositories.NewUserRepository(dbConn, logger)

	roleService := services.NewRoleService(
		roleRepository,
		userRepository,
		authPermissionService,
		logger,
	)

	roleCtrl := controllers.NewRoleController(roleService, logger)

	roles := secureGroup.Group("/role")

	roles.GET("", roleCtrl.GetRoles, authMW.AuthorizeAny("roles:view"))
	roles.POST("", roleCtrl.CreateRole, authMW.AuthorizeAny("roles:create"))
	roles.GET("/:id", roleCtrl.FindRole, authMW.AuthorizeAny("roles:view"))
	roles.PUT("/:id", roleCtrl.UpdateRole, authMW.AuthorizeAny("roles:update"))
	roles.DELETE("/:id", roleCtrl.DeleteRole, authMW.AuthorizeAny("roles:delete"))
}

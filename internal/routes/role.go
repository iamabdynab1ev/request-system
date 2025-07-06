package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func RUN_ROLE_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool, logger *zap.Logger) {
	roleRepository := repositories.NewRoleRepository(dbConn)
	roleService := services.NewRoleService(roleRepository, logger)
	roleCtrl := controllers.NewRoleController(roleService, logger)

	rolesGroup := e.Group("/api/roles")
	{
		rolesGroup.GET("", roleCtrl.GetRoles)
		rolesGroup.POST("", roleCtrl.CreateRole)
		rolesGroup.GET("/:id", roleCtrl.FindRole)
		rolesGroup.PUT("/:id", roleCtrl.UpdateRole)
		rolesGroup.DELETE("/:id", roleCtrl.DeleteRole)
	}
}

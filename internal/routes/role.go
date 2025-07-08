package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runRoleRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	roleRepository := repositories.NewRoleRepository(dbConn)
	roleService := services.NewRoleService(roleRepository, logger)
	roleCtrl := controllers.NewRoleController(roleService, logger)

	secureGroup.GET("/roles", roleCtrl.GetRoles)
	secureGroup.POST("/role", roleCtrl.CreateRole)
	secureGroup.GET("/role/:id", roleCtrl.FindRole)
	secureGroup.PUT("/role/:id", roleCtrl.UpdateRole)
	secureGroup.DELETE("/role/:id", roleCtrl.DeleteRole)
}

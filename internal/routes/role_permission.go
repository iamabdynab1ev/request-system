package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services" // <-- ДОБАВЛЕНО для AuthPermissionServiceInterface
	"request-system/pkg/middleware"    // <-- ДОБАВЛЕНО для AuthMiddleware

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap" // Убедитесь, что zap импортирован
)

// runRolePermissionRouter теперь принимает secureGroup, logger, authMW и authPermissionService
func runRolePermissionRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware, authPermissionService services.AuthPermissionServiceInterface) { // <-- ИЗМЕНЕНО
	var (
		rpRepository = repositories.NewRolePermissionRepository(dbConn)
		// ИСПРАВЛЕНО: Передаем authPermissionService в конструктор RolePermissionService
		rpService = services.NewRolePermissionService(rpRepository, authPermissionService, logger)
		rpCtrl    = controllers.NewRolePermissionController(rpService, logger)
	)

	// Используем secureGroup для маршрутов
	// Добавляем middleware авторизации AuthorizeAny/All
	secureGroup.GET("/role_permission", rpCtrl.GetRolePermissions, authMW.AuthorizeAny("role_permissions:view:all", "role_permissions:manage"))
	secureGroup.GET("/role_permission/:id", rpCtrl.FindRolePermission, authMW.AuthorizeAny("role_permissions:view:all", "role_permissions:manage"))
	secureGroup.POST("/role_permission", rpCtrl.CreateRolePermission, authMW.AuthorizeAny("role_permissions:create", "role_permissions:manage"))
	secureGroup.PUT("/role_permission/:id", rpCtrl.UpdateRolePermission, authMW.AuthorizeAny("role_permissions:update", "role_permissions:manage"))
	secureGroup.DELETE("/role_permission/:id", rpCtrl.DeleteRolePermission, authMW.AuthorizeAny("role_permissions:delete", "role_permissions:manage"))
}

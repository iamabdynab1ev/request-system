// Файл: internal/routes/department_router.go
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

func runDepartmentRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware, txManager repositories.TxManagerInterface) { // <-- ДОБАВЛЕН txManager
	departmentRepo := repositories.NewDepartmentRepository(dbConn, logger)
	userRepo := repositories.NewUserRepository(dbConn, logger)

	// ИСПРАВЛЕНИЕ: Передаем txManager в конструктор
	departmentService := services.NewDepartmentService(txManager, departmentRepo, userRepo, logger)
	departmentCtrl := controllers.NewDepartmentController(departmentService, logger)

	// Роуты (без изменений)
	secureGroup.GET("/main", departmentCtrl.GetDepartmentStats, authMW.AuthorizeAny(authz.DepartmentsView))
	departmentsGroup := secureGroup.Group("/department")
	departmentsGroup.GET("", departmentCtrl.GetDepartments, authMW.AuthorizeAny(authz.DepartmentsView))
	departmentsGroup.GET("/:id", departmentCtrl.FindDepartment, authMW.AuthorizeAny(authz.DepartmentsView))
	departmentsGroup.POST("", departmentCtrl.CreateDepartment, authMW.AuthorizeAny(authz.DepartmentsCreate))
	departmentsGroup.PUT("/:id", departmentCtrl.UpdateDepartment, authMW.AuthorizeAny(authz.DepartmentsUpdate))
	departmentsGroup.DELETE("/:id", departmentCtrl.DeleteDepartment, authMW.AuthorizeAny(authz.DepartmentsDelete))
}

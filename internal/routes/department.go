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

func runDepartmentRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware) {

	departmentRepo := repositories.NewDepartmentRepository(dbConn)
	userRepo := repositories.NewUserRepository(dbConn, logger)

	departmentService := services.NewDepartmentService(departmentRepo, userRepo, logger)
	departmentCtrl := controllers.NewDepartmentController(departmentService, logger)

	secureGroup.GET("/main", departmentCtrl.GetDepartmentStats, authMW.AuthorizeAny(authz.DepartmentsView))

	departmentsGroup := secureGroup.Group("/department")

	departmentsGroup.GET("", departmentCtrl.GetDepartments, authMW.AuthorizeAny(authz.DepartmentsView))
	departmentsGroup.GET("/:id", departmentCtrl.FindDepartment, authMW.AuthorizeAny(authz.DepartmentsView))
	departmentsGroup.POST("", departmentCtrl.CreateDepartment, authMW.AuthorizeAny(authz.DepartmentsCreate))
	departmentsGroup.PUT("/:id", departmentCtrl.UpdateDepartment, authMW.AuthorizeAny(authz.DepartmentsUpdate))
	departmentsGroup.DELETE("/:id", departmentCtrl.DeleteDepartment, authMW.AuthorizeAny(authz.DepartmentsDelete))
}

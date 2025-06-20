package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_DEPARTMENT_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		logger               = logger.NewLogger()
		departmentRepository = repositories.NewDepartmentRepository(dbConn)
		departmentService    = services.NewDepartmentService(departmentRepository, logger)
		departmentCtrl       = controllers.NewDepartmentController(departmentService, logger)
	)
	e.GET("/departments", departmentCtrl.GetDepartments)
	e.GET("/department/:id", departmentCtrl.FindDepartment)
	e.POST("/department", departmentCtrl.CreateDepartment)
	e.PUT("/department/:id", departmentCtrl.UpdateDepartment)
	e.DELETE("/department/:id", departmentCtrl.DeleteDepartment)
}

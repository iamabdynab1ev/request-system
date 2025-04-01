package routes

import (
	"request-system/internal/controllers"
	"github.com/labstack/echo/v4"
)

var departmentCtrl = controllers.NewDepartmentController()

func RUN_DEPARTMENT_ROUTER(e *echo.Echo) {
	e.GET("department", departmentCtrl.GetDepartments)
	e.GET("department/:id", departmentCtrl.FindDepartments)
	e.POST("department", departmentCtrl.CreateDepartments)
	e.PUT("department/:id", departmentCtrl.UpdateDepartments)
	e.DELETE("department/:id", departmentCtrl.DeleteDepartments)
}
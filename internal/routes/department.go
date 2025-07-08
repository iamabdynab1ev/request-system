// Package routes содержит настройку маршрутов HTTP для различных сущностей приложения.
// Здесь определяются группы маршрутов, связываются контроллеры с HTTP-эндпоинтами и инициализи
package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runDepartmentRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	departmentRepository := repositories.NewDepartmentRepository(dbConn)
	departmentService := services.NewDepartmentService(departmentRepository, logger)
	departmentCtrl := controllers.NewDepartmentController(departmentService, logger)

	secureGroup.GET("/departments", departmentCtrl.GetDepartments)
	secureGroup.GET("/department/:id", departmentCtrl.FindDepartment)
	secureGroup.POST("/department", departmentCtrl.CreateDepartment)
	secureGroup.PUT("/department/:id", departmentCtrl.UpdateDepartment)
	secureGroup.DELETE("/department/:id", departmentCtrl.DeleteDepartment)
}

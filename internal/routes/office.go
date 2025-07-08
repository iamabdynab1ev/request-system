package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runOfficeRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	officeRepository := repositories.NewOfficeRepository(dbConn)
	officeService := services.NewOfficeService(officeRepository, logger)
	officeCtrl := controllers.NewOfficeController(officeService, logger)

	secureGroup.GET("/offices", officeCtrl.GetOffices)
	secureGroup.GET("/office/:id", officeCtrl.FindOffice)
	secureGroup.POST("/office", officeCtrl.CreateOffice)
	secureGroup.PUT("/office/:id", officeCtrl.UpdateOffice)
	secureGroup.DELETE("/office/:id", officeCtrl.DeleteOffice)
}

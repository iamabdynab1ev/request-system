package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_OFFICE_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	
	var (
		logger = logger.NewLogger()

		officeRepository = repositories.NewOfficeRepository(dbConn)
		officeService    = services.NewOfficeService(officeRepository, logger)
		officeCtrl       = controllers.NewOfficeController(officeService, logger)
	)
	e.GET("/office", officeCtrl.GetOffices)
	e.GET("/office/:id", officeCtrl.FindOffice)
	e.POST("/office", officeCtrl.CreateOffice)
	e.PUT("/office/:id", officeCtrl.UpdateOffice)
	e.DELETE("/office/:id", officeCtrl.DeleteOffice)
}
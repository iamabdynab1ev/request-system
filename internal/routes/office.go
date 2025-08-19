// Файл: internal/routes/office.go
// СКОПИРУЙТЕ И ПОЛНОСТЬЮ ЗАМЕНИТЕ СОДЕРЖИМОЕ

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

func runOfficeRouter(
	secureGroup *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	officeRepository := repositories.NewOfficeRepository(dbConn)
	officeService := services.NewOfficeService(officeRepository, logger)
	officeCtrl := controllers.NewOfficeController(officeService, logger)

	offices := secureGroup.Group("/office")

	offices.GET("", officeCtrl.GetOffices, authMW.AuthorizeAny(authz.OfficesView))
	offices.GET("/:id", officeCtrl.FindOffice, authMW.AuthorizeAny(authz.OfficesView))
	offices.POST("", officeCtrl.CreateOffice, authMW.AuthorizeAny(authz.OfficesCreate))
	offices.PUT("/:id", officeCtrl.UpdateOffice, authMW.AuthorizeAny(authz.OfficesUpdate))
	offices.DELETE("/:id", officeCtrl.DeleteOffice, authMW.AuthorizeAny(authz.OfficesDelete))
}

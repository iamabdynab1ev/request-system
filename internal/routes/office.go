package routes

import (
	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/services" // Убрали 'repositories' - он больше не нужен
	"request-system/pkg/middleware"

	// "github.com/jackc/pgx/v5/pgxpool" // Убрали - больше не нужен
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runOfficeRouter(
	secureGroup *echo.Group,

	officeService services.OfficeServiceInterface,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	officeCtrl := controllers.NewOfficeController(officeService, logger)

	offices := secureGroup.Group("/office")

	offices.GET("", officeCtrl.GetOffices, authMW.AuthorizeAny(authz.OfficesView))
	offices.GET("/:id", officeCtrl.FindOffice, authMW.AuthorizeAny(authz.OfficesView))
	offices.POST("", officeCtrl.CreateOffice, authMW.AuthorizeAny(authz.OfficesCreate))
	offices.PUT("/:id", officeCtrl.UpdateOffice, authMW.AuthorizeAny(authz.OfficesUpdate))
	offices.DELETE("/:id", officeCtrl.DeleteOffice, authMW.AuthorizeAny(authz.OfficesDelete))
}

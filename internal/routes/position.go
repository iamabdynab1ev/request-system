// Файл: internal/routes/position.go
package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/services"
	"request-system/pkg/middleware"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)


func runPositionRouter(
	secureGroup *echo.Group,
	positionService services.PositionServiceInterface,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {

	posCtrl := controllers.NewPositionController(positionService, logger)

	positions := secureGroup.Group("/position")
	{
		positions.POST("", posCtrl.Create, authMW.AuthorizeAny("position:create"))
		positions.GET("", posCtrl.GetAll, authMW.AuthorizeAny("position:view"))
		positions.GET("/:id", posCtrl.GetByID, authMW.AuthorizeAny("position:view"))
		positions.PUT("/:id", posCtrl.Update, authMW.AuthorizeAny("position:update"))
		positions.DELETE("/:id", posCtrl.Delete, authMW.AuthorizeAny("position:delete"))

		positions.GET("/types", posCtrl.GetTypes, authMW.AuthorizeAny("position:view"))
	}
}

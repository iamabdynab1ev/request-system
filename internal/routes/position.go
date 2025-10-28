// Файл: internal/routes/position.go
package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/services"
	"request-system/pkg/middleware"

	// "request-system/internal/authz"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// ---> ГЛАВНОЕ ИЗМЕНЕНИЕ: Принимаем SERVICE, а не dbConn <---
func runPositionRouter(
	secureGroup *echo.Group,
	positionService services.PositionServiceInterface,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	// Сервис уже готов, просто создаем контроллер
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

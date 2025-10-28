package routes

import (
	"github.com/labstack/echo/v4"

	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/pkg/middleware"
)

func runOrderHistoryRouter(
	secureGroup *echo.Group,
	historyController *controllers.OrderHistoryController,
	authMW *middleware.AuthMiddleware,
) {
	binder := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return next(c)
		}
	}

	secureGroup.GET("/order/:orderID/history", historyController.GetHistoryForOrder,
		binder,
		authMW.AuthorizeAny(authz.OrdersView))
}

// Файл: internal/routes/order_routing_rule.go
package routes

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/controllers"
	"request-system/internal/services"
	"request-system/pkg/middleware"
)

func runOrderRoutingRuleRouter(
	secureGroup *echo.Group,
	orderRuleService services.OrderRoutingRuleServiceInterface,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	ruleCtrl := controllers.NewOrderRoutingRuleController(orderRuleService, logger)

	rules := secureGroup.Group("/order_rule")
	{
		rules.POST("", ruleCtrl.Create, authMW.AuthorizeAny("order_rule:create"))
		rules.GET("", ruleCtrl.GetAll, authMW.AuthorizeAny("order_rule:view"))
		rules.GET("/:id", ruleCtrl.GetByID, authMW.AuthorizeAny("order_rule:view"))
		rules.PUT("/:id", ruleCtrl.Update, authMW.AuthorizeAny("order_rule:update"))
		rules.DELETE("/:id", ruleCtrl.Delete, authMW.AuthorizeAny("order_rule:delete"))
	}
}

package routes

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/services"
	"request-system/pkg/middleware"
)

func runReportRouter(
	secureGroup *echo.Group,
	reportService services.ReportServiceInterface,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	reportController := controllers.NewReportController(reportService, logger)

	secureGroup.GET("/report", reportController.GetReport, authMW.AuthorizeAny(authz.ReportView))
}

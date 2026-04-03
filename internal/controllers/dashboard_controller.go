package controllers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"
)

type DashboardController struct {
	dashboardService *services.DashboardService
	logger           *zap.Logger
}

func NewDashboardController(ds *services.DashboardService, logger *zap.Logger) *DashboardController {
	return &DashboardController{
		dashboardService: ds,
		logger:           logger,
	}
}

func (ctrl *DashboardController) GetDashboardStats(c echo.Context) error {
	filter := dto.DashboardFilterDTO{}
	filter.Period = strings.TrimSpace(c.QueryParam("period"))
	filter.Granularity = strings.TrimSpace(c.QueryParam("granularity"))

	if widgets := strings.TrimSpace(c.QueryParam("widgets")); widgets != "" {
		for _, widget := range strings.Split(widgets, ",") {
			widget = strings.TrimSpace(widget)
			if widget != "" {
				filter.Widgets = append(filter.Widgets, widget)
			}
		}
	}

	if dateFrom, ok := parseDashboardDate(c.QueryParam("dateFrom")); ok {
		filter.DateFrom = dateFrom
	} else if dateFrom, ok := parseDashboardDate(c.QueryParam("date_from")); ok {
		filter.DateFrom = dateFrom
	}

	if dateTo, ok := parseDashboardDate(c.QueryParam("dateTo")); ok {
		filter.DateTo = dateTo
	} else if dateTo, ok := parseDashboardDate(c.QueryParam("date_to")); ok {
		filter.DateTo = dateTo
	}

	stats, err := ctrl.dashboardService.GetDashboardStats(c.Request().Context(), filter)
	if err != nil {
		return utils.ErrorResponse(c, err, ctrl.logger)
	}

	return utils.SuccessResponse(c, stats, "Статистика для дашборда получена", http.StatusOK)
}

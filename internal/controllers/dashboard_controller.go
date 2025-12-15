package controllers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/services"
	"request-system/pkg/types"
	"request-system/pkg/utils"
)

// DashboardController использует НОВЫЙ сервис
type DashboardController struct {
	dashboardService *services.DashboardService // Изменился тип сервиса
	logger           *zap.Logger
}

func NewDashboardController(ds *services.DashboardService, logger *zap.Logger) *DashboardController {
	return &DashboardController{
		dashboardService: ds,
		logger:           logger,
	}
}

func (ctrl *DashboardController) GetDashboardStats(c echo.Context) error {
	var filter types.Filter
	// Простая логика фильтрации дат, если прилетит с фронта
	if dateFromStr := c.QueryParam("dateFrom"); dateFromStr != "" {
		if t, err := time.Parse(time.RFC3339, dateFromStr); err == nil {
			filter.DateFrom = &t
		}
	}
	if dateToStr := c.QueryParam("dateTo"); dateToStr != "" {
		if t, err := time.Parse(time.RFC3339, dateToStr); err == nil {
			filter.DateTo = &t
		}
	}

	
	stats, err := ctrl.dashboardService.GetDashboardStats(c.Request().Context(), filter)
	if err != nil {
		return utils.ErrorResponse(c, err, ctrl.logger)
	}
	return utils.SuccessResponse(c, stats, "Статистика для дашборда получена (DashboardService)", http.StatusOK)
}

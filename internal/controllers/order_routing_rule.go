// Файл: internal/controllers/order_routing_rule.go
package controllers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
)

type OrderRoutingRuleController struct {
	service services.OrderRoutingRuleServiceInterface
	logger  *zap.Logger
}

func NewOrderRoutingRuleController(service services.OrderRoutingRuleServiceInterface, logger *zap.Logger) *OrderRoutingRuleController {
	return &OrderRoutingRuleController{service: service, logger: logger}
}

func (c *OrderRoutingRuleController) Create(ctx echo.Context) error {
	var d dto.CreateOrderRoutingRuleDTO
	if err := ctx.Bind(&d); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверные данные", err, nil), c.logger)
	}
	if err := ctx.Validate(&d); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	result, err := c.service.Create(ctx.Request().Context(), d)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, result, "Правило создано", http.StatusCreated)
}

func (c *OrderRoutingRuleController) Update(ctx echo.Context) error {
	id, _ := strconv.Atoi(ctx.Param("id"))
	var d dto.UpdateOrderRoutingRuleDTO
	if err := ctx.Bind(&d); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверные данные", err, nil), c.logger)
	}
	if err := ctx.Validate(&d); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	result, err := c.service.Update(ctx.Request().Context(), id, d)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, result, "Правило обновлено", http.StatusOK)
}

func (c *OrderRoutingRuleController) Delete(ctx echo.Context) error {
	id, _ := strconv.Atoi(ctx.Param("id"))
	if err := c.service.Delete(ctx.Request().Context(), id); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Правило удалено", http.StatusOK)
}

func (c *OrderRoutingRuleController) GetByID(ctx echo.Context) error {
	id, _ := strconv.Atoi(ctx.Param("id"))
	result, err := c.service.GetByID(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, result, "Правило найдено", http.StatusOK)
}

func (c *OrderRoutingRuleController) GetAll(ctx echo.Context) error {
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	result, err := c.service.GetAll(ctx.Request().Context(), uint64(filter.Limit), uint64(filter.Offset), filter.Search)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, result.List, "Список правил получен", http.StatusOK, result.Pagination.TotalCount)
}

package controllers

import (
	"context"
	"net/http"

	"request-system/internal/services"
	"request-system/pkg/utils"
	"github.com/labstack/echo/v4"
)

type StatusController struct {
	statusService *services.StatusService
}

func NewStatusController(statusService *services.StatusService) *StatusController {
	return &StatusController{
		statusService: statusService,
	}
}

func (c *StatusController) GetStatuses(ctx echo.Context) error {
	contx := context.WithoutCancel(ctx.Request().Context())
	res, err := c.statusService.GetAll(contx)

	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	return ctx.JSON(http.StatusOK, utils.SuccessResponse(res))
}

func (c *StatusController) FindStatus(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success"))
}

func (c *StatusController) CreateStatus(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success"))
}

func (c *StatusController) UpdateStatus(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success"))
}

func (c *StatusController) DeleteStatus(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success"))
}

package controllers

import (
	"net/http"

	"request-system/pkg/utils"
	"github.com/labstack/echo/v4"
)

type OrderController struct {}

func NewOrderController() *OrderController {
	return &OrderController{}
}


func (c *OrderController) GetOrders(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *OrderController) FindOrders(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *OrderController) CreateOrders(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *OrderController) UpdateOrders(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}

func (c *OrderController) DeleteOrders(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}



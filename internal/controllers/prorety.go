package controllers

import (
	"net/http"

	"request-system/pkg/utils"
	"github.com/labstack/echo/v4"
)

type ProretyController struct {}

func NewProretyController() *ProretyController {
	return &ProretyController{}
}

func (c *ProretyController) GetProreties(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *ProretyController) FindProreties(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *ProretyController) CreateProreties(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}


func (c *ProretyController) UpdateProreties(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}

func (c *ProretyController) DeleteProreties(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, utils.SuccessResponse("success") )
}





package controllers

import (
	"net/http"

	"request-system/internal/services"
	"github.com/labstack/echo/v4"
)

type UserController struct {
	service *services.UserService
}

func NewUserController(service *services.UserService) *UserController {
	return &UserController{service: service}
}

func (c *UserController) GetUsers(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "success")
}

func (c *UserController) FindUser(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "success")
}

func (c *UserController) CreateUser(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "success")
}

func (c *UserController) UpdateUser(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "success")
}

func (c *UserController) DeleteUser(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, "success")
}

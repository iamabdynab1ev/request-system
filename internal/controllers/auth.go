package controllers

import "github.com/labstack/echo/v4"

type AuthController struct {}

func NewAuthController() *AuthController {
    return &AuthController{}
}

func (c *AuthController) Login(ctx echo.Context) error {

	
    return ctx.JSON(200, "Login success")
}

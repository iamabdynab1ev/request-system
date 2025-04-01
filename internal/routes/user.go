package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/services"
	"github.com/labstack/echo/v4"
)

var userCtrl = controllers.NewUserController(&services.UserService{})

func RUN_USER_ROUTER(e *echo.Echo) {
	e.GET("/user", userCtrl.GetUsers)
	e.GET("/user/:id", userCtrl.FindUser)
	e.POST("/user", userCtrl.CreateUser)
	e.PUT("/user/:id", userCtrl.UpdateUser)
	e.DELETE("/user/:id", userCtrl.DeleteUser)
}

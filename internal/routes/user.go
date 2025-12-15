package routes

import (
	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/pkg/middleware"

	"github.com/labstack/echo/v4"
)

// Сигнатура функции изменилась!
func runUserRouter(
	secureGroup *echo.Group,
	userCtrl *controllers.UserController, // <<< ПРИНИМАЕМ ГОТОВЫЙ КОНТРОЛЛЕР
	authMW *middleware.AuthMiddleware,
) {
	secureGroup.GET("/ad-users", userCtrl.SearchADUsers, authMW.AuthorizeAny(authz.UserManageADLink))
	users := secureGroup.Group("/user")
	{
		users.POST("", userCtrl.CreateUser, authMW.AuthorizeAny(authz.UsersCreate))
		users.GET("", userCtrl.GetUsers, authMW.AuthorizeAny(authz.UsersView))
		users.GET("/:id", userCtrl.FindUser, authMW.AuthorizeAny(authz.UsersView))
		users.PUT("/:id", userCtrl.UpdateUser, authMW.AuthorizeAny(authz.UsersUpdate))
		users.DELETE("/:id", userCtrl.DeleteUser, authMW.AuthorizeAny(authz.UsersDelete))

		users.GET("/permission/:id", userCtrl.GetUserPermissions, authMW.AuthorizeAny(authz.UsersView))
		users.PUT("/permission/:id", userCtrl.UpdateUserPermissions, authMW.AuthorizeAny(authz.UsersUpdate))
	}
}

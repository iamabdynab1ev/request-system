package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/services"
	"github.com/labstack/echo/v4"
)

var (
	branchService = services.NewBranchService()
	branchCtrl    = controllers.NewBranchController(branchService)
)

func RUN_BRANCH_ROUTER(e *echo.Echo) {
	e.GET("branch", branchCtrl.GetBranches)
	e.GET("branch/:id", branchCtrl.FindBranch)
}

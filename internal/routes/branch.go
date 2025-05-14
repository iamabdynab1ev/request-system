package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_BRANCH_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		branchRepository = repositories.NewBranchRepository(dbConn)
		branchService    = services.NewBranchService(branchRepository)
		branchCtrl       = controllers.NewBranchController(branchService)
	)

	e.GET("branches", branchCtrl.GetBranches)
	e.GET("branches/:id", branchCtrl.FindBranch)
	e.POST("branches", branchCtrl.CreateBranch)
	e.PUT("branches/:id", branchCtrl.UpdateBranch)
	e.DELETE("branches/:id", branchCtrl.DeleteBranch)
}
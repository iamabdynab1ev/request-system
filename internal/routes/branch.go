package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_BRANCH_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {
	var (
		logger = logger.NewLogger()

		branchRepository = repositories.NewBranchRepository(dbConn)
		branchService    = services.NewBranchService(branchRepository, logger)
		branchCtrl       = controllers.NewBranchController(branchService, logger)
	)

	e.GET("branches", branchCtrl.GetBranches)
	e.GET("branche/:id", branchCtrl.FindBranch)
	e.POST("branche", branchCtrl.CreateBranch)
	e.PUT("branche/:id", branchCtrl.UpdateBranch)
	e.DELETE("branche/:id", branchCtrl.DeleteBranch)
}

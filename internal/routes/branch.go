package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runBranchRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	branchRepository := repositories.NewBranchRepository(dbConn)
	branchService := services.NewBranchService(branchRepository, logger)
	branchCtrl := controllers.NewBranchController(branchService, logger)

	secureGroup.GET("/branche", branchCtrl.GetBranches)
	secureGroup.GET("/branche/:id", branchCtrl.FindBranch)
	secureGroup.POST("/branche", branchCtrl.CreateBranch)
	secureGroup.PUT("/branche/:id", branchCtrl.UpdateBranch)
	secureGroup.DELETE("/branche/:id", branchCtrl.DeleteBranch)
}

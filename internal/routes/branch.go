// Сигнатура функции полностью соответствует вашему примеру для statusRouter
package routes

import (
	"request-system/internal/authz"
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func runBranchRouter(
	secureGroup *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	branchRepository := repositories.NewBranchRepository(dbConn, logger)
	userRepository := repositories.NewUserRepository(dbConn, logger)
	branchService := services.NewBranchService(branchRepository, userRepository, logger)

	branchCtrl := controllers.NewBranchController(branchService, logger)

	branches := secureGroup.Group("/branch")

	branches.GET("", branchCtrl.GetBranches, authMW.AuthorizeAny(authz.BranchesView))
	branches.GET("/:id", branchCtrl.FindBranch, authMW.AuthorizeAny(authz.BranchesView))
	branches.POST("", branchCtrl.CreateBranch, authMW.AuthorizeAny(authz.BranchesCreate))
	branches.PUT("/:id", branchCtrl.UpdateBranch, authMW.AuthorizeAny(authz.BranchesUpdate))
	branches.DELETE("/:id", branchCtrl.DeleteBranch, authMW.AuthorizeAny(authz.BranchesDelete))
}

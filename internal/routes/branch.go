package routes

import (
	// Импорты для DI
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	// Импорты для авторизации
	"request-system/internal/authz"
	"request-system/pkg/middleware"

	// Системные импорты
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Сигнатура функции полностью соответствует вашему примеру для statusRouter
func runBranchRouter(
	secureGroup *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware, // <-- Добавлен, как и в statusRouter
) {
	// 1. Инициализация зависимостей (проще, чем у статуса, т.к. нет файлов)
	branchRepository := repositories.NewBranchRepository(dbConn)
	branchService := services.NewBranchService(branchRepository, logger)
	branchCtrl := controllers.NewBranchController(branchService, logger)

	// 2. Создание группы роутов (рекомендуется использовать множественное число "branches")
	branches := secureGroup.Group("/branch")

	// 3. Регистрация эндпоинтов с применением middleware для проверки прав доступа
	branches.GET("", branchCtrl.GetBranches, authMW.AuthorizeAny(authz.BranchesView))
	branches.GET("/:id", branchCtrl.FindBranch, authMW.AuthorizeAny(authz.BranchesView))
	branches.POST("", branchCtrl.CreateBranch, authMW.AuthorizeAny(authz.BranchesCreate))
	branches.PUT("/:id", branchCtrl.UpdateBranch, authMW.AuthorizeAny(authz.BranchesUpdate))
	branches.DELETE("/:id", branchCtrl.DeleteBranch, authMW.AuthorizeAny(authz.BranchesDelete))
}

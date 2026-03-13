package routes

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/controllers"
	"request-system/internal/services"
	"request-system/pkg/middleware"
)

func runEquipImportRouter(
	group *echo.Group,
	dbConn *pgxpool.Pool,
	logger *zap.Logger,
	authMW *middleware.AuthMiddleware,
) {
	importSvc := services.NewEquipImportService(dbConn)
	ctrl := controllers.NewEquipImportController(importSvc, logger)

	equip := group.Group("/equipment-import")
	equip.POST("", ctrl.Import)
}

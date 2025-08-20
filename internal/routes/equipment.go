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

func runEquipmentRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware) {
	equipmentRepo := repositories.NewEquipmentRepository(dbConn, logger)
	userRepo := repositories.NewUserRepository(dbConn, logger)

	equipmentService := services.NewEquipmentService(equipmentRepo, userRepo, logger)
	equipmentCtrl := controllers.NewEquipmentController(equipmentService, logger)

	eqGroup := secureGroup.Group("/equipment")

	eqGroup.GET("", equipmentCtrl.GetEquipments, authMW.AuthorizeAny(authz.EquipmentsView))
	eqGroup.GET("/:id", equipmentCtrl.FindEquipment, authMW.AuthorizeAny(authz.EquipmentsView))
	eqGroup.POST("", equipmentCtrl.CreateEquipment, authMW.AuthorizeAny(authz.EquipmentsCreate))
	eqGroup.PUT("/:id", equipmentCtrl.UpdateEquipment, authMW.AuthorizeAny(authz.EquipmentsUpdate))
	eqGroup.DELETE("/:id", equipmentCtrl.DeleteEquipment, authMW.AuthorizeAny(authz.EquipmentsDelete))
}

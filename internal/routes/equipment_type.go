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

func runEquipmentTypeRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger, authMW *middleware.AuthMiddleware) {
	// Добавляем зависимости
	etRepository := repositories.NewEquipmentTypeRepository(dbConn, logger)
	userRepo := repositories.NewUserRepository(dbConn, logger)

	etService := services.NewEquipmentTypeService(etRepository, userRepo, logger)
	etCtrl := controllers.NewEquipmentTypeController(etService, logger)

	// Группируем роуты для чистоты
	etGroup := secureGroup.Group("/equipment_type")

	// Добавляем middleware для проверки прав
	etGroup.GET("", etCtrl.GetEquipmentTypes, authMW.AuthorizeAny(authz.EquipmentTypesView))
	etGroup.GET("/:id", etCtrl.FindEquipmentType, authMW.AuthorizeAny(authz.EquipmentTypesView))
	etGroup.POST("", etCtrl.CreateEquipmentType, authMW.AuthorizeAny(authz.EquipmentTypesCreate))
	etGroup.PUT("/:id", etCtrl.UpdateEquipmentType, authMW.AuthorizeAny(authz.EquipmentTypesUpdate))
	etGroup.DELETE("/:id", etCtrl.DeleteEquipmentType, authMW.AuthorizeAny(authz.EquipmentTypesDelete))
}

package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func RunOrderDocumentRouter(secureGroup *echo.Group, dbConn *pgxpool.Pool, logger *zap.Logger) {
	orderDocumentRepository := repositories.NewOrderDocumentRepository(dbConn)
	orderDocumentService := services.NewOrderDocumentService(orderDocumentRepository)
	orderDocumentCtrl := controllers.NewOrderDocumentController(orderDocumentService, logger)

	secureGroup.GET("/order-documents", orderDocumentCtrl.GetOrderDocuments)
	secureGroup.GET("/order-document/:id", orderDocumentCtrl.FindOrderDocument)
	secureGroup.POST("/order-document", orderDocumentCtrl.CreateOrderDocument)
	secureGroup.PUT("/order-document/:id", orderDocumentCtrl.UpdateOrderDocument)
	secureGroup.DELETE("/order-document/:id", orderDocumentCtrl.DeleteOrderDocument)
}

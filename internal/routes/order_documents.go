package routes

import (
	"request-system/internal/controllers"
	"request-system/internal/repositories"
	"request-system/internal/services"
	"request-system/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

func RUN_ORDER_DOCUMENT_ROUTER(e *echo.Echo, dbConn *pgxpool.Pool) {

	var (
		logger = logger.NewLogger()

		orderDocumentRepository = repositories.NewOrderDocumentRepository(dbConn)
		orderDocumentService    = services.NewOrderDocumentService(orderDocumentRepository)
		orderDocumentCtrl       = controllers.NewOrderDocumentController(orderDocumentService, logger)
	)
	e.GET("/order-documents", orderDocumentCtrl.GetOrderDocuments)
	e.GET("/order-document/:id", orderDocumentCtrl.FindOrderDocument)
	e.POST("/order-document", orderDocumentCtrl.CreateOrderDocument)
	e.PUT("/order-document/:id", orderDocumentCtrl.UpdateOrderDocument)
	e.DELETE("/order-document/:id", orderDocumentCtrl.DeleteOrderDocument)
}

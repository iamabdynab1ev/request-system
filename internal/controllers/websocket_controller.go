// internal/controllers/websocket_controller.go - ИСПРАВЛЕННАЯ ФИНАЛЬНАЯ ВЕРСИЯ

package controllers

import (
	"net/http"

	"request-system/pkg/service"
	appwebsocket "request-system/pkg/websocket"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// ИСПОЛЬЗУЕМ ИМЯ ПАКЕТА `websocket` НАПРЯМУЮ
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketController struct {
	hub        *appwebsocket.Hub // Используем наш тип с псевдонимом
	jwtService service.JWTService
	logger     *zap.Logger
}

func NewWebSocketController(hub *appwebsocket.Hub, jwtService service.JWTService, logger *zap.Logger) *WebSocketController {
	return &WebSocketController{
		hub:        hub,
		jwtService: jwtService,
		logger:     logger,
	}
}

func (c *WebSocketController) ServeWs(ctx echo.Context) error {
	tokenString := ctx.QueryParam("token")
	if tokenString == "" {
		return ctx.String(http.StatusUnauthorized, "Missing token")
	}

	claims, err := c.jwtService.ValidateToken(tokenString)
	if err != nil || claims.IsRefreshToken {
		return ctx.String(http.StatusUnauthorized, "Invalid token")
	}

	conn, err := upgrader.Upgrade(ctx.Response(), ctx.Request(), nil)
	if err != nil {
		c.logger.Error("WebSocket: не удалось улучшить соединение", zap.Error(err))
		return err
	}

	// ИСПОЛЬЗУЕМ КОНСТРУКТОР `NewClient`, в который передаем `conn` типа `*websocket.Conn`
	client := appwebsocket.NewClient(c.hub, conn, claims.UserID)
	client.Hub.Register <- client // Используем публичное поле

	go client.WritePump() // Используем публичный метод
	go client.ReadPump()  // Используем публичный метод

	c.logger.Info("WebSocket: клиент успешно подключен", zap.Uint64("userID", claims.UserID))
	return nil
}

package controllers

import (
	"net/http"
	"net/url"
	"strings"

	"request-system/pkg/service"
	appwebsocket "request-system/pkg/websocket"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		if websocketAllowAnyOrigin {
			return true
		}

		origin := normalizeOrigin(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}

		_, allowed := websocketAllowedOrigins[origin]
		return allowed
	},
	Subprotocols: []string{"bearer"},
}

var (
	websocketAllowedOrigins = map[string]struct{}{}
	websocketAllowAnyOrigin bool
)

type WebSocketController struct {
	hub        *appwebsocket.Hub
	jwtService service.JWTService
	logger     *zap.Logger
}

func NewWebSocketController(hub *appwebsocket.Hub, jwtService service.JWTService, logger *zap.Logger, allowedOrigins []string) *WebSocketController {
	websocketAllowedOrigins = map[string]struct{}{}
	websocketAllowAnyOrigin = false

	for _, origin := range allowedOrigins {
		normalized := normalizeOrigin(origin)
		if normalized == "*" {
			websocketAllowAnyOrigin = true
			continue
		}
		if normalized != "" {
			websocketAllowedOrigins[normalized] = struct{}{}
		}
	}

	return &WebSocketController{
		hub:        hub,
		jwtService: jwtService,
		logger:     logger,
	}
}

func (c *WebSocketController) ServeWs(ctx echo.Context) error {
	if ctx.QueryParam("token") != "" {
		return ctx.String(http.StatusUnauthorized, "Token in query string is not allowed")
	}

	tokenString, err := c.extractToken(ctx.Request())
	if err != nil {
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

	client := appwebsocket.NewClient(c.hub, conn, claims.UserID)
	client.Hub.Register <- client

	go client.WritePump()
	go client.ReadPump()

	c.logger.Info("WebSocket: клиент успешно подключен", zap.Uint64("userID", claims.UserID))
	return nil
}

func (c *WebSocketController) extractToken(r *http.Request) (string, error) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && strings.TrimSpace(parts[1]) != "" {
			return strings.TrimSpace(parts[1]), nil
		}
	}

	protocols := websocket.Subprotocols(r)
	if len(protocols) >= 2 && strings.EqualFold(protocols[0], "bearer") && strings.TrimSpace(protocols[1]) != "" {
		return strings.TrimSpace(protocols[1]), nil
	}

	return "", http.ErrNoCookie
}

func normalizeOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if raw == "*" {
		return "*"
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return strings.TrimSuffix(strings.ToLower(raw), "/")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimSuffix(strings.ToLower(raw), "/")
	}

	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

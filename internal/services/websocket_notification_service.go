// СОДЕРЖИМОЕ ДЛЯ internal/services/websocket_notification_service.go

package services

import (
	"go.uber.org/zap"

	"request-system/pkg/websocket"
)

// Интерфейс, чтобы можно было легко подменять в тестах
type WebSocketNotificationServiceInterface interface {
	SendNotification(userID uint64, payload interface{}, messageType string) error
}

// Конкретная реализация
type WebSocketNotificationService struct {
	hub    *websocket.Hub
	logger *zap.Logger
}

// Конструктор
func NewWebSocketNotificationService(hub *websocket.Hub, logger *zap.Logger) WebSocketNotificationServiceInterface {
	return &WebSocketNotificationService{
		hub:    hub,
		logger: logger,
	}
}

// Метод, который просто "пробрасывает" вызов в Hub
func (s *WebSocketNotificationService) SendNotification(userID uint64, payload interface{}, messageType string) error {
	s.logger.Info("Отправка WebSocket-уведомления",
		zap.Uint64("userID", userID),
		zap.String("type", messageType),
	)
	return s.hub.SendMessageToUser(userID, payload, messageType)
}

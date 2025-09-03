// Файл: internal/services/notification_service.go
package services

import "go.uber.org/zap"

// NotificationServiceInterface - интерфейс нашего сервиса уведомлений
type NotificationServiceInterface interface {
	SendPasswordResetEmail(to, token string) error
	SendPasswordResetSMS(to, code string) error
}

// mockNotificationService - это реализация-заглушка (mock), которая пишет в лог
// вместо реальной отправки сообщений. Идеально для тестирования.
type mockNotificationService struct {
	logger *zap.Logger
}

// NewMockNotificationService - конструктор для нашего сервиса-заглушки.
func NewMockNotificationService(logger *zap.Logger) NotificationServiceInterface {
	return &mockNotificationService{logger: logger}
}

// SendPasswordResetEmail имитирует отправку email.
func (s *mockNotificationService) SendPasswordResetEmail(to, token string) error {
	// В реальном приложении здесь будет код для интеграции с SendGrid, Mailgun и т.д.
	s.logger.Info("!!! ИМИТАЦИЯ ОТПРАВКИ EMAIL !!!",
		zap.String("кому", to),
		zap.String("токен_сброса", token),
		zap.String("готовая_ссылка", "https://your-frontend.com/password-reset?token="+token),
	)
	return nil // Всегда возвращаем успех для имитации
}

// SendPasswordResetSMS имитирует отправку SMS.
func (s *mockNotificationService) SendPasswordResetSMS(to, code string) error {
	// В реальном приложении здесь будет код для интеграции с Twilio или местным SMS-шлюзом.
	s.logger.Info("!!! ИМИТАЦИЯ ОТПРАВКИ SMS !!!",
		zap.String("кому", to),
		zap.String("код_верификации", code),
	)
	return nil
}

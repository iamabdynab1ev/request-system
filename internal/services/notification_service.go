package services

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"request-system/pkg/telegram"
)

// NotificationServiceInterface - Наш главный интерфейс для уведомлений.
type NotificationServiceInterface interface {
	// SendPlainMessage отправляет обычный текст, автоматически экранируя его.
	// Идеально для кодов верификации и простого текста.
	SendPlainMessage(ctx context.Context, chatID int64, message string) error

	// SendFormattedMessage отправляет текст "как есть", ожидая, что он уже содержит разметку Markdown.
	// Идеально для красивых уведомлений о заявках.
	SendFormattedMessage(ctx context.Context, chatID int64, message string) error
}

// ==========================================
// 1. Mock-реализация
// ==========================================
type mockNotificationService struct {
	logger *zap.Logger
}

func NewMockNotificationService(logger *zap.Logger) NotificationServiceInterface {
	return &mockNotificationService{logger: logger}
}

func (s *mockNotificationService) SendPlainMessage(ctx context.Context, chatID int64, message string) error {
	s.logger.Info("!!! MOCK: ОТПРАВКА PLAIN УВЕДОМЛЕНИЯ !!!", zap.Int64("chatID", chatID), zap.String("сообщение", message))
	return nil
}

func (s *mockNotificationService) SendFormattedMessage(ctx context.Context, chatID int64, message string) error {
	s.logger.Info("!!! MOCK: ОТПРАВКА FORMATTED УВЕДОМЛЕНИЯ !!!", zap.Int64("chatID", chatID), zap.String("сообщение (с разметкой)", message))
	return nil
}

// ==========================================
// 2. Telegram-реализация
// ==========================================
type telegramNotificationService struct {
	tgService telegram.ServiceInterface
	logger    *zap.Logger
}

func NewTelegramNotificationService(tgService telegram.ServiceInterface, logger *zap.Logger) NotificationServiceInterface {
	return &telegramNotificationService{tgService: tgService, logger: logger}
}

func (s *telegramNotificationService) SendPlainMessage(ctx context.Context, chatID int64, message string) error {
	if chatID == 0 {
		return fmt.Errorf("chat id не может быть 0")
	}
	// Экранируем спецсимволы, так как это простой текст
	escapedMessage := telegram.EscapeTextForMarkdownV2(message)
	return s.tgService.SendMessage(ctx, chatID, escapedMessage)
}

func (s *telegramNotificationService) SendFormattedMessage(ctx context.Context, chatID int64, message string) error {
	if chatID == 0 {
		return fmt.Errorf("chat id не может быть 0")
	}
	// Отправляем "как есть", доверяя вызывающему коду
	return s.tgService.SendMessageEx(ctx, chatID, message, telegram.WithMarkdownV2())
}

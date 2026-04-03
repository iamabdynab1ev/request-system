package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"request-system/pkg/telegram"
)

var markdownLinkPattern = regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)

type NotificationServiceInterface interface {
	SendPlainMessage(ctx context.Context, chatID int64, message string) error

	SendFormattedMessage(ctx context.Context, chatID int64, message string) error
}

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
	return s.tgService.SendMessageEx(ctx, chatID, telegram.EscapeTextForMarkdownV2(message), telegram.WithMarkdownV2())
}

func (s *telegramNotificationService) SendFormattedMessage(ctx context.Context, chatID int64, message string) error {
	if chatID == 0 {
		return fmt.Errorf("chat id не может быть 0")
	}
	// Отправляем "как есть", доверяя вызывающему коду
	err := s.tgService.SendMessageEx(ctx, chatID, message, telegram.WithMarkdownV2())
	if err == nil {
		return nil
	}

	s.logger.Warn("Telegram formatted notification failed, retrying as plain message",
		zap.Int64("chat_id", chatID),
		zap.Error(err))

	fallback := telegram.EscapeTextForMarkdownV2(normalizeTelegramMessageForPlainText(message))
	return s.tgService.SendMessageEx(ctx, chatID, fallback, telegram.WithMarkdownV2())
}

func normalizeTelegramMessageForPlainText(message string) string {
	normalized := markdownLinkPattern.ReplaceAllString(message, "$1: $2")
	normalized = strings.ReplaceAll(normalized, "*", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "`", "")
	return normalized
}

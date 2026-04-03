package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"request-system/pkg/config"
)

const telegramWebhookSecretHeader = "X-Telegram-Bot-Api-Secret-Token"

type TelegramIntegrationServiceInterface interface {
	Enabled() bool
	BuildBotStartLink(token string) string
	ValidateWebhookRequest(r *http.Request) error
	RegisterWebhook(ctx context.Context, baseURL string) (*TelegramWebhookInfo, error)
	GetWebhookInfo(ctx context.Context) (*TelegramWebhookInfo, error)
}

type TelegramWebhookInfo struct {
	URL                string `json:"url"`
	PendingUpdateCount int    `json:"pending_update_count"`
	LastErrorDate      int64  `json:"last_error_date,omitempty"`
	LastErrorMessage   string `json:"last_error_message,omitempty"`
	MaxConnections     int    `json:"max_connections,omitempty"`
	IPAddress          string `json:"ip_address,omitempty"`
}

type TelegramIntegrationService struct {
	botToken           string
	botUsername        string
	webhookSecretToken string
	logger             *zap.Logger
	httpClient         *http.Client
}

type telegramAPIResponse[T any] struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	ErrorCode   int    `json:"error_code,omitempty"`
	Result      T      `json:"result"`
}

type telegramSetWebhookRequest struct {
	URL            string   `json:"url"`
	SecretToken    string   `json:"secret_token,omitempty"`
	AllowedUpdates []string `json:"allowed_updates,omitempty"`
	MaxConnections int      `json:"max_connections,omitempty"`
	DropPending    bool     `json:"drop_pending_updates,omitempty"`
}

func NewTelegramIntegrationService(cfg config.TelegramConfig, logger *zap.Logger) TelegramIntegrationServiceInterface {
	service := &TelegramIntegrationService{
		botToken:           strings.TrimSpace(cfg.BotToken),
		botUsername:        strings.TrimPrefix(strings.TrimSpace(cfg.BotUsername), "@"),
		webhookSecretToken: strings.TrimSpace(cfg.WebhookSecretToken),
		logger:             logger,
		httpClient:         &http.Client{Timeout: 15 * time.Second},
	}

	if service.Enabled() && service.botUsername == "" {
		logger.Warn("Telegram deep links disabled: TELEGRAM_BOT_USERNAME is empty")
	}

	if service.Enabled() && service.webhookSecretToken == "" {
		logger.Warn("Telegram webhook request validation disabled: TELEGRAM_WEBHOOK_SECRET_TOKEN is empty")
	}

	return service
}

func (s *TelegramIntegrationService) Enabled() bool {
	return strings.TrimSpace(s.botToken) != ""
}

func (s *TelegramIntegrationService) BuildBotStartLink(token string) string {
	if !s.Enabled() || s.botUsername == "" || strings.TrimSpace(token) == "" {
		return ""
	}

	return fmt.Sprintf("https://t.me/%s?start=%s", s.botUsername, url.QueryEscape(strings.TrimSpace(token)))
}

func (s *TelegramIntegrationService) ValidateWebhookRequest(r *http.Request) error {
	if strings.TrimSpace(s.webhookSecretToken) == "" {
		return nil
	}

	if got := strings.TrimSpace(r.Header.Get(telegramWebhookSecretHeader)); got != s.webhookSecretToken {
		return fmt.Errorf("telegram webhook secret token mismatch")
	}

	return nil
}

func (s *TelegramIntegrationService) RegisterWebhook(ctx context.Context, baseURL string) (*TelegramWebhookInfo, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("telegram bot token is not configured")
	}

	webhookURL, err := buildTelegramWebhookURL(baseURL)
	if err != nil {
		return nil, err
	}

	payload := telegramSetWebhookRequest{
		URL:            webhookURL,
		SecretToken:    s.webhookSecretToken,
		AllowedUpdates: []string{"message", "callback_query"},
		MaxConnections: 40,
	}

	var result bool
	if err := s.callTelegramAPI(ctx, "setWebhook", payload, &result); err != nil {
		return nil, err
	}

	info, err := s.GetWebhookInfo(ctx)
	if err != nil {
		return nil, err
	}

	s.logger.Info(
		"Telegram webhook registered",
		zap.String("webhook_url", webhookURL),
		zap.Int("pending_updates", info.PendingUpdateCount),
		zap.String("telegram_ip", info.IPAddress),
	)

	return info, nil
}

func (s *TelegramIntegrationService) GetWebhookInfo(ctx context.Context) (*TelegramWebhookInfo, error) {
	if !s.Enabled() {
		return nil, fmt.Errorf("telegram bot token is not configured")
	}

	var info TelegramWebhookInfo
	if err := s.callTelegramAPI(ctx, "getWebhookInfo", nil, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

func (s *TelegramIntegrationService) callTelegramAPI(ctx context.Context, method string, payload any, out any) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", s.botToken, method)

	var bodyReader *bytes.Reader
	if payload == nil {
		bodyReader = bytes.NewReader(nil)
	} else {
		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("telegram %s: marshal request: %w", method, err)
		}
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bodyReader)
	if err != nil {
		return fmt.Errorf("telegram %s: create request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram %s: request failed: %w", method, err)
	}
	defer resp.Body.Close()

	var telegramResp telegramAPIResponse[json.RawMessage]
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		return fmt.Errorf("telegram %s: decode response: %w", method, err)
	}

	if !telegramResp.OK {
		return fmt.Errorf("telegram %s: api error %d: %s", method, telegramResp.ErrorCode, telegramResp.Description)
	}

	if out != nil && len(telegramResp.Result) > 0 {
		if err := json.Unmarshal(telegramResp.Result, out); err != nil {
			return fmt.Errorf("telegram %s: decode result: %w", method, err)
		}
	}

	return nil
}

func buildTelegramWebhookURL(baseURL string) (string, error) {
	cleanBaseURL := strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	if cleanBaseURL == "" {
		return "", fmt.Errorf("server base url is empty")
	}

	parsedURL, err := url.Parse(cleanBaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid server base url: %w", err)
	}

	if parsedURL.Scheme != "https" {
		return "", fmt.Errorf("telegram webhook requires https base url")
	}

	return parsedURL.String() + "/api/webhooks/telegram", nil
}

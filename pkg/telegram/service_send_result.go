package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (s *Service) SendMessageWithID(ctx context.Context, chatID int64, text string, options ...MessageOption) (int, error) {
	if s.botToken == "" {
		return 0, fmt.Errorf("telegram bot token is not configured")
	}

	reqPayload := &sendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}

	for _, opt := range options {
		opt(reqPayload)
	}

	var result telegramMessageResult
	if err := s.sendRequestForResult(ctx, "sendMessage", reqPayload, &result); err != nil {
		return 0, err
	}

	return result.MessageID, nil
}

func (s *Service) sendRequestForResult(ctx context.Context, methodName string, payload interface{}, out interface{}) error {
	if s.botToken == "" {
		return fmt.Errorf("telegram bot token is not configured")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", s.botToken, methodName)

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Telegram request payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send Telegram request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if s.debug {
		fmt.Printf("[telegram] %s -> %s\nRequest: %s\nResponse: %s\n\n", methodName, apiURL, string(reqBody), string(body))
	}

	var telegramResp struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description,omitempty"`
		ErrorCode   int             `json:"error_code,omitempty"`
		Result      json.RawMessage `json:"result,omitempty"`
	}

	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return fmt.Errorf("failed to decode Telegram API response: %w", err)
	}

	if !telegramResp.OK {
		return fmt.Errorf("telegram API error (%s): code %d, description: %s", methodName, telegramResp.ErrorCode, telegramResp.Description)
	}

	if out != nil && len(telegramResp.Result) > 0 {
		if err := json.Unmarshal(telegramResp.Result, out); err != nil {
			return fmt.Errorf("failed to decode Telegram API result: %w", err)
		}
	}

	return nil
}

// Файл: pkg/telegram/service.go
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// --- ОСНОВНОЙ ИНТЕРФЕЙС СЕРВИСА ---

type ServiceInterface interface {
	SendMessage(ctx context.Context, chatID int64, text string) error

	SendMessageEx(ctx context.Context, chatID int64, text string, options ...MessageOption) error

	AnswerCallbackQuery(ctx context.Context, callbackQueryID string, text string) error

	EditMessageText(ctx context.Context, chatID int64, messageID int, text string, options ...MessageOption) error
	EditOrSendMessage(ctx context.Context, chatID int64, messageID int, text string, options ...MessageOption) error
}

// --- СТРУКТУРА СЕРВИСА ---

type Service struct {
	botToken   string
	httpClient *http.Client
	debug      bool
}

func NewService(botToken string) ServiceInterface {
	debug := strings.Contains(strings.ToLower(os.Getenv("DEBUG")), "telegram")

	return &Service{
		botToken:   botToken,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		debug:      debug,
	}
}

// --- ОСНОВНЫЕ СТРУКТУРЫ ЗАПРОСОВ ---

type sendMessageRequest struct {
	ChatID      int64       `json:"chat_id"`
	Text        string      `json:"text"`
	ParseMode   string      `json:"parse_mode,omitempty"`
	ReplyMarkup interface{} `json:"reply_markup,omitempty"`
}

type inlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}
type ReplyKeyboardButton struct {
	Text string `json:"text"`
}

type replyKeyboardMarkup struct {
	Keyboard        [][]ReplyKeyboardButton `json:"keyboard"`
	ResizeKeyboard  bool                    `json:"resize_keyboard"`
	OneTimeKeyboard bool                    `json:"one_time_keyboard,omitempty"`
}

type callbackQueryRequest struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
	ShowAlert       bool   `json:"show_alert,omitempty"`
}
type editMessageTextRequest struct {
	ChatID      int64       `json:"chat_id"`
	MessageID   int         `json:"message_id"`
	Text        string      `json:"text"`
	ParseMode   string      `json:"parse_mode,omitempty"`
	ReplyMarkup interface{} `json:"reply_markup,omitempty"`
}

type MessageOption func(*sendMessageRequest)

func WithKeyboard(rows [][]InlineKeyboardButton) MessageOption {
	return func(req *sendMessageRequest) {
		if len(rows) > 0 {
			req.ReplyMarkup = inlineKeyboardMarkup{InlineKeyboard: rows}
		}
	}
}

func WithMarkdownV2() MessageOption {
	return func(req *sendMessageRequest) {
		req.ParseMode = "MarkdownV2"
	}
}

func WithHTML() MessageOption {
	return func(req *sendMessageRequest) {
		req.ParseMode = "HTML"
	}
}

func WithReplyKeyboard(rows [][]ReplyKeyboardButton) MessageOption {
	return func(req *sendMessageRequest) {
		if len(rows) > 0 {
			req.ReplyMarkup = replyKeyboardMarkup{
				Keyboard:       rows,
				ResizeKeyboard: true,
			}
		}
	}
}

func (s *Service) EditMessageText(ctx context.Context, chatID int64, messageID int, text string, options ...MessageOption) error {
	if messageID == 0 {
		return s.SendMessageEx(ctx, chatID, text, options...)
	}

	editReq := &editMessageTextRequest{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
	}

	tempSendReq := &sendMessageRequest{}
	for _, opt := range options {
		opt(tempSendReq)
	}

	editReq.ParseMode = tempSendReq.ParseMode
	editReq.ReplyMarkup = tempSendReq.ReplyMarkup

	return s.sendRequest(ctx, "editMessageText", editReq)
}


func (s *Service) SendMessage(ctx context.Context, chatID int64, text string) error {
	escapedText := EscapeTextForMarkdownV2(text)
	return s.SendMessageEx(ctx, chatID, escapedText, WithMarkdownV2())
}

func (s *Service) SendMessageEx(ctx context.Context, chatID int64, text string, options ...MessageOption) error {
	if s.botToken == "" {
		return fmt.Errorf("токен Telegram-бота не установлен")
	}

	reqPayload := &sendMessageRequest{
		ChatID: chatID,
		Text:   text,
	}

	for _, opt := range options {
		opt(reqPayload)
	}

	return s.sendRequest(ctx, "sendMessage", reqPayload)
}

// Ответ на callback-кнопку
func (s *Service) AnswerCallbackQuery(ctx context.Context, callbackQueryID string, text string) error {
	if callbackQueryID == "" {
		return fmt.Errorf("callbackQueryID не может быть пустым")
	}

	reqPayload := callbackQueryRequest{
		CallbackQueryID: callbackQueryID,
		Text:            text,
	}
	return s.sendRequest(ctx, "answerCallbackQuery", reqPayload)
}

// --- ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ---

func (s *Service) sendRequest(ctx context.Context, methodName string, payload interface{}) error {
	if s.botToken == "" {
		return fmt.Errorf("токен Telegram-бота не установлен")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", s.botToken, methodName)

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("ошибка сериализации JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка отправки запроса в Telegram: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if s.debug {
		fmt.Printf("[telegram] %s → %s\nRequest: %s\nResponse: %s\n\n", methodName, apiURL, string(reqBody), string(body))
	}

	// Telegram всегда возвращает 200 OK, даже при ошибках
	var telegramResp struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description,omitempty"`
		ErrorCode   int             `json:"error_code,omitempty"`
		Result      json.RawMessage `json:"result,omitempty"`
	}

	if err := json.Unmarshal(body, &telegramResp); err != nil {
		return fmt.Errorf("ошибка декодирования ответа Telegram API: %w", err)
	}

	if !telegramResp.OK {
		return fmt.Errorf("telegram API ошибка (%s): код %d, описание: %s", methodName, telegramResp.ErrorCode, telegramResp.Description)
	}

	return nil
}

// --- ЭКРАНИРОВАНИЕ ДЛЯ MARKDOWNV2 ---

func EscapeTextForMarkdownV2(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]",
		"(", "\\(", ")", "\\)", "\\", "\\\\",
		"~", "\\~", "`", "\\`", ">", "\\>", "#", "\\#", "+", "\\+",
		"-", "\\-", "=", "\\=", "|", "\\|", "{", "\\{", "}", "\\}", ".", "\\.", "!", "\\!",
	)
	return replacer.Replace(text)
}

func (s *Service) EditOrSendMessage(ctx context.Context, chatID int64, messageID int, text string, options ...MessageOption) error {

	if messageID == 0 {
		return s.SendMessageEx(ctx, chatID, text, options...)
	}
	return s.EditMessageText(ctx, chatID, messageID, text, options...)
}

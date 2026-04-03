package dto

type TelegramLinkStatusDTO struct {
	Linked         bool   `json:"linked"`
	TelegramChatID *int64 `json:"telegram_chat_id,omitempty"`
}

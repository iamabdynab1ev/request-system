package dto

type TelegramLinkStatusDTO struct {
	Linked         bool   `json:"linked"`
	TelegramChatID *int64 `json:"telegram_chat_id,omitempty"`
}

type TelegramLinkTokenDTO struct {
	Token            string `json:"token"`
	ShortCode        string `json:"short_code"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

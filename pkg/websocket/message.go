package websocket

import "time"

// Envelope — это "конверт", в котором мы отправляем наши сообщения.
// Он содержит тип сообщения, что позволяет фронтенду понять, что делать.
type Envelope struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// NotificationPayload — это структура нашего уведомления из "колокольчика".
// Это наш DTO для фронтенда.
type NotificationPayload struct {
	EventID   string       `json:"eventId"`
	Type      string       `json:"type"`
	IsRead    bool         `json:"isRead"`
	Actor     ActorInfo    `json:"actor"`
	Message   string       `json:"message"`
	Changes   []ChangeInfo `json:"changes"`
	Links     LinkInfo     `json:"links"`
	CreatedAt time.Time    `json:"created_at"`
}

// Вспомогательные структуры для NotificationPayload
type ActorInfo struct {
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatarUrl,omitempty"`
}
type ChangeInfo struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
type LinkInfo struct {
	Primary    string  `json:"primary"`
	Attachment *string `json:"attachment,omitempty"`
}

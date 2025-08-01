package dto

type TimelineEventDTO struct {
	Icon      string       `json:"icon"`
	Lines     []string     `json:"lines"`
	Actor     ShortUserDTO `json:"actor"`
	CreatedAt string       `json:"created_at"`
}

type CreateOrderHistoryDTO struct {
	OrderID      uint64  `json:"order_id"`
	UserID       uint64  `json:"user_id"`
	EventType    string  `json:"event_type"`
	OldValue     *string `json:"old_value,omitempty"`
	NewValue     *string `json:"new_value,omitempty"`
	Comment      *string `json:"comment,omitempty"`
	AttachmentID *uint64 `json:"attachment_id,omitempty"`
}

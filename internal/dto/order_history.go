package dto

type TimelineEventDTO struct {
	Icon      string       `json:"icon"`  // Например: "status_open", "status_inprogress", "comment", "file"
	Lines     []string     `json:"lines"` // Несколько строк текста для одного события
	Actor     ShortUserDTO `json:"actor"`
	CreatedAt string       `json:"created_at"`
}
type CreateOrderHistoryDTO struct {
	OrderID   uint64
	UserID    uint64
	EventType string
	NewValue  string
	OldValue  string
	Comment   string
}

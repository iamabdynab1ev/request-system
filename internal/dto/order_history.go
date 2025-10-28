package dto

// TimelineEventDTO - структура ответа для timeline (с Role для UI/отчётов)
type TimelineEventDTO struct {
	Lines      []string               `json:"lines"`                // Список строк события
	Comment    *string                `json:"comment,omitempty"`    // Комментарий (если есть)
	Actor      ShortUserDTO           `json:"actor"`                // Информация об акторе
	Role       string                 `json:"role,omitempty"`       // Роль актора (creator, delegator, executor, participant)
	CreatedAt  string                 `json:"created_at"`           // Время события
	Attachment *AttachmentResponseDTO `json:"attachment,omitempty"` // Вложение (если есть)
}

type CreateOrderHistoryDTO struct {
	OrderID      uint64  `json:"order_id" validate:"required"`   // ID заявки
	UserID       uint64  `json:"user_id" validate:"required"`    // ID пользователя
	EventType    string  `json:"event_type" validate:"required"` // Тип события
	OldValue     *string `json:"old_value,omitempty"`            // Старое значение
	NewValue     *string `json:"new_value,omitempty"`            // Новое значение
	Comment      *string `json:"comment,omitempty"`              // Комментарий
	AttachmentID *uint64 `json:"attachment_id,omitempty"`        // ID вложения
}

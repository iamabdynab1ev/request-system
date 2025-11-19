package dto

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type TelegramState struct {
	Mode      string            `json:"mode"`
	OrderID   uint64            `json:"order_id"`
	MessageID int               `json:"message_id"`
	Changes   map[string]string `json:"changes"`
}

// NewTelegramState создает новое состояние
func NewTelegramState(orderID uint64, messageID int) *TelegramState {
	return &TelegramState{
		Mode:      "editing_order",
		OrderID:   orderID,
		MessageID: messageID,
		Changes:   make(map[string]string),
	}
}

func (s *TelegramState) SetDuration(t *time.Time) {
	if t == nil {
		s.Changes["duration"] = ""
	} else {
		s.Changes["duration"] = t.Format(time.RFC3339)
	}
}

// GetDuration получает срок выполнения
func (s *TelegramState) GetDuration() (*time.Time, error) {
	val, ok := s.Changes["duration"]
	if !ok || val == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return nil, fmt.Errorf("invalid duration format: %w", err)
	}
	return &t, nil
}

// ClearDuration очищает срок выполнения
func (s *TelegramState) ClearDuration() {
	s.Changes["duration"] = ""
}

// === Методы для работы с ID полями ===

// SetStatusID устанавливает ID статуса
func (s *TelegramState) SetStatusID(id uint64) {
	s.Changes["status_id"] = strconv.FormatUint(id, 10)
}

// GetStatusID получает ID статуса
func (s *TelegramState) GetStatusID() (uint64, bool, error) {
	val, ok := s.Changes["status_id"]
	if !ok || val == "" {
		return 0, false, nil
	}
	id, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid status_id format: %w", err)
	}
	return id, true, nil
}

// SetExecutorID устанавливает ID исполнителя
func (s *TelegramState) SetExecutorID(id uint64) {
	s.Changes["executor_id"] = strconv.FormatUint(id, 10)
}

// GetExecutorID получает ID исполнителя
func (s *TelegramState) GetExecutorID() (uint64, bool, error) {
	val, ok := s.Changes["executor_id"]
	if !ok || val == "" {
		return 0, false, nil
	}
	id, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid executor_id format: %w", err)
	}
	return id, true, nil
}

// === Методы для работы с текстовыми полями ===

// SetComment устанавливает комментарий
func (s *TelegramState) SetComment(comment string) {
	s.Changes["comment"] = comment
}

// GetComment получает комментарий
func (s *TelegramState) GetComment() (string, bool) {
	val, ok := s.Changes["comment"]
	return val, ok
}

// === Универсальные методы ===

// HasChanges проверяет наличие изменений
func (s *TelegramState) HasChanges() bool {
	return len(s.Changes) > 0
}

// ClearChanges очищает все изменения
func (s *TelegramState) ClearChanges() {
	s.Changes = make(map[string]string)
}

// ToJSON конвертирует состояние в JSON
func (s *TelegramState) ToJSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}
	return string(data), nil
}

// FromJSON создает состояние из JSON
func FromJSON(data string) (*TelegramState, error) {
	var state TelegramState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}
	if state.Changes == nil {
		state.Changes = make(map[string]string)
	}
	return &state, nil
}

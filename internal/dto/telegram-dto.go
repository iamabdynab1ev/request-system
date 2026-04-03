package dto

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type TelegramState struct {
	Mode        string            `json:"mode"`
	OrderID     uint64            `json:"order_id"`
	MessageID   int               `json:"message_id"`
	Source      string            `json:"source,omitempty"`
	SearchQuery string            `json:"search_query,omitempty"`
	Changes     map[string]string `json:"changes"`
}

func NewTelegramState(orderID uint64, messageID int, source string, searchQuery string) *TelegramState {
	return &TelegramState{
		Mode:        "editing_order",
		OrderID:     orderID,
		MessageID:   messageID,
		Source:      source,
		SearchQuery: searchQuery,
		Changes:     make(map[string]string),
	}
}

func (s *TelegramState) SetDuration(t *time.Time) {
	if t == nil {
		s.Changes["duration"] = ""
	} else {
		s.Changes["duration"] = t.Format(time.RFC3339)
	}
}

func (s *TelegramState) GetDuration() (*time.Time, bool, error) {
	if _, cleared := s.Changes["duration_cleared"]; cleared {
		return nil, true, nil
	}
	val, ok := s.Changes["duration"]
	if !ok {
		return nil, false, nil
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return nil, false, fmt.Errorf("invalid duration format: %w", err)
	}
	return &t, true, nil
}

func (s *TelegramState) ClearDuration() {
	s.Changes["duration_cleared"] = "true"
	delete(s.Changes, "duration")
}

func (s *TelegramState) SetStatusID(id uint64) {
	s.Changes["status_id"] = strconv.FormatUint(id, 10)
}

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

func (s *TelegramState) SetExecutorID(id uint64) {
	s.Changes["executor_id"] = strconv.FormatUint(id, 10)
}

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

func (s *TelegramState) SetComment(comment string) {
	s.Changes["comment"] = comment
}

func (s *TelegramState) GetComment() (string, bool) {
	val, ok := s.Changes["comment"]
	return val, ok
}

func (s *TelegramState) HasChanges() bool {
	for k := range s.Changes {
		if k != "duration_cleared" {
			return true
		}
	}
	if _, cleared := s.Changes["duration_cleared"]; cleared {
		return true
	}
	return false
}

func (s *TelegramState) ClearChanges() {
	s.Changes = make(map[string]string)
}

func (s *TelegramState) ToJSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}
	return string(data), nil
}

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

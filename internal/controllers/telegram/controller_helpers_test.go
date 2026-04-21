package telegram

import (
	"testing"

	"request-system/internal/entities"
)

func TestGetStatusEmojiReturnsReadableUnicode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{name: "open", code: "OPEN", want: "❗"},
		{name: "in progress", code: "IN_PROGRESS", want: "⏳"},
		{name: "refinement", code: "REFINEMENT", want: "🔁"},
		{name: "clarification", code: "CLARIFICATION", want: "❓"},
		{name: "completed", code: "COMPLETED", want: "✅"},
		{name: "closed", code: "CLOSED", want: "✔️"},
		{name: "rejected", code: "REJECTED", want: "❌"},
		{name: "confirmed", code: "CONFIRMED", want: "🔄"},
		{name: "service", code: "SERVICE", want: "🛠️"},
		{name: "unknown", code: "UNKNOWN", want: "🔷"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &entities.Status{Code: &tt.code}
			if got := getStatusEmoji(status); got != tt.want {
				t.Fatalf("getStatusEmoji(%q) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestGetStatusEmojiHandlesNilStatus(t *testing.T) {
	if got := getStatusEmoji(nil); got != "🔷" {
		t.Fatalf("getStatusEmoji(nil) = %q, want default emoji", got)
	}

	status := &entities.Status{}
	if got := getStatusEmoji(status); got != "🔷" {
		t.Fatalf("getStatusEmoji(status without code) = %q, want default emoji", got)
	}
}

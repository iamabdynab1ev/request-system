package utils

import (
	"fmt"
	"math"
	"strings"
)

// FormatSecondsToHumanReadable преобразует секунды (целое число) в строку вида "1д 2ч 3м 4с".
func FormatSecondsToHumanReadable(totalSeconds uint64) string {
	if totalSeconds == 0 {
		return "0с"
	}

	days := totalSeconds / (24 * 3600)
	totalSeconds %= (24 * 3600)
	hours := totalSeconds / 3600
	totalSeconds %= 3600
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dд", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dч", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dм", minutes))
	}
	// Показываем секунды, только если это единственная единица времени или если они не равны нулю
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dс", seconds))
	}

	return strings.Join(parts, " ")
}

// FormatFloatSecondsToHumanReadable преобразует секунды (дробное число) в строку.
// Используется для средних значений в дашборде.
func FormatFloatSecondsToHumanReadable(totalSeconds float64) string {
	if totalSeconds < 1 {
		return "0с"
	}
	// Просто округляем до ближайшей секунды и используем уже существующую функцию
	return FormatSecondsToHumanReadable(uint64(math.Round(totalSeconds)))
}

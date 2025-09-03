// Файл: pkg/utils/phone_helpers.go
package utils

import (
	"regexp"
	"strings"
)

var digitsOnly = regexp.MustCompile(`^\d+$`)

// NormalizeTajikPhoneNumber приводит разные форматы таджикских номеров к единому
// виду, который хранится в БД (12 цифр, например, "992955555555").
// Возвращает пустую строку, если формат неверный.
func NormalizeTajikPhoneNumber(phone string) string {
	// 1. Убираем все символы, кроме цифр
	phone = strings.TrimPrefix(phone, "+")
	if !digitsOnly.MatchString(phone) {
		return "" // Если после очистки остались не-цифры, это не номер
	}

	// 2. Приводим к стандартному формату (12 цифр)
	if len(phone) == 9 {
		// Формат "955555555" -> "992955555555"
		return "992" + phone
	} else if len(phone) == 12 && strings.HasPrefix(phone, "992") {
		// Формат "992955555555" -> уже правильный
		return phone
	}

	// Все остальные форматы считаем неверными
	return ""
}

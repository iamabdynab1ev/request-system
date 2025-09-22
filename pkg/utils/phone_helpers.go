package utils

import (
	"regexp"
	"strings"
)

// nonDigitRegexp находит все символы, которые НЕ являются цифрами.
// Он лучше, чем твой digitsOnly, так как позволяет очищать строки вида "+992 (92) ..."
var nonDigitRegexp = regexp.MustCompile(`\D`)

// NormalizeTajikPhoneNumber приводит разные форматы таджикских номеров
// к единому 9-значному виду, который будет храниться в БД (например, "927771122").
// Возвращает пустую строку, если формат неверный.
func NormalizeTajikPhoneNumber(phone string) string {
	// 1. Убираем из входной строки ВСЁ, кроме цифр.
	// "+992 (92) 777-11-22" -> "992927771122"
	// "8(92) 777-11-22"   -> "8927771122"
	digitsOnly := nonDigitRegexp.ReplaceAllString(phone, "")

	// 2. Проверяем, начинается ли номер с кода страны '992'
	if strings.HasPrefix(digitsOnly, "992") {
		// Если да, отрезаем этот префикс, чтобы получить 9 цифр
		// "992927771122" -> "927771122"
		phoneWithoutCountryCode := digitsOnly[3:]
		if len(phoneWithoutCountryCode) == 9 {
			return phoneWithoutCountryCode
		}
	}

	// 3. Проверяем, не является ли номер уже 9-значным
	if len(digitsOnly) == 9 {
		// "927771122" -> "927771122"
		return digitsOnly
	}

	// 4. Если мы не смогли привести номер к 9-значному формату,
	// считаем его неверным и возвращаем пустую строку.
	return ""
}

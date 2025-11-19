package utils

import (
	"regexp"
)

var nonDigitRegexp = regexp.MustCompile(`\D`)

func NormalizeTajikPhoneNumber(phone string) string {
	digitsOnly := nonDigitRegexp.ReplaceAllString(phone, "")
	if len(digitsOnly) < 9 {
		return ""
	}
	return digitsOnly[len(digitsOnly)-9:]
}

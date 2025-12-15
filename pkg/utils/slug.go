package utils

import (
	"regexp"
	"strings"
)

// GenerateCodeFromName создает системный CODE из названия.
// "Ремонт Принтера!" -> "REMONT_PRINTERA"
func GenerateCodeFromName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))

	// Заменяем кириллицу (и таджикские буквы) на латиницу
	replacements := map[string]string{
		"а": "a", "б": "b", "в": "v", "г": "g", "д": "d",
		"е": "e", "ё": "yo", "ж": "zh", "з": "z", "и": "i",
		"й": "y", "к": "k", "л": "l", "м": "m", "н": "n",
		"о": "o", "п": "p", "р": "r", "с": "s", "т": "t",
		"у": "u", "ф": "f", "х": "h", "ц": "ts", "ч": "ch",
		"ш": "sh", "щ": "sch", "ъ": "", "ы": "y", "ь": "",
		"э": "e", "ю": "yu", "я": "ya",
		"ғ": "gh", "ӣ": "i", "қ": "q", "ӯ": "u", "ҳ": "h", "ҷ": "j",
	}

	var sb strings.Builder
	for _, r := range s {
		char := string(r)
		if repl, ok := replacements[char]; ok {
			sb.WriteString(repl)
		} else {
			sb.WriteString(char)
		}
	}
	res := sb.String()

	// Заменяем всё кроме букв и цифр на нижнее подчеркивание
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	res = reg.ReplaceAllString(res, "_")

	// Убираем лишние подчеркивания по краям и переводим в капс
	res = strings.Trim(res, "_")
	return strings.ToUpper(res)
}

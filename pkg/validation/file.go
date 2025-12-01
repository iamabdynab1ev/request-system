package validation

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"slices"

	"request-system/config"
)

// ValidateFile проверяет размер и MIME-тип файла
// contextName - ключ из config.UploadContexts (например, "avatar", "documents")
func ValidateFile(fileHeader *multipart.FileHeader, file io.ReadSeeker, contextName string) error {
	// 1. Получаем правила из конфига
	rules, ok := config.UploadContexts[contextName]
	if !ok {
		return fmt.Errorf("внутренняя ошибка: неизвестный контекст загрузки '%s'", contextName)
	}

	// 2. Проверка размера (если ограничение > 0)
	if rules.MaxSizeMB > 0 {
		maxSizeBytes := int64(rules.MaxSizeMB) * 1024 * 1024
		if fileHeader.Size > maxSizeBytes {
			return fmt.Errorf("размер файла (%.2f MB) превышает лимит в %d MB", float64(fileHeader.Size)/1024/1024, rules.MaxSizeMB)
		}
	}

	// 3. Проверка содержимого (Magic Numbers)
	// Читаем заголовок файла (первые 512 байт)
	buffer := make([]byte, 512)
	if _, err := file.Read(buffer); err != nil && err != io.EOF {
		return fmt.Errorf("ошибка чтения файла")
	}

	// Важно: Возвращаем курсор чтения в начало!
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("ошибка обработки файла")
	}

	// Определяем тип
	mimeType := http.DetectContentType(buffer)

	// Хак для SVG/XML, которые часто определяются как text/plain
	if isPossibleXml(mimeType) {
		if isSvgSignature(buffer) {
			mimeType = "image/svg+xml"
		}
	}

	// 4. Сверка с разрешенными типами
	if !slices.Contains(rules.AllowedMimeTypes, mimeType) {
		return fmt.Errorf("недопустимый формат файла: %s", mimeType)
	}

	return nil
}

// Хелперы
func isPossibleXml(mime string) bool {
	return mime == "text/plain; charset=utf-8" ||
		mime == "text/xml; charset=utf-8" ||
		mime == "application/octet-stream"
}

func isSvgSignature(buf []byte) bool {
	return len(buf) > 5 && (string(buf[:4]) == "<svg" || string(buf[:5]) == "<?xml")
}

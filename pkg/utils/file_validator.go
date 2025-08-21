package utils

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"slices"

	"request-system/config"
)

// ValidateFile проверяет файл на соответствие правилам из контекста.
func ValidateFile(fileHeader *multipart.FileHeader, file io.ReadSeeker, contextName string) error {
	rules, ok := config.UploadContexts[contextName]
	if !ok {
		return fmt.Errorf("неизвестный контекст загрузки: %s", contextName)
	}

	if rules.MaxSizeMB > 0 {
		maxSizeBytes := rules.MaxSizeMB * 1024 * 1024
		if fileHeader.Size > maxSizeBytes {
			return fmt.Errorf("размер файла (%d MB) превышает лимит в %d MB", fileHeader.Size/(1024*1024), rules.MaxSizeMB)
		}
	}

	buffer := make([]byte, 512)
	if _, err := file.Read(buffer); err != nil {
		return fmt.Errorf("не удалось прочитать файл для определения типа")
	}
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("не удалось сбросить указатель файла")
	}
	mimeType := http.DetectContentType(buffer)

	if !slices.Contains(rules.AllowedMimeTypes, mimeType) {
		return fmt.Errorf("недопустимый тип файла: %s", mimeType)
	}

	isImage := slices.Contains([]string{"image/jpeg", "image/png", "image/gif"}, mimeType)
	if isImage && (rules.MinWidth > 0 || rules.MaxWidth > 0 || rules.MinHeight > 0 || rules.MaxHeight > 0) {
		img, _, err := image.DecodeConfig(file)
		if err != nil {
			return fmt.Errorf("не удалось определить размеры изображения")
		}
		if _, err := file.Seek(0, 0); err != nil {
			return fmt.Errorf("не удалось сбросить указатель файла")
		}

		width, height := img.Width, img.Height

		if rules.MinWidth > 0 && width < rules.MinWidth {
			return fmt.Errorf("ширина изображения (%dpx) меньше минимально допустимой (%dpx)", width, rules.MinWidth)
		}
		if rules.MaxWidth > 0 && width > rules.MaxWidth {
			return fmt.Errorf("ширина изображения (%dpx) больше максимально допустимой (%dpx)", width, rules.MaxWidth)
		}
		if rules.MinHeight > 0 && height < rules.MinHeight {
			return fmt.Errorf("высота изображения (%dpx) меньше минимально допустимой (%dpx)", height, rules.MinHeight)
		}
		if rules.MaxHeight > 0 && height > rules.MaxHeight {
			return fmt.Errorf("высота изображения (%dpx) больше максимально допустимой (%dpx)", height, rules.MaxHeight)
		}
	}

	return nil
}

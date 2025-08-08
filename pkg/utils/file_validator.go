package utils

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"io"
	"mime/multipart"
	"request-system/config"
)

// ValidateFile использует UploadContexts для полной проверки файла.
func ValidateFile(fileHeader *multipart.FileHeader, file io.ReadSeeker, contextName string) error {
	uploadConfig, ok := config.UploadContexts[contextName]
	if !ok {
		return fmt.Errorf("неизвестный контекст загрузки: %s", contextName)
	}

	// Проверка MIME-типа
	contentType := fileHeader.Header.Get("Content-Type")
	isAllowedMimeType := false
	for _, allowedType := range uploadConfig.AllowedMimeTypes {
		if contentType == allowedType {
			isAllowedMimeType = true
			break
		}
	}
	if !isAllowedMimeType {
		return fmt.Errorf("недопустимый тип файла: %s", contentType)
	}

	// Проверка размера файла
	if fileHeader.Size > (uploadConfig.MaxSizeMB * 1024 * 1024) {
		return fmt.Errorf("файл слишком большой. Максимум: %d МБ", uploadConfig.MaxSizeMB)
	}

	// Проверка размеров изображения
	if uploadConfig.ExpectedWidth > 0 && uploadConfig.ExpectedHeight > 0 && contentType != "image/svg+xml" && contentType != "image/svg" {
		imgConfig, _, err := image.DecodeConfig(file)
		if err != nil {
			return fmt.Errorf("не удалось прочитать размеры растрового изображения: %w", err)
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("внутренняя ошибка при работе с файлом")
		}

		if imgConfig.Width != uploadConfig.ExpectedWidth || imgConfig.Height != uploadConfig.ExpectedHeight {
			return fmt.Errorf("неверный размер изображения. Ожидается: %dx%d, получено: %dx%d",
				uploadConfig.ExpectedWidth, uploadConfig.ExpectedHeight, imgConfig.Width, imgConfig.Height)
		}
	}

	return nil
}

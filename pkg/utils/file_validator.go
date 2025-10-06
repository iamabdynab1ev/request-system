package utils

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"slices"

	"request-system/config"
)

func ValidateFile(fileHeader *multipart.FileHeader, file io.ReadSeeker, contextName string) error {
	rules, ok := config.UploadContexts[contextName]
	if !ok {
		return fmt.Errorf("неизвестный контекст загрузки: %s", contextName)
	}

	if rules.MaxSizeMB > 0 {
		maxSizeBytes := rules.MaxSizeMB * 1024 * 1024
		if fileHeader.Size > maxSizeBytes {
			return fmt.Errorf("размер файла (%d KB) превышает лимит в %d MB", fileHeader.Size/1024, rules.MaxSizeMB)
		}
	}

	buffer := make([]byte, 512)
	_, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("не удалось прочитать файл для определения типа")
	}
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("не удалось сбросить указатель файла")
	}

	mimeType := http.DetectContentType(buffer)

	if mimeType == "text/plain; charset=utf-8" ||
		mimeType == "text/xml; charset=utf-8" ||
		mimeType == "application/octet-stream" {
		if string(buffer[:4]) == "<svg" || string(buffer[:5]) == "<?xml" {
			mimeType = "image/svg+xml"
		}
	}

	if !slices.Contains(rules.AllowedMimeTypes, mimeType) {
		return fmt.Errorf("недопустимый тип файла: %s", mimeType)
	}


	return nil 
}

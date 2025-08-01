package filestorage

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type FileStorageInterface interface {
	Save(fileHeader *multipart.FileHeader) (filePath string, err error)
}

type LocalFileStorage struct {
	basePath string
}

func NewLocalFileStorage(basePath string) (FileStorageInterface, error) {
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		if err := os.MkdirAll(basePath, 0755); err != nil {
			return nil, fmt.Errorf("не удалось создать директорию для хранения файлов: %w", err)
		}
	}
	return &LocalFileStorage{basePath: basePath}, nil
}

func (s *LocalFileStorage) Save(fileHeader *multipart.FileHeader) (string, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	ext := filepath.Ext(fileHeader.Filename)
	uniqueFileName := fmt.Sprintf("%s-%s%s", time.Now().Format("2006-01-02"), uuid.New().String(), ext)

	datePath := time.Now().Format("2006/01/02")
	fullDirPath := filepath.Join(s.basePath, datePath)
	if err = os.MkdirAll(fullDirPath, 0755); err != nil {
		return "", err
	}

	dst, err := os.Create(filepath.Join(fullDirPath, uniqueFileName))
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return "", err
	}

	return filepath.ToSlash(filepath.Join(datePath, uniqueFileName)), nil
}

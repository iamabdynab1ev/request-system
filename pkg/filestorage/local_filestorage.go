// pkg/filestorage/local_filestorage.go

package filestorage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Интерфейс теперь требует `prefix`
type FileStorageInterface interface {
	Save(file io.Reader, originalFileName string, prefix string) (filePath string, err error)
	Delete(filePath string) error
}

type LocalFileStorage struct {
	basePath string
}

func NewLocalFileStorage(basePath string) (FileStorageInterface, error) {
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		if err := os.MkdirAll(basePath, 0o755); err != nil {
			return nil, fmt.Errorf("не удалось создать директорию: %w", err)
		}
	}
	return &LocalFileStorage{basePath: basePath}, nil
}

func (s *LocalFileStorage) Save(file io.Reader, originalFileName string, prefix string) (string, error) {
	ext := filepath.Ext(originalFileName)
	uniqueFileName := fmt.Sprintf("%s-%s%s", time.Now().Format("2006-01-02"), uuid.New().String(), ext)

	datePath := time.Now().Format("2006/01/02")
	fullDirPath := filepath.Join(s.basePath, prefix, datePath)

	if err := os.MkdirAll(fullDirPath, 0o755); err != nil {
		return "", err
	}

	dst, err := os.Create(filepath.Join(fullDirPath, uniqueFileName))
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, file); err != nil {
		return "", err
	}

	return filepath.ToSlash(filepath.Join(prefix, datePath, uniqueFileName)), nil
}

func (s *LocalFileStorage) Delete(fileURL string) error {
	// fileURL приходит в виде "/uploads/prefix/2024/08/21/file.jpg"
	// Нам нужно отсечь "/uploads/" чтобы получить путь относительно s.basePath,
	// который и есть "uploads".
	relativePath := strings.TrimPrefix(fileURL, "/uploads/")

	// Собираем полный путь на диске: s.basePath ("uploads") + relativePath ("prefix/...")
	fullPath := filepath.Join(s.basePath, relativePath)

	// Если файла и так нет, считаем операцию успешной.
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil
	}

	// Удаляем файл.
	return os.Remove(fullPath)
}

package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// CreateLogger создает новый экземпляр zap.Logger для указанного модуля.
func CreateLogger(logLevel, moduleName string) (*zap.Logger, error) {
	// 1. Устанавливаем уровень логирования
	var level zapcore.Level
	if err := level.Set(logLevel); err != nil {
		level = zap.InfoLevel // Уровень по умолчанию, если передан неверный
	}

	// 2. Настраиваем кодировщик (формат вывода JSON)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.CallerKey = "caller"

	// 3. Создаем "ядра" для вывода
	// Ядро №1: для вывода в консоль
	consoleCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.Lock(os.Stdout),
		zap.NewAtomicLevelAt(level),
	)

	// Ядро №2: для вывода в файл
	logDir := "./logs"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		os.Mkdir(logDir, os.ModePerm)
	}

	logFilePath := filepath.Join(logDir, fmt.Sprintf("%s.log", moduleName))
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл логов %s: %w", logFilePath, err)
	}

	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(logFile),
		zap.NewAtomicLevelAt(level),
	)

	// 4. Объединяем два ядра в одно
	combinedCore := zapcore.NewTee(consoleCore, fileCore)

	// 5. Создаем логгер
	logger := zap.New(combinedCore, zap.AddCaller())

	return logger, nil
}

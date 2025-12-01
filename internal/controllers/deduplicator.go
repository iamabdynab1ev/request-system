// Файл: internal/controllers/deduplicator.go
package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RequestDeduplicator предотвращает обработку дублирующихся запросов
type RequestDeduplicator struct {
	processing sync.Map // key: string (chatID_command), value: time.Time
	timeout    time.Duration
}

func NewRequestDeduplicator(timeout time.Duration) *RequestDeduplicator {
	return &RequestDeduplicator{
		timeout: timeout,
	}
}

// TryAcquire пытается захватить "лок" для обработки запроса
// Возвращает true если можно обрабатывать, false если запрос уже обрабатывается
func (d *RequestDeduplicator) TryAcquire(chatID int64, command string) bool {
	key := fmt.Sprintf("%d_%s", chatID, command)

	// Проверяем, есть ли уже обработка этого запроса
	if val, exists := d.processing.Load(key); exists {
		lastTime := val.(time.Time)
		// Если прошло меньше timeout, запрос ещё обрабатывается
		if time.Since(lastTime) < d.timeout {
			return false
		}
	}

	// Записываем время начала обработки
	d.processing.Store(key, time.Now())
	return true
}

// Release освобождает "лок" после обработки
func (d *RequestDeduplicator) Release(chatID int64, command string) {
	key := fmt.Sprintf("%d_%s", chatID, command)
	d.processing.Delete(key)
}

// Cleanup периодически очищает старые записи (запускать в горутине)
func (d *RequestDeduplicator) Cleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			d.processing.Range(func(key, value interface{}) bool {
				lastTime := value.(time.Time)
				if now.Sub(lastTime) > d.timeout*2 {
					d.processing.Delete(key)
				}
				return true
			})
		}
	}
}

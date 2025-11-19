package eventbus

import (
	"context"
	"sync"
	"time" // Убедитесь, что этот импорт есть, он нужен для WithTimeout

	"go.uber.org/zap"
)

// Event представляет собой любое событие в системе.
type Event interface {
	Name() string
}

// Listener - это обработчик (слушатель) событий.
type Listener func(ctx context.Context, event Event) error

// Bus - это наша шина событий.
type Bus struct {
	listeners map[string][]Listener
	mu        sync.RWMutex // ИСПРАВЛЕНИЕ: RWMex -> RWMutex
	logger    *zap.Logger
}

// New создает новую шину событий.
func New(logger *zap.Logger) *Bus {
	return &Bus{
		listeners: make(map[string][]Listener),
		logger:    logger,
	}
}

// Subscribe подписывает слушателя на определенное событие.
func (b *Bus) Subscribe(eventName string, listener Listener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners[eventName] = append(b.listeners[eventName], listener)
}

// Publish публикует событие. Все подписчики будут вызваны.
func (b *Bus) Publish(ctx context.Context, event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	eventName := event.Name()
	if listeners, ok := b.listeners[eventName]; ok {
		for _, listener := range listeners {
			go func(l Listener) {
				// Создаем контекст с таймаутом, чтобы избежать "вечных" горутин.
				// Например, 1 минута на обработку события.
				ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
				defer cancel()

				// Логируем ошибки от слушателей, а не игнорируем их.
				if err := l(ctxWithTimeout, event); err != nil {
					b.logger.Error("Ошибка в обработчике события",
						zap.String("event", eventName),
						zap.Error(err),
					)
				}
			}(listener)
		}
	}
}

package telegram

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type dedupEntry struct {
	expiry time.Time
}

type RequestDeduplicator struct {
	mu    sync.Mutex
	locks map[string]dedupEntry
}

func NewRequestDeduplicator() *RequestDeduplicator {
	return &RequestDeduplicator{
		locks: make(map[string]dedupEntry),
	}
}

func (d *RequestDeduplicator) TryAcquire(chatID int64, keySuffix string, ttl time.Duration) bool {
	key := fmt.Sprintf("%d_%s", chatID, keySuffix)
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	if entry, exists := d.locks[key]; exists {
		if now.Before(entry.expiry) {
			return false
		}
	}

	d.locks[key] = dedupEntry{expiry: now.Add(ttl)}
	return true
}

func (d *RequestDeduplicator) Cleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			d.mu.Lock()
			for key, entry := range d.locks {
				if now.After(entry.expiry) {
					delete(d.locks, key)
				}
			}
			d.mu.Unlock()
		}
	}
}

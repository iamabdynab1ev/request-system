package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type RequestDeduplicator struct {
	locks sync.Map
}

func NewRequestDeduplicator() *RequestDeduplicator {
	return &RequestDeduplicator{}
}

func (d *RequestDeduplicator) TryAcquire(chatID int64, keySuffix string, ttl time.Duration) bool {
	key := fmt.Sprintf("%d_%s", chatID, keySuffix)
	now := time.Now()

	if val, exists := d.locks.Load(key); exists {
		expiry := val.(time.Time)
		if now.Before(expiry) {
			return false
		}
	}

	d.locks.Store(key, now.Add(ttl))
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
			d.locks.Range(func(key, value interface{}) bool {
				expiry := value.(time.Time)
				if now.After(expiry) {
					d.locks.Delete(key)
				}
				return true
			})
		}
	}
}

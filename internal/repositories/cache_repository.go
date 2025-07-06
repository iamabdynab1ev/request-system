package repositories

import (
	"context"
	"time"
)

type CacheRepositoryInterface interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, key ...string) error
	Incr(ctx context.Context, key string) (int64, error)
}

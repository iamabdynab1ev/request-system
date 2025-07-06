package repositories

import (
	"context"
	"fmt"
	apperrors "request-system/pkg/errors"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisCacheRepository struct {
	client *redis.Client
}

func NewRedisCacheRepository(client *redis.Client) CacheRepositoryInterface {
	return &RedisCacheRepository{
		client: client,
	}
}

func (r *RedisCacheRepository) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	err := r.client.Set(ctx, key, value, expiration).Err()
	if err != nil {
		return fmt.Errorf("ошибка установки значения в Redis: %w", err)
	}
	return nil
}

func (r *RedisCacheRepository) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", apperrors.ErrNotFound
	} else if err != nil {
		return "", fmt.Errorf("ошибка получения значения из Redis: %w", err)
	}
	return val, nil
}

func (r *RedisCacheRepository) Del(ctx context.Context, key ...string) error {
	err := r.client.Del(ctx, key...).Err()
	if err != nil {
		return fmt.Errorf("ошибка удаления ключа из Redis: %w", err)
	}
	return nil
}

func (r *RedisCacheRepository) Incr(ctx context.Context, key string) (int64, error) {
	val, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("ошибка инкремента значения в Redis: %w", err)
	}
	return val, nil
}

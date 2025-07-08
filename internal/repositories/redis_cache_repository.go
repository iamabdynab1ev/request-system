package repositories

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisCacheRepository - реализация кеша на Redis.
type RedisCacheRepository struct {
	client *redis.Client
}

// NewRedisCacheRepository - конструктор для репозитория.
// Он возвращает объект, который соответствует CacheRepositoryInterface.
func NewRedisCacheRepository(client *redis.Client) CacheRepositoryInterface {
	return &RedisCacheRepository{client: client}
}

// Get получает значение из кеша по ключу.
func (r *RedisCacheRepository) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// Set устанавливает значение в кеш.
func (r *RedisCacheRepository) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Del удаляет ключи из кеша.
func (r *RedisCacheRepository) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Incr атомарно увеличивает значение ключа на 1.
func (r *RedisCacheRepository) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// Expire устанавливает время жизни для ключа.
// ЭТОТ МЕТОД РЕШАЕТ ВАШУ ОШИБКУ КОМПИЛЯЦИИ.
func (r *RedisCacheRepository) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return r.client.Expire(ctx, key, expiration).Result()
}

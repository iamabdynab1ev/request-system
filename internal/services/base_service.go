package services

import (
	"context"
	"encoding/json"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type BaseService struct {
	redis  *redis.Client
	logger *zap.Logger
}

func NewBaseService(redis *redis.Client, logger *zap.Logger) *BaseService {
	return &BaseService{redis: redis, logger: logger}
}

// CheckPermission проверяет права доступа
func (s *BaseService) CheckPermission(ctx context.Context, permission string) (uint64, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		s.logger.Error("Пользователь не авторизован", zap.Error(err))
		return 0, apperrors.ErrUnauthorized
	}
	claims, err := utils.GetClaimsFromContext(ctx)
	if err != nil {
		s.logger.Error("Ошибка получения claims", zap.Error(err))
		return 0, apperrors.ErrUnauthorized
	}
	if !utils.HasPermission(claims, permission) {
		s.logger.Warn("Отказано в доступе", zap.Uint64("userID", userID))
		return userID, apperrors.ErrForbidden
	}
	return userID, nil
}

// CacheGet получает данные из кэша
func (s *BaseService) CacheGet(ctx context.Context, key string, dest interface{}) bool {
	cached, err := s.redis.Get(ctx, key).Result()
	if err == nil {
		if err := json.Unmarshal([]byte(cached), dest); err == nil {
			s.logger.Info("Данные получены из кэша", zap.String("key", key))
			return true
		}
	}
	return false
}

// CacheSet сохраняет данные в кэш
func (s *BaseService) CacheSet(ctx context.Context, key string, data interface{}, ttl time.Duration) {
	serialized, _ := json.Marshal(data)
	s.redis.SetEX(ctx, key, serialized, ttl)
}

// LogAudit записывает действие в аудит
func (s *BaseService) LogAudit(ctx context.Context, storage *pgxpool.Pool, userID uint64, action, entity string, entityID uint64, message string) {
	query := `
        INSERT INTO audit_log (user_id, action, entity, entity_id, message, created_at)
        VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := storage.Exec(ctx, query, userID, action, entity, entityID, message, time.Now())
	if err != nil {
		s.logger.Error("Ошибка записи в аудит", zap.Error(err))
	}
	s.logger.Info("Аудит действия",
		zap.Uint64("user_id", userID),
		zap.String("action", action),
		zap.String("entity", entity),
		zap.Uint64("entity_id", entityID),
		zap.String("message", message))
}

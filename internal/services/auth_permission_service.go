package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"

	"go.uber.org/zap"
)

type AuthPermissionServiceInterface interface {
	GetAllUserPermissions(ctx context.Context, userID uint64) ([]string, error)
	InvalidateUserPermissionsCache(ctx context.Context, userID uint64) error
}

type AuthPermissionService struct {
	permissionRepo repositories.PermissionRepositoryInterface
	cacheRepo      repositories.CacheRepositoryInterface
	logger         *zap.Logger
	cacheTTL       time.Duration
}

func NewAuthPermissionService(
	permissionRepo repositories.PermissionRepositoryInterface,
	cacheRepo repositories.CacheRepositoryInterface,
	logger *zap.Logger,
	cacheTTL time.Duration,
) AuthPermissionServiceInterface {
	return &AuthPermissionService{
		permissionRepo: permissionRepo,
		cacheRepo:      cacheRepo,
		logger:         logger,
		cacheTTL:       cacheTTL,
	}
}

func (s *AuthPermissionService) GetAllUserPermissions(ctx context.Context, userID uint64) ([]string, error) {
	cacheKey := fmt.Sprintf("auth:permissions:user:%d", userID)

	cachedData, err := s.cacheRepo.Get(ctx, cacheKey)
	if err == nil {
		var permissions []string
		if err := json.Unmarshal([]byte(cachedData), &permissions); err == nil {
			s.logger.Debug("Привилегии из кэша", zap.Uint64("userID", userID))
			return permissions, nil
		}
		s.logger.Warn("Не удалось распарсить привилегии из кэша", zap.Error(err), zap.String("key", cacheKey))
	}

	s.logger.Debug("Кэш не найден. Загружаем привилегии из БД", zap.Uint64("userID", userID))
	permissions, err := s.permissionRepo.GetAllUserPermissionsNames(ctx, userID)
	if err != nil {
		s.logger.Error("Не удалось получить привилегии из БД", zap.Uint64("userID", userID), zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}

	encoded, err := json.Marshal(permissions)
	if err != nil {
		s.logger.Error("Не удалось сериализовать привилегии", zap.Uint64("userID", userID), zap.Error(err))
	} else {
		if err := s.cacheRepo.Set(ctx, cacheKey, string(encoded), s.cacheTTL); err != nil {
			s.logger.Error("Не удалось сохранить привилегии в кэше", zap.Uint64("userID", userID), zap.Error(err))
		} else {
			s.logger.Debug("Привилегии успешно сохранены в кэше", zap.Uint64("userID", userID))
		}
	}

	return permissions, nil
}

func (s *AuthPermissionService) InvalidateUserPermissionsCache(ctx context.Context, userID uint64) error {
	cacheKey := fmt.Sprintf("auth:permissions:user:%d", userID)
	s.logger.Info("Попытка удаления кэша по ключу.", zap.String("cacheKey", cacheKey))
	if err := s.cacheRepo.Del(ctx, cacheKey); err != nil {
		s.logger.Error("Не удалось удалить кэш привилегий", zap.Uint64("userID", userID), zap.Error(err))
		return err
	}
	s.logger.Info("Кэш привилегий успешно удален", zap.Uint64("userID", userID))
	return nil
}

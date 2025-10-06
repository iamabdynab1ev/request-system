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
			s.logger.Debug("Permissions loaded from cache", zap.Uint64("userID", userID))
			return permissions, nil
		}
		s.logger.Warn("Failed to unmarshal cached permissions", zap.Error(err), zap.String("key", cacheKey))
	}

	s.logger.Debug("Cache miss. Loading permissions from DB", zap.Uint64("userID", userID))
	permissions, err := s.permissionRepo.GetAllUserPermissionsNames(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get permissions from DB", zap.Uint64("userID", userID), zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}

	encoded, err := json.Marshal(permissions)
	if err != nil {
		s.logger.Error("Failed to marshal permissions", zap.Uint64("userID", userID), zap.Error(err))
	} else {
		if err := s.cacheRepo.Set(ctx, cacheKey, string(encoded), s.cacheTTL); err != nil {
			s.logger.Error("Failed to cache permissions", zap.Uint64("userID", userID), zap.Error(err))
		} else {
			s.logger.Debug("Permissions cached successfully", zap.Uint64("userID", userID))
		}
	}

	return permissions, nil
}

func (s *AuthPermissionService) InvalidateUserPermissionsCache(ctx context.Context, userID uint64) error {
	cacheKey := fmt.Sprintf("auth:permissions:user:%d", userID)
	if err := s.cacheRepo.Del(ctx, cacheKey); err != nil {
		s.logger.Error("Failed to invalidate permission cache", zap.Uint64("userID", userID), zap.Error(err))
		return err
	}
	s.logger.Info("Permission cache invalidated successfully", zap.Uint64("userID", userID))
	return nil
}

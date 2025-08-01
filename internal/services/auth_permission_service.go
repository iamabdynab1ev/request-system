package services

import (
	"context"
	"encoding/json"
	"fmt"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"time"

	"go.uber.org/zap"
)

type AuthPermissionServiceInterface interface {
	GetRolePermissionsNames(ctx context.Context, roleID uint64) ([]string, error)
	InvalidateRolePermissionsCache(ctx context.Context, roleID uint64) error
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

func (s *AuthPermissionService) GetRolePermissionsNames(ctx context.Context, roleID uint64) ([]string, error) {
	cacheKey := fmt.Sprintf("auth:permissions:role:%d", roleID)
	var permissions []string

	// 1. Попытка получить данные из Redis-кеша
	cachedPermissionsJSON, errGet := s.cacheRepo.Get(ctx, cacheKey)
	if errGet == nil {
		// пытаемся распаковать из кеша
		if err := json.Unmarshal([]byte(cachedPermissionsJSON), &permissions); err == nil {
			s.logger.Debug("AuthPermissionService: Привилегии роли найдены в кеше", zap.Uint64("roleID", roleID))
			return permissions, nil
		} else {
			s.logger.Warn("AuthPermissionService: Ошибка при десериализации привилегий из кеша", zap.Error(err), zap.String("key", cacheKey), zap.Uint64("roleID", roleID))
		}
	} else {
		s.logger.Debug("AuthPermissionService: Привилегии роли не найдены в кеше, запрос к БД", zap.Uint64("roleID", roleID), zap.Error(errGet))
	}

	// 2. Если данные не были в кеше или были повреждены, получаем их из базы данных
	permissions, errDB := s.permissionRepo.GetPermissionsNamesByRoleID(ctx, roleID)
	if errDB != nil {
		s.logger.Error("AuthPermissionService: Не удалось получить привилегии для роли из БД", zap.Uint64("roleID", roleID), zap.Error(errDB))
		return nil, apperrors.ErrInternalServer
	}

	// 3. Кешируем полученные из БД данные обратно в Redis
	if len(permissions) > 0 {
		permissionsJSONBytes, errMarshal := json.Marshal(permissions)
		if errMarshal != nil {
			s.logger.Error("AuthPermissionService: Не удалось сериализовать привилегии для кеширования", zap.Uint64("roleID", roleID), zap.Error(errMarshal))
		} else {
			if errSet := s.cacheRepo.Set(ctx, cacheKey, string(permissionsJSONBytes), s.cacheTTL); errSet != nil {
				s.logger.Error("AuthPermissionService: Не удалось сохранить привилегии роли в кеш", zap.Uint64("roleID", roleID), zap.Error(errSet))
			} else {
				s.logger.Debug("AuthPermissionService: Привилегии роли успешно закешированы", zap.Uint64("roleID", roleID))
			}
		}
	}
	return permissions, nil
}

func (s *AuthPermissionService) InvalidateRolePermissionsCache(ctx context.Context, roleID uint64) error {
	cacheKey := fmt.Sprintf("auth:permissions:role:%d", roleID)
	if err := s.cacheRepo.Del(ctx, cacheKey); err != nil {
		s.logger.Error("AuthPermissionService: Ошибка инвалидации кеша привилегий для роли", zap.Uint64("roleID", roleID), zap.Error(err))
		return err
	}
	s.logger.Info("AuthPermissionService: Кеш привилегий для роли инвалидирован", zap.Uint64("roleID", roleID))
	return nil
}

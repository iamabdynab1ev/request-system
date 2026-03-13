package middleware

import (
	"context"
	"strings"

	apperrors "request-system/pkg/errors"
	"request-system/pkg/service"
	"request-system/pkg/utils"

	"request-system/internal/services"
	"request-system/pkg/contextkeys"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type AuthMiddleware struct {
	jwtService            service.JWTService
	authPermissionService services.AuthPermissionServiceInterface
	logger                *zap.Logger
}

func NewAuthMiddleware(jwtSvc service.JWTService, authPermissionSvc services.AuthPermissionServiceInterface, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{jwtService: jwtSvc, authPermissionService: authPermissionSvc, logger: logger}
}

func (m *AuthMiddleware) Auth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return utils.ErrorResponse(c, apperrors.ErrEmptyAuthHeader, m.logger)
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return utils.ErrorResponse(c, apperrors.ErrInvalidAuthHeader, m.logger)
		}
		tokenString := parts[1]

		claims, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			return m.handleAuthError(c, err)
		}

		if claims.IsRefreshToken {
			m.logger.Warn("Попытка доступа с refresh токеном", zap.Uint64("userID", claims.UserID))
			return utils.ErrorResponse(c, apperrors.ErrTokenIsNotAccess, m.logger)
		}

		permissions, err := m.authPermissionService.GetAllUserPermissions(c.Request().Context(), claims.UserID)
		if err != nil {
			m.logger.Error("Ошибка получения прав пользователя",
				zap.Uint64("userID", claims.UserID),
				zap.Error(err),
			)
			return utils.ErrorResponse(c, apperrors.ErrInternalServer, m.logger)
		}

		permissionsMap := make(map[string]bool)
		for _, p := range permissions {
			permissionsMap[p] = true
		}

		ctx := c.Request().Context()
		newCtx := context.WithValue(ctx, contextkeys.UserIDKey, claims.UserID)
		newCtx = context.WithValue(newCtx, contextkeys.UserRoleIDKey, claims.RoleID)
		newCtx = context.WithValue(newCtx, contextkeys.UserPermissionsKey, permissions)
		newCtx = context.WithValue(newCtx, contextkeys.UserPermissionsMapKey, permissionsMap)
		c.SetRequest(c.Request().WithContext(newCtx))

		return next(c)
	}
}

func (m *AuthMiddleware) handleAuthError(c echo.Context, err error) error {
	m.logger.Warn("Ошибка аутентификации", zap.Error(err))
	if !c.Response().Committed {
		return utils.ErrorResponse(c, err, m.logger)
	}
	return nil
}

func getUserPermissionsFromContext(ctx context.Context) ([]string, bool) {
	permissions, ok := ctx.Value(contextkeys.UserPermissionsKey).([]string)
	if !ok || permissions == nil {
		return []string{}, false
	}
	return permissions, true
}

func (m *AuthMiddleware) AuthorizeAny(requiredPermissions ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userPermissions, ok := getUserPermissionsFromContext(c.Request().Context())
			if !ok {
				m.logger.Error("Права не найдены в контексте (Middleware Auth не сработал?)")
				return utils.ErrorResponse(c, apperrors.ErrUnauthorized, m.logger)
			}
			for _, requiredPerm := range requiredPermissions {
				for _, userPerm := range userPermissions {
					if userPerm == requiredPerm {
						return next(c)
					}
				}
			}

			userID := c.Request().Context().Value(contextkeys.UserIDKey)
			m.logger.Warn("Доступ запрещен (AuthorizeAny)", 
				zap.Any("userID", userID), 
				zap.Strings("required", requiredPermissions),
				zap.Strings("actual", userPermissions),
			)
			return utils.ErrorResponse(c, apperrors.ErrForbidden, m.logger)
		}
	}
}

func (m *AuthMiddleware) AuthorizeAll(requiredPermissions ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userPermissions, ok := getUserPermissionsFromContext(c.Request().Context())
			if !ok {
				m.logger.Error("Права не найдены в контексте")
				return utils.ErrorResponse(c, apperrors.ErrUnauthorized, m.logger)
			}

			missingPermissions := make([]string, 0)
			for _, requiredPerm := range requiredPermissions {
				found := false
				for _, userPerm := range userPermissions {
					if userPerm == requiredPerm {
						found = true
						break
					}
				}
				if !found {
					missingPermissions = append(missingPermissions, requiredPerm)
				}
			}

			if len(missingPermissions) > 0 {
				userID := c.Request().Context().Value(contextkeys.UserIDKey)
				m.logger.Warn("Доступ запрещен (AuthorizeAll)", 
					zap.Any("userID", userID), 
					zap.Strings("missing", missingPermissions),
				)
				return utils.ErrorResponse(c, apperrors.ErrForbidden, m.logger)
			}

			return next(c)
		}
	}
}

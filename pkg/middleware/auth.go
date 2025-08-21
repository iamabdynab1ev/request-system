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
		m.logger.Debug("AuthMiddleware: Получен заголовок Authorization", zap.String("authHeader", authHeader))
		if authHeader == "" {
			return m.handleAuthError(c, apperrors.ErrEmptyAuthHeader)
		}

		parts := strings.Split(authHeader, " ")
		m.logger.Debug("AuthMiddleware: Разбитый заголовок на части", zap.Strings("parts", parts))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return m.handleAuthError(c, apperrors.ErrInvalidAuthHeader)
		}
		tokenString := parts[1]

		claims, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			return m.handleAuthError(c, err)
		}

		if claims.IsRefreshToken {
			m.logger.Warn("Попытка доступа с refresh токеном")
			return utils.ErrorResponse(c, apperrors.ErrTokenIsNotAccess)
		}

		permissions, err := m.authPermissionService.GetRolePermissionsNames(c.Request().Context(), claims.RoleID)
		if err != nil {
			m.logger.Error("Не удалось получить имена привилегий для роли пользователя", zap.Uint64("userID", claims.UserID), zap.Uint64("roleID", claims.RoleID), zap.Error(err))
			return utils.ErrorResponse(c, apperrors.ErrInternalServer)
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

		m.logger.Info("Пользователь успешно аутентифицирован и привилегии загружены", zap.Uint64("userID", claims.UserID), zap.Uint64("roleID", claims.RoleID), zap.Strings("permissions", permissions))
		return next(c)
	}
}

func (m *AuthMiddleware) handleAuthError(c echo.Context, err error) error {
	m.logger.Warn("Ошибка аутентификации", zap.Error(err))
	return utils.ErrorResponse(c, err)
}

func getUserPermissionsFromContext(ctx context.Context) ([]string, bool) {
	permissions, ok := ctx.Value(contextkeys.UserPermissionsKey).([]string)
	if !ok || permissions == nil {
		return []string{}, false
	}
	return permissions, true
}

func isSuperuser(permissions []string) bool {
	for _, perm := range permissions {
		if perm == "superuser" {
			return true
		}
	}
	return false
}

func (m *AuthMiddleware) AuthorizeAny(requiredPermissions ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userPermissions, ok := getUserPermissionsFromContext(c.Request().Context())
			if !ok {
				m.logger.Error("Привилегии пользователя не найдены в контексте запроса (возможно, AuthMiddleware не был запущен).")
				return utils.ErrorResponse(c, apperrors.ErrUnauthorized)
			}

			if isSuperuser(userPermissions) {
				m.logger.Debug("Доступ разрешен Superuser'у.", zap.Any("userID", c.Request().Context().Value(contextkeys.UserIDKey)))
				return next(c)
			}

			for _, requiredPerm := range requiredPermissions {
				for _, userPerm := range userPermissions {
					if userPerm == requiredPerm {
						m.logger.Debug("Пользователь имеет требуемую привилегию.", zap.Any("userID", c.Request().Context().Value(contextkeys.UserIDKey)), zap.String("required", requiredPerm))
						return next(c)
					}
				}
			}

			m.logger.Warn("Пользователь не имеет ни одной из необходимых привилегий.", zap.Any("userID", c.Request().Context().Value(contextkeys.UserIDKey)), zap.Strings("requiredPermissions", requiredPermissions), zap.Strings("userPermissions", userPermissions))
			return utils.ErrorResponse(c, apperrors.ErrForbidden)
		}
	}
}

func (m *AuthMiddleware) AuthorizeAll(requiredPermissions ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userPermissions, ok := getUserPermissionsFromContext(c.Request().Context())
			if !ok {
				m.logger.Error("Привилегии пользователя не найдены в контексте запроса (возможно, AuthMiddleware не был запущен).")
				return utils.ErrorResponse(c, apperrors.ErrUnauthorized)
			}

			if isSuperuser(userPermissions) {
				m.logger.Debug("Доступ разрешен Superuser'у.", zap.Any("userID", c.Request().Context().Value(contextkeys.UserIDKey)))
				return next(c)
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
				m.logger.Warn("Пользователь не имеет всех необходимых привилегий.", zap.Any("userID", c.Request().Context().Value(contextkeys.UserIDKey)), zap.Strings("missingPermissions", missingPermissions), zap.Strings("requiredPermissions", requiredPermissions), zap.Strings("userPermissions", userPermissions))
				return utils.ErrorResponse(c, apperrors.ErrForbidden)
			}

			m.logger.Debug("Пользователь имеет все требуемые привилегии.", zap.Any("userID", c.Request().Context().Value(contextkeys.UserIDKey)))
			return next(c)
		}
	}
}

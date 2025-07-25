package middleware

import (
	"context"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/service"
	"request-system/pkg/utils"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type AuthMiddleware struct {
	jwtService service.JWTService
	logger     *zap.Logger
}

func NewAuthMiddleware(jwtSvc service.JWTService, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtSvc,
		logger:     logger,
	}
}

func (m *AuthMiddleware) Auth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return m.handleAuthError(c, apperrors.ErrEmptyAuthHeader)
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return m.handleAuthError(c, apperrors.ErrInvalidAuthHeader)
		}
		tokenString := parts[1]

		claims, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			return m.handleAuthError(c, err)
		}

		if claims.IsRefreshToken {
			m.logger.Warn("AuthMiddleware: Попытка доступа с refresh токеном")
			return utils.ErrorResponse(c, apperrors.ErrTokenIsNotAccess)
		}

		ctx := c.Request().Context()
		newCtx := context.WithValue(ctx, contextkeys.UserIDKey, claims.UserID)
		c.SetRequest(c.Request().WithContext(newCtx))

		m.logger.Info("AuthMiddleware: Пользователь успешно аутентифицирован", zap.Uint64("userID", claims.UserID))
		return next(c)
	}
}

func (m *AuthMiddleware) handleAuthError(c echo.Context, err error) error {
	m.logger.Warn("AuthMiddleware: Ошибка аутентификации", zap.Error(err))
	return utils.ErrorResponse(c, err)
}

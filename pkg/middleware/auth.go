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

// Auth - это основная функция middleware.
func (m *AuthMiddleware) Auth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// 1. Извлекаем токен из заголовка
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			m.logger.Warn("AuthMiddleware: Пустой заголовок Authorization")
			return utils.ErrorResponse(c, apperrors.ErrEmptyAuthHeader)
		}

		// 2. Проверяем формат заголовка "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			m.logger.Warn("AuthMiddleware: Неверный формат заголовка Authorization")
			return utils.ErrorResponse(c, apperrors.ErrInvalidAuthHeader)
		}

		tokenString := parts[1]

		// 3. Валидируем токен
		claims, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			m.logger.Warn("AuthMiddleware: Ошибка валидации токена", zap.Error(err))
			return utils.ErrorResponse(c, err) // ErrorResponse сам определит нужный статус (401)
		}

		// 4. Убеждаемся, что это не refresh токен
		if claims.IsRefreshToken {
			m.logger.Warn("AuthMiddleware: Попытка доступа с refresh токеном")
			return utils.ErrorResponse(c, apperrors.ErrTokenIsNotAccess)
		}

		// 5. Записываем UserID в контекст запроса для дальнейшего использования
		ctx := c.Request().Context()
		newCtx := context.WithValue(ctx, contextkeys.UserIDKey, claims.UserID)
		c.SetRequest(c.Request().WithContext(newCtx))

		m.logger.Info("AuthMiddleware: Пользователь успешно аутентифицирован", zap.Int("userID", claims.UserID))

		// 6. Если все в порядке, передаем управление следующему обработчику
		return next(c)
	}
}

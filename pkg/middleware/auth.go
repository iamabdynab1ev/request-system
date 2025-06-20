package middleware

import (
	"context"
	"log"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/service"
	"strings"

	"github.com/labstack/echo/v4"
)

type AuthMiddleware struct {
	jwt service.JWTService
}

func NewAuthMiddleware(jwtSvc service.JWTService) *AuthMiddleware {
	log.Printf("[NewAuthMiddleware] INFO: Экземпляр JWTService получен.")
	if jwtSvc == nil {
		log.Fatal("[NewAuthMiddleware] FATAL: Экземпляр JWTService не может быть nil!")
	}
	return &AuthMiddleware{
		jwt: jwtSvc,
	}
}

func (m *AuthMiddleware) Auth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		log.Println("[AuthMiddleware.Auth] INFO: Получен запрос.")

		claims, err := m.extractTokenClaims(c)
		if err != nil {
			log.Printf("[AuthMiddleware.Auth] ERROR: Ошибка при извлечении claims из токена: %v (Тип ошибки: %T)", err, err)
			return c.JSON(401, map[string]string{"error": err.Error()})
		}

		if claims.UserID == 0 {
			log.Printf("[AuthMiddleware.Auth] WARN: Полученный UserID в claims равен нулю, что неожиданно. Claims: %+v", *claims)
			return c.JSON(401, map[string]string{"error": apperrors.ErrInvalidToken.Error()})
		}

		log.Printf("[AuthMiddleware.Auth] INFO: Claims успешно получены, UserID: %d (Тип: %T)", claims.UserID, claims.UserID)

		ctx := c.Request().Context()
		newCtx := context.WithValue(ctx, contextkeys.UserIDKey, claims.UserID)
		c.SetRequest(c.Request().WithContext(newCtx))

		log.Println("[AuthMiddleware.Auth] INFO: UserID был записан в контекст под ключом 'UserID'.")

		return next(c)
	}
}

func (m *AuthMiddleware) extractTokenClaims(c echo.Context) (*service.JwtCustomClaim, error) {
	authHeader := c.Request().Header.Get("Authorization")
	log.Printf("[AuthMiddleware.extractTokenClaims] INFO: Заголовок Authorization: '%s'", authHeader)

	if authHeader == "" {
		log.Println("[AuthMiddleware.extractTokenClaims] WARN: Заголовок Authorization пустой.")
		return nil, apperrors.ErrEmptyAuthHeader
	}

	parts := strings.Split(authHeader, " ")
	log.Printf("[AuthMiddleware.extractTokenClaims] DEBUG: Части заголовка: %v (количество: %d)", parts, len(parts))

	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		log.Printf("[AuthMiddleware.extractTokenClaims] WARN: Неверный формат заголовка Authorization. parts[0]='%s'", parts[0])
		return nil, apperrors.ErrInvalidAuthHeader
	}

	tokenString := parts[1]
	if tokenString == "" {
		log.Println("[AuthMiddleware.extractTokenClaims] WARN: Строка токена пустая.")
		return nil, apperrors.ErrTokenNotFound
	}
	log.Printf("[AuthMiddleware.extractTokenClaims] INFO: Получен токен (строка): %s", tokenString)

	claims, err := m.jwt.ValidateToken(tokenString)
	if err != nil {
		log.Printf("[AuthMiddleware.extractTokenClaims] ERROR: m.jwt.ValidateToken вернул ошибку: %v (Тип ошибки: %T)", err, err)
		return nil, err
	}

	if claims.IsRefreshToken {
		log.Println("[AuthMiddleware.extractTokenClaims] WARN: Предоставленный токен является refresh токеном, но ожидался access токен.")
		return nil, apperrors.ErrTokenIsNotRefresh
	}

	log.Printf("[AuthMiddleware.extractTokenClaims] INFO: Токен успешно проверен, получен access токен claims: UserID: %d", claims.UserID)
	return claims, nil
}

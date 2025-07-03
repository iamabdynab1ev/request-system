package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"request-system/internal/repositories"
	"request-system/pkg/contextkeys"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/service"

	"github.com/labstack/echo/v4"
)

type AuthMiddleware struct {
	jwt      service.JWTService
	userRepo repositories.UserRepositoryInterface
}

func NewAuthMiddleware(jwtSvc service.JWTService, userRepo repositories.UserRepositoryInterface) *AuthMiddleware {
	log.Printf("[NewAuthMiddleware] INFO: Экземпляр JWTService получен.")
	if jwtSvc == nil {
		log.Fatal("[NewAuthMiddleware] FATAL: Экземпляр JWTService не может быть nil!")
	}
	if userRepo == nil {
		log.Fatal("[NewAuthMiddleware] FATAL: Экземпляр UserRepository не может быть nil!")
	}
	return &AuthMiddleware{
		jwt:      jwtSvc,
		userRepo: userRepo,
	}
}

func (m *AuthMiddleware) Auth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {

		log.Println("[AuthMiddleware.Auth] INFO: Получен запрос.")

		claims, err := m.extractTokenClaims(c)
		if err != nil {
			log.Printf("[AuthMiddleware.Auth] ERROR: Ошибка при извлечении claims из токена: %v (Тип ошибки: %T)", err, err)
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
		}

		if claims.UserID == 0 {
			log.Printf("[AuthMiddleware.Auth] WARN: Полученный UserID в claims равен нулю, что неожиданно. Claims: %+v", *claims)
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": apperrors.ErrInvalidToken.Error()})
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

func (m *AuthMiddleware) CheckPermission(requiredPermission string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := c.Request().Context().Value(contextkeys.UserIDKey).(int)
			if !ok || userID == 0 {
				log.Println("[CheckPermission] WARN: UserID не найден в контексте.")
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Невалидный контекст пользователя"})
			}

			log.Printf("[CheckPermission] INFO: Проверка права '%s' для UserID: %d", requiredPermission, userID)

			hasPermission, err := m.userRepo.UserHasPermission(c.Request().Context(), userID, requiredPermission)
			if err != nil {
				log.Printf("[CheckPermission] FATAL: Ошибка БД при проверке прав: %v", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Ошибка сервера при проверке доступа"})
			}

			if !hasPermission {
				log.Printf("[CheckPermission] FORBIDDEN: UserID: %d не имеет права '%s'", userID, requiredPermission)
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "Доступ запрещен. Требуется разрешение: " + requiredPermission,
				})
			}

			log.Printf("[CheckPermission] OK: UserID: %d имеет право '%s'. Пропускаем.", userID, requiredPermission)
			return next(c)
		}
	}
}

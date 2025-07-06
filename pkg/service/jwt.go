// Файл: pkg/service/jwt.go (или как он у вас называется)
// КОНЕЧНАЯ, ИСПРАВЛЕННАЯ ВЕРСИЯ

package service

import (
	"errors"                              // Убедитесь, что этот стандартный пакет импортирован
	apperrors "request-system/pkg/errors" // Используем ваш пакет ошибок
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/labstack/gommon/log"
)

type JwtCustomClaim struct {
	UserID         int `json:"userId"`
	IsRefreshToken bool
	jwt.RegisteredClaims
}

type JWTService interface {
	GenerateTokens(userId int) (string, string, error)
	ValidateToken(tokenString string) (*JwtCustomClaim, error)
	GetAccessTokenTTL() time.Duration
	GetRefreshTokenTTL() time.Duration
}

type jwtService struct {
	SecretKey       string
	AccessTokenExp  time.Duration
	RefreshTokenExp time.Duration
}

func NewJWTService(secretKey string, accessTokenExp, refreshTokenExp time.Duration) JWTService {
	return &jwtService{
		SecretKey:       secretKey,
		AccessTokenExp:  accessTokenExp,
		RefreshTokenExp: refreshTokenExp,
	}
}

// ИЗМЕНЕНО: Генерируем время в UTC
func (service *jwtService) GenerateTokens(userId int) (string, string, error) {
	// Используем .UTC(), чтобы гарантировать единый часовой пояс
	accessTokenExp := time.Now().UTC().Add(service.AccessTokenExp)
	refreshTokenExp := time.Now().UTC().Add(service.RefreshTokenExp)

	
	issuedAt := time.Now().UTC()

	accessTokenClaims := &JwtCustomClaim{
		UserID:         userId,
		IsRefreshToken: false,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessTokenExp),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
		},
	}

	refreshTokenClaims := &JwtCustomClaim{
		UserID:         userId,
		IsRefreshToken: true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshTokenExp),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS512, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString([]byte(service.SecretKey))
	if err != nil {
		return "", "", err
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS512, refreshTokenClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(service.SecretKey))
	if err != nil {
		return "", "", err
	}

	return accessTokenString, refreshTokenString, nil
}

// ИЗМЕНЕНО: Упрощаем валидацию и доверяем библиотеке
func (service *jwtService) ValidateToken(tokenString string) (*JwtCustomClaim, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JwtCustomClaim{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			// Используем вашу ошибку
			return nil, apperrors.ErrInvalidSigningMethod
		}
		return []byte(service.SecretKey), nil
	})

	// Библиотека jwt/v5 сама возвращает ошибку, если токен просрочен.
	// Нам нужно ее просто перехватить.
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			log.Warn("Проверка токена: срок действия истек")
			return nil, apperrors.ErrTokenExpired
		}
		log.Errorf("Ошибка парсинга токена: %v", err)
		// Используем общую ошибку для всех остальных проблем
		return nil, apperrors.ErrInvalidToken
	}

	// Если ошибок нет, проверяем, что claims удалось извлечь и токен валиден
	if claims, ok := token.Claims.(*JwtCustomClaim); ok && token.Valid {
		log.Debugf("Успешно извлечены claims: %+v", claims)
		return claims, nil
	}

	log.Warn("Токен невалиден по неизвестной причине")
	return nil, apperrors.ErrInvalidToken
}

func (s *jwtService) GetAccessTokenTTL() time.Duration {
	return s.AccessTokenExp
}

func (s *jwtService) GetRefreshTokenTTL() time.Duration {
	return s.RefreshTokenExp
}

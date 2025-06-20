package service

import (
	"request-system/pkg/errors"
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
	GetAccessTokenTTL() time.Duration  // <<--- ЯНГИ МЕТОД
	GetRefreshTokenTTL() time.Duration // <<--- ЯНГИ МЕТОД
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

func (service *jwtService) GenerateTokens(userId int) (string, string, error) {
	accessTokenExp := time.Now().Add(service.AccessTokenExp)
	refreshTokenExp := time.Now().Add(service.RefreshTokenExp)

	accessTokenClaims := &JwtCustomClaim{
		UserID:         userId,
		IsRefreshToken: false,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessTokenExp),
		},
	}

	refreshTokenClaims := &JwtCustomClaim{
		UserID:         userId,
		IsRefreshToken: true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshTokenExp),
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
func (s *jwtService) GetAccessTokenTTL() time.Duration {
	return s.AccessTokenExp
}

func (s *jwtService) GetRefreshTokenTTL() time.Duration {
	return s.RefreshTokenExp
}

// --- МЕТОДЛАР ҚЎШИЛДИ/ТЎҒРИЛАНДИ ---

func (service *jwtService) ValidateToken(tokenString string) (*JwtCustomClaim, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JwtCustomClaim{}, func(token *jwt.Token) (interface{}, error) {
		switch token.Method.(type) {
		case *jwt.SigningMethodHMAC:
			return []byte(service.SecretKey), nil
		default:
			return nil, errors.ErrInvalidSigningMethod
		}
	})

	if err != nil {
		log.Errorf("Ошибка парсинга или проверки подписи токена: %v", err)
		return nil, err
	}

	claims, ok := token.Claims.(*JwtCustomClaim)
	if !ok || !token.Valid {
		log.Warn("Токен невалиден или не удалось извлечь claims")
		return nil, errors.ErrInvalidToken
	}

	log.Debugf("Успешно извлечены claims: %+v", claims)

	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, errors.ErrTokenExpired
	}

	if claims.IssuedAt != nil && claims.IssuedAt.Time.After(time.Now()) {
		return nil, errors.ErrTokenNotYetValid
	}

	return claims, nil
}

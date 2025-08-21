package service

import (
	"errors"
	"time"

	apperrors "request-system/pkg/errors"

	jwt "github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type JwtCustomClaim struct {
	UserID         uint64 `json:"userID"`
	RoleID         uint64 `json:"roleID"`
	IsRefreshToken bool
	jwt.RegisteredClaims
}

type JWTService interface {
	GenerateTokens(userID uint64, roleID uint64) (string, string, error)
	ValidateToken(tokenString string) (*JwtCustomClaim, error)
	ValidateRefreshToken(tokenString string) (uint64, error)
	GetAccessTokenTTL() time.Duration
	GetRefreshTokenTTL() time.Duration
}

type jwtService struct {
	SecretKey       string
	AccessTokenExp  time.Duration
	RefreshTokenExp time.Duration
	logger          *zap.Logger
}

func NewJWTService(secretKey string, accessTokenExp, refreshTokenExp time.Duration, logger *zap.Logger) JWTService {
	return &jwtService{
		SecretKey:       secretKey,
		AccessTokenExp:  accessTokenExp,
		RefreshTokenExp: refreshTokenExp,
		logger:          logger,
	}
}

func (s *jwtService) GenerateTokens(userID uint64, roleID uint64) (string, string, error) {
	accessTokenExp := time.Now().UTC().Add(s.AccessTokenExp)
	refreshTokenExp := time.Now().UTC().Add(s.RefreshTokenExp)
	issuedAt := time.Now().UTC()

	accessTokenClaims := &JwtCustomClaim{
		UserID:         userID,
		RoleID:         roleID,
		IsRefreshToken: false,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessTokenExp),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
		},
	}

	refreshTokenClaims := &JwtCustomClaim{
		UserID:         userID,
		RoleID:         roleID,
		IsRefreshToken: true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshTokenExp),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS512, accessTokenClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.SecretKey))
	if err != nil {
		return "", "", err
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS512, refreshTokenClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(s.SecretKey))
	if err != nil {
		return "", "", err
	}

	return accessTokenString, refreshTokenString, nil
}

func (s *jwtService) ValidateToken(tokenString string) (*JwtCustomClaim, error) {
	s.logger.Debug("ValidateToken: Получена строка токена для валидации", zap.String("receivedToken", tokenString))
	token, err := jwt.ParseWithClaims(tokenString, &JwtCustomClaim{}, func(token *jwt.Token) (interface{}, error) {
		s.logger.Debug("ValidateToken: Получена строка токена для валидации", zap.String("receivedToken", tokenString))
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, apperrors.ErrInvalidSigningMethod
		}
		return []byte(s.SecretKey), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			s.logger.Warn("Проверка токена: срок действия истек")
			return nil, apperrors.ErrTokenExpired
		}
		s.logger.Error("Ошибка парсинга токена", zap.Error(err))
		return nil, apperrors.ErrInvalidToken
	}

	if claims, ok := token.Claims.(*JwtCustomClaim); ok && token.Valid {
		if claims.UserID <= 0 {
			s.logger.Warn("Недопустимый UserID в токене")
			return nil, apperrors.ErrInvalidToken
		}
		s.logger.Debug("Успешно извлечены claims", zap.Any("claims", claims))
		return claims, nil
	}

	s.logger.Warn("Токен невалиден по неизвестной причине")
	return nil, apperrors.ErrInvalidToken
}

func (s *jwtService) ValidateRefreshToken(tokenString string) (uint64, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return 0, err
	}
	if !claims.IsRefreshToken {
		s.logger.Warn("Попытка использовать access токен для обновления", zap.Uint64("userID", claims.UserID))
		return 0, apperrors.ErrInvalidToken
	}
	return claims.UserID, nil
}

func (s *jwtService) GetAccessTokenTTL() time.Duration {
	return s.AccessTokenExp
}

func (s *jwtService) GetRefreshTokenTTL() time.Duration {
	return s.RefreshTokenExp
}

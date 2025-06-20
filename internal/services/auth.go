package services

import (
	"context"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/errors"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo repositories.UserRepositoryInterface
	logger   *zap.Logger
}

func NewAuthService(
	userRepo repositories.UserRepositoryInterface,
	logger *zap.Logger,
) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		logger:   logger,
	}
}

func (a *AuthService) ValidateToken(token string) bool {
	return true
}

func (a *AuthService) Login(ctx context.Context, payload dto.LoginDto) (*entities.User, error) {
	user, err := a.userRepo.FindUserByEmail(ctx, payload.Email)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(payload.Password)); err != nil {
		return nil, errors.ErrInvalidCredentials
	}

	return user, nil
}

func (a *AuthService) Logout(token string) bool {
	return true
}


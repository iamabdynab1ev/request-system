package services

import "request-system/internal/repositories"

type AuthService struct {
	userRepo *repositories.UserRepository
}

func NewAuthService() *AuthService {
	return &AuthService{}
}

func (a *AuthService) ValidateToken(token string) bool {
	return true
}


func (a *AuthService) login(token string) bool {
	return true
}

func (a *AuthService) logout(token string) bool {
	return true
}

func (a *AuthService) me(token string) bool {
	return true
}


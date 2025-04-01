package services

import (
	"context"

	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
)

type UserService struct {
	userRepo *repositories.UserRepository
}

func NewUserService(userRepo *repositories.UserRepository) *UserService {
	return &UserService{userRepo: userRepo}
}

func (s *UserService) GetUsers(ctx context.Context, limit, offset uint64) ([]entities.User, error) {
	return nil, nil
}

func (s *UserService) FindUser(ctx context.Context, id uint64) (*entities.User, error) {
	return nil, nil
}

func (s *UserService) CreateUser(ctx context.Context, payload dto.CreateUserDTO) error {
	return nil
}

func (s *UserService) UpdateUser(ctx context.Context, id uint64, payload dto.UpdateUserDTO) error {
	return nil
}

func (s *UserService) DeleteUser(ctx context.Context, id uint64) error {
	return nil
}


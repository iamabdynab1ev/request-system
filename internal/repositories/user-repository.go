package repositories

import (
    "context"
	"request-system/internal/dto"
	"request-system/internal/entities"
)

type UserRepositoryInterface interface {
	GetUsers(ctx context.Context, limit uint64, offset uint64) ([]entities.User, error)
	FindUser(ctx context.Context, id uint64) (*entities.User, error)
	CreateUser(ctx context.Context, payload dto.CreateUserDTO) error
	UpdateUser(ctx context.Context, id uint64, payload dto.UpdateUserDTO) error
	DeleteUser(ctx context.Context, id uint64) error
}

type UserRepository struct {}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (r *UserRepository) GetUsers(ctx context.Context, limit uint64, offset uint64) ([]entities.User, error) {
	return []entities.User{}, nil
}

func (r *UserRepository) FindUsertx (ctx context.Context, id uint64) (*entities.User, error) {
	return nil, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, payload dto.CreateUserDTO) error {
	return nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, id uint64, payload dto.UpdateUserDTO) error {
	return nil
}

func (r *UserRepository) DeleteUser(ctx context.Context, id uint64) error {
	return nil
}
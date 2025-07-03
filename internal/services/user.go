package services

import (
	"context"
	"database/sql"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/utils"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepository repositories.UserRepositoryInterface
}

func NewUserService(userRepository repositories.UserRepositoryInterface) *UserService {
	return &UserService{
		userRepository: userRepository,
	}
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

func toInt64Pointer(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	return &ni.Int64
}

func userEntityToDTO(entity *entities.User) *dto.UserDTO {
	if entity == nil {
		return nil
	}

	return &dto.UserDTO{
		ID:          entity.ID,
		FIO:         entity.FIO,
		Email:       entity.Email,
		PhoneNumber: entity.PhoneNumber,
		Position:    entity.Position.String,

		Status: dto.ShortStatusDTO{
			ID:   entity.StatusID,
			Name: entity.StatusName.String,
		},
		Role: dto.ShortRoleDTO{
			ID:   entity.RoleID,
			Name: entity.RoleName.String,
		},
		Department: dto.ShortDepartmentDTO{
			ID:   entity.DepartmentID,
			Name: entity.DepartmentName.String,
		},

		BranchID: toInt64Pointer(entity.BranchID),
		OfficeID: toInt64Pointer(entity.OfficeID),
		OtdelID:  toInt64Pointer(entity.OtdelID),

		CreatedAt: entity.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: entity.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func userEntitiesToDTOs(entities []*entities.User) []dto.UserDTO {
	if len(entities) == 0 {
		return []dto.UserDTO{}
	}
	dtos := make([]dto.UserDTO, 0, len(entities))
	for _, entity := range entities {
		if entity != nil {
			dtos = append(dtos, *userEntityToDTO(entity))
		}
	}
	return dtos
}

func (s *UserService) GetUsers(ctx context.Context, limit uint64, offset uint64) ([]dto.UserDTO, error) {
	users, err := s.userRepository.GetUsers(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	return userEntitiesToDTOs(users), nil
}

func (s *UserService) FindUser(ctx context.Context, id uint64) (*dto.UserDTO, error) {
	user, err := s.userRepository.FindUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return userEntityToDTO(user), nil
}

func toSqlNullInt(p *int) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*p), Valid: true}
}

func (s *UserService) CreateUser(ctx context.Context, payload dto.CreateUserDTO) (*dto.UserDTO, error) {
	hashedPassword, err := hashPassword(payload.Password)
	if err != nil {
		return nil, err
	}

	userEntity := &entities.User{
		FIO:          payload.Fio,
		Email:        payload.Email,
		PhoneNumber:  payload.PhoneNumber,
		Password:     hashedPassword,
		StatusID:     payload.StatusID,
		RoleID:       payload.RoleID,
		DepartmentID: payload.DepartmentID,

		Position: sql.NullString{String: payload.Position, Valid: payload.Position != ""},
		BranchID: toSqlNullInt(payload.BranchID),
		OfficeID: toSqlNullInt(payload.OfficeID),
		OtdelID:  toSqlNullInt(payload.OtdelID),
	}

	createdEntity, err := s.userRepository.CreateUser(ctx, userEntity)
	if err != nil {
		return nil, err
	}
	return s.FindUser(ctx, createdEntity.ID)
}
func (s *UserService) UpdateUser(ctx context.Context, userID uint64, payload dto.UpdateUserDTO) (*dto.UserDTO, error) {
	if payload.Fio == nil && payload.Email == nil && payload.PhoneNumber == nil && payload.Password == nil && payload.Position == nil &&
		payload.StatusID == nil && payload.DepartmentID == nil && payload.BranchID == nil && payload.OfficeID == nil && payload.OtdelID == nil {
		return nil, fmt.Errorf("пустой запрос на обновление: %w", utils.ErrorBadRequest)
	}
	if payload.Password != nil && *payload.Password != "" {
		hashedPassword, err := hashPassword(*payload.Password)
		if err != nil {
			return nil, err
		}
		payload.Password = &hashedPassword
	}
	_, err := s.userRepository.UpdateUser(ctx, userID, payload)
	if err != nil {
		return nil, err
	}
	return s.FindUser(ctx, userID)
}
func (s *UserService) DeleteUser(ctx context.Context, id uint64) error {
	return s.userRepository.DeleteUser(ctx, id)
}

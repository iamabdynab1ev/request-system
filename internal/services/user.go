package services

import (
	"context"
	"fmt"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5/pgconn"
)

type UserService struct {
	userRepository   repositories.UserRepositoryInterface
	statusRepository repositories.StatusRepositoryInterface
}

func NewUserService(
	userRepository repositories.UserRepositoryInterface,
	statusRepository repositories.StatusRepositoryInterface,
) *UserService {
	return &UserService{
		userRepository:   userRepository,
		statusRepository: statusRepository,
	}
}

func statusDTOToShortStatusDTO(status *dto.StatusDTO) *dto.ShortStatusDTO {
	if status == nil {
		return nil
	}
	return &dto.ShortStatusDTO{
		ID:   status.ID,
		Name: status.Name,
	}
}

func userEntityToDTO(entity *entities.User, status *dto.ShortStatusDTO) *dto.UserDTO {
	if entity == nil {
		return nil
	}

	dto := &dto.UserDTO{
		ID:          entity.ID,
		Fio:         entity.FIO,
		Email:       entity.Email,
		Position:    entity.Position,
		PhoneNumber: entity.PhoneNumber,

		Role:       entity.RoleID,
		Branch:     entity.BranchID,
		Department: entity.DepartmentID,
		Office:     entity.OfficeID,
		Otdel:      entity.OtdelID,

		Status: *status,

		CreatedAt: entity.CreatedAt.Format("2006-01-02, 15:04:05"),
		UpdatedAt: entity.UpdatedAt.Format("2006-01-02, 15:04:05"),
	}

	return dto
}

func userEntitiesToDTOs(entities []entities.User, statusRepo repositories.StatusRepositoryInterface) ([]dto.UserDTO, error) {
	if entities == nil {
		return nil, nil
	}
	dtos := make([]dto.UserDTO, len(entities))
	for i, entity := range entities {
		statusDTO, err := statusRepo.FindStatus(context.Background(), uint64(entity.StatusID))
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve status for user %d: %w", entity.ID, err)
		}

		shortStatusDTO := statusDTOToShortStatusDTO(statusDTO)
		dtos[i] = *userEntityToDTO(&entity, shortStatusDTO)
	}
	return dtos, nil
}

func (service *UserService) GetUsers(ctx context.Context, limit uint64, offset uint64) ([]dto.UserDTO, error) {
	users, err := service.userRepository.GetUsers(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	return userEntitiesToDTOs(users, service.statusRepository)
}

func (service *UserService) FindUser(ctx context.Context, id uint64) (*dto.UserDTO, error) {
	user, err := service.userRepository.FindUser(ctx, id)
	if err != nil {
		return nil, err
	}

	statusDTO, err := service.statusRepository.FindStatus(ctx, uint64(user.StatusID))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve status for user %d: %w", user.ID, err)
	}

	shortStatusDTO := statusDTOToShortStatusDTO(statusDTO)

	return userEntityToDTO(user, shortStatusDTO), nil
}

func (service *UserService) CreateUser(ctx context.Context, payload dto.CreateUserDTO) (*dto.UserDTO, error) {
	hashedPassword, err := utils.HashPassword(payload.Password)
	if err != nil {
		return nil, err
	}

	userEntity := &entities.User{
		FIO:          payload.Fio,
		Email:        payload.Email,
		PhoneNumber:  payload.PhoneNumber,
		Password:     hashedPassword,
		Position:     payload.Position,
		StatusID:     payload.StatusID,
		RoleID:       payload.RoleID,
		BranchID:     payload.BranchID,
		DepartmentID: payload.DepartmentID,
		OfficeID:     payload.OfficeID,
		OtdelID:      payload.OtdelID,
	}

	createdEntity, err := service.userRepository.CreateUser(ctx, userEntity)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			fmt.Printf("Postgres error code: %s, message: %s, constraint: %s\n", pgErr.Code, pgErr.Message, pgErr.ConstraintName)
		}
		return nil, err
	}

	statusDTO, err := service.statusRepository.FindStatus(ctx, uint64(createdEntity.StatusID))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve status for created user %d: %w", createdEntity.ID, err)
	}

	shortStatusDTO := statusDTOToShortStatusDTO(statusDTO)

	return userEntityToDTO(createdEntity, shortStatusDTO), nil
}

func (service *UserService) UpdateUser(ctx context.Context, payload dto.UpdateUserDTO) (*dto.UserDTO, error) {
	existingUser, err := service.userRepository.FindUser(ctx, uint64(payload.ID))
	if err != nil {
		return nil, err
	}

	if payload.Fio != "" {
		existingUser.FIO = payload.Fio
	}
	if payload.Email != "" {
		existingUser.Email = payload.Email
	}
	if payload.PhoneNumber != "" {
		existingUser.PhoneNumber = payload.PhoneNumber
	}
	if payload.Position != "" {
		existingUser.Position = payload.Position
	}

	if payload.Password != "" {
		hashedPassword, err := utils.HashPassword(payload.Password)
		if err != nil {
			return nil, err
		}
		existingUser.Password = hashedPassword
	}

	if payload.StatusID != 0 {
		existingUser.StatusID = payload.StatusID
	}
	if payload.RoleID != 0 {
		existingUser.RoleID = payload.RoleID
	}
	if payload.BranchID != 0 {
		existingUser.BranchID = payload.BranchID
	}
	if payload.DepartmentID != 0 {
		existingUser.DepartmentID = payload.DepartmentID
	}
	if payload.OfficeID != nil {
		existingUser.OfficeID = payload.OfficeID
	}
	if payload.OtdelID != nil {
		existingUser.OtdelID = payload.OtdelID
	}

	existingUser.OfficeID = payload.OfficeID
	existingUser.OtdelID = payload.OtdelID

	updatedEntity, err := service.userRepository.UpdateUser(ctx, existingUser)
	if err != nil {
		return nil, err
	}

	statusDTO, err := service.statusRepository.FindStatus(ctx, uint64(updatedEntity.StatusID))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve status for updated user %d: %w", updatedEntity.ID, err)
	}

	shortStatusDTO := statusDTOToShortStatusDTO(statusDTO)

	return userEntityToDTO(updatedEntity, shortStatusDTO), nil
}

func (service *UserService) DeleteUser(ctx context.Context, id uint64) error {
	return service.userRepository.DeleteUser(ctx, id)
}

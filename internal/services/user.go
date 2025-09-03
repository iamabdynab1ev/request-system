package services

import (
	"context"
	"net/http"
	"strings"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

type UserServiceInterface interface {
	GetUsers(ctx context.Context, filter types.Filter) ([]dto.UserDTO, uint64, error)
	FindUser(ctx context.Context, id uint64) (*dto.UserDTO, error)
	CreateUser(ctx context.Context, payload dto.CreateUserDTO) (*dto.UserDTO, error)
	UpdateUser(ctx context.Context, payload dto.UpdateUserDTO) (*dto.UserDTO, error)
	DeleteUser(ctx context.Context, id uint64) error
}

type UserService struct {
	userRepository   repositories.UserRepositoryInterface
	statusRepository repositories.StatusRepositoryInterface
	logger           *zap.Logger
}

func NewUserService(
	userRepository repositories.UserRepositoryInterface,
	statusRepository repositories.StatusRepositoryInterface,
	logger *zap.Logger,
) UserServiceInterface {
	return &UserService{
		userRepository:   userRepository,
		statusRepository: statusRepository,
		logger:           logger,
	}
}

func userEntityToUserDTO(entity *entities.User) *dto.UserDTO {
	if entity == nil {
		return nil
	}

	return &dto.UserDTO{
		ID:           entity.ID,
		Fio:          entity.Fio,
		Email:        entity.Email,
		Position:     entity.Position,
		PhoneNumber:  entity.PhoneNumber,
		RoleID:       entity.RoleID,
		RoleName:     entity.RoleName,
		BranchID:     entity.BranchID,
		DepartmentID: entity.DepartmentID,
		OfficeID:     entity.OfficeID,
		OtdelID:      entity.OtdelID,
		StatusID:     entity.StatusID,

		PhotoURL: entity.PhotoURL,
	}
}

func (s *UserService) GetUsers(ctx context.Context, filter types.Filter) ([]dto.UserDTO, uint64, error) {
	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, _ := s.userRepository.FindUserByID(ctx, userID)

	authContext := authz.Context{Actor: actor, Permissions: permissionsMap, Target: nil}
	if !authz.CanDo(authz.UsersView, authContext) {
		return nil, 0, apperrors.ErrForbidden
	}

	var securityFilter string
	var securityArgs []interface{}

	entities, totalCount, err := s.userRepository.GetUsers(ctx, filter, securityFilter, securityArgs)
	if err != nil {
		return nil, 0, err
	}

	if len(entities) == 0 {
		return []dto.UserDTO{}, totalCount, nil
	}

	dtos := make([]dto.UserDTO, 0, len(entities))
	for _, entity := range entities {
		dtos = append(dtos, *userEntityToUserDTO(&entity))
	}

	return dtos, totalCount, nil
}

func (s *UserService) FindUser(ctx context.Context, id uint64) (*dto.UserDTO, error) {
	// 1. Находим пользователя в базе. Этот код остается.
	user, err := s.userRepository.FindUser(ctx, id)
	if err != nil {
		return nil, err
	}

	// 2. Проверяем права доступа. Этот код тоже остается.
	actorID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := s.userRepository.FindUserByID(ctx, actorID)
	if err != nil {
		s.logger.Error("FindUser: Не удалось найти пользователя-актора", zap.Uint64("actorID", actorID), zap.Error(err))
		return nil, apperrors.ErrUserNotFound
	}

	authContext := authz.Context{
		Actor:       actor,
		Permissions: permissionsMap,
		Target:      user,
	}

	if !authz.CanDo(authz.UsersView, authContext) {
		s.logger.Warn("FindUser: Отказано в доступе при просмотре пользователя", zap.Uint64("targetUserID", id), zap.Uint64("actorID", actor.ID))
		return nil, apperrors.ErrForbidden
	}

	return userEntityToUserDTO(user), nil
}

func (s *UserService) CreateUser(ctx context.Context, payload dto.CreateUserDTO) (*dto.UserDTO, error) {
	actorID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := s.userRepository.FindUserByID(ctx, actorID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	authContext := authz.Context{
		Actor:       actor,
		Permissions: permissionsMap,
	}

	if !authz.CanDo(authz.UsersCreate, authContext) {
		s.logger.Warn("CreateUser: Отказано в доступе на создание пользователя",
			zap.Uint64("actorID", actor.ID),
			zap.Any("payload", payload),
		)
		return nil, apperrors.ErrForbidden
	}

	hashedPassword, err := utils.HashPassword(payload.Password)
	if err != nil {
		s.logger.Error("CreateUser: Не удалось хешировать пароль", zap.Error(err))
		return nil, err
	}

	userEntity := &entities.User{
		Fio:          payload.Fio,
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
		PhotoURL:     payload.PhotoURL,
	}

	createdEntity, err := s.userRepository.CreateUser(ctx, userEntity)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			s.logger.Error("CreateUser: Postgres error при создании пользователя",
				zap.String("code", pgErr.Code),
				zap.String("message", pgErr.Message),
				zap.String("constraint", pgErr.ConstraintName),
				zap.Uint64("actorID", actor.ID),
				zap.Error(err),
			)
			if pgErr.Code == "23505" { // Unique violation
				if strings.Contains(pgErr.ConstraintName, "users_email_key") {
					return nil, apperrors.NewHttpError(http.StatusBadRequest, "Пользователь с таким email уже существует.", nil, nil)
				}
				if strings.Contains(pgErr.ConstraintName, "users_phone_number_key") {
					return nil, apperrors.NewHttpError(http.StatusBadRequest, "Пользователь с таким номером телефона уже существует.", nil, nil)
				}
			}
			if pgErr.Code == "23503" { // Foreign key violation
				return nil, apperrors.NewHttpError(http.StatusBadRequest, "Неверные данные: нарушено ограничение внешнего ключа (BranchID/RoleID и т.д.).", nil, nil)
			}
		} else {
			s.logger.Error("CreateUser: Неизвестная ошибка при создании пользователя",
				zap.Uint64("actorID", actor.ID),
				zap.Error(err),
			)
		}
		return nil, apperrors.ErrInternalServer
	}
	return userEntityToUserDTO(createdEntity), nil
}

func (s *UserService) UpdateUser(ctx context.Context, payload dto.UpdateUserDTO) (*dto.UserDTO, error) {
	actorID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, _ := s.userRepository.FindUserByID(ctx, actorID)
	existingUser, _ := s.userRepository.FindUser(ctx, payload.ID)

	authContext := authz.Context{
		Actor:       actor,
		Permissions: permissionsMap,
		Target:      existingUser,
	}

	isAdmin := authz.CanDo(authz.UsersUpdate, authContext)
	isOwnProfile := actor.ID == existingUser.ID && permissionsMap[authz.ProfileUpdate]

	if !(isAdmin || isOwnProfile) {
		return nil, apperrors.ErrForbidden
	}

	// --- Обновление данных для администратора ---
	if isAdmin {
		if payload.Fio != nil {
			existingUser.Fio = *payload.Fio
		}
		if payload.Email != nil {
			existingUser.Email = *payload.Email
		}
		if payload.PhoneNumber != nil {
			existingUser.PhoneNumber = *payload.PhoneNumber
		}
		if payload.Position != nil {
			existingUser.Position = *payload.Position
		}
		if payload.StatusID != nil {
			existingUser.StatusID = *payload.StatusID
		}
		if payload.RoleID != nil {
			existingUser.RoleID = *payload.RoleID
		}
		if payload.BranchID != nil {
			existingUser.BranchID = *payload.BranchID
		}
		if payload.DepartmentID != nil {
			existingUser.DepartmentID = *payload.DepartmentID
		}
		if payload.OfficeID != nil {
			existingUser.OfficeID = payload.OfficeID
		}
		if payload.OtdelID != nil {
			existingUser.OtdelID = payload.OtdelID
		}
	} else if isOwnProfile {
		// --- Обновление собственного профиля ---
		if payload.Fio != nil {
			existingUser.Fio = *payload.Fio
		}
		if payload.PhoneNumber != nil {
			existingUser.PhoneNumber = *payload.PhoneNumber
		}
		if payload.Position != nil {
			existingUser.Position = *payload.Position
		}
	}

	// --- Пароль ---
	if payload.Password != nil && *payload.Password != "" {
		hashedPassword, _ := utils.HashPassword(*payload.Password)
		existingUser.Password = hashedPassword
	}

	// --- Фото ---
	if payload.PhotoURL != nil {
		existingUser.PhotoURL = payload.PhotoURL
	}

	return userEntityToUserDTO(existingUser), nil
}

func (s *UserService) DeleteUser(ctx context.Context, id uint64) error {
	actorID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, _ := s.userRepository.FindUserByID(ctx, actorID)
	targetUser, _ := s.userRepository.FindUser(ctx, id)

	authContext := authz.Context{
		Actor:       actor,
		Permissions: permissionsMap,
		Target:      targetUser,
	}

	if !authz.CanDo(authz.UsersDelete, authContext) {
		return apperrors.ErrForbidden
	}

	return s.userRepository.DeleteUser(ctx, id)
}

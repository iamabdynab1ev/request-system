package services

import (
	"context"
	"errors"
	"net/http"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/constants" // <-- Убедись, что импорт есть
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"

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
	statusRepository repositories.StatusRepositoryInterface, // <-- Этот параметр уже есть у тебя в router.go, так что все ок.
	logger *zap.Logger,
) UserServiceInterface {
	return &UserService{
		userRepository:   userRepository,
		statusRepository: statusRepository, // <-- Сохраняем его
		logger:           logger,
	}
}

// userEntityToUserDTO не содержит StatusCode.
func userEntityToUserDTO(entity *entities.User) *dto.UserDTO {
	if entity == nil {
		return nil
	}

	return &dto.UserDTO{
		ID:                 entity.ID,
		Fio:                entity.Fio,
		Email:              entity.Email,
		Position:           entity.Position,
		PhoneNumber:        entity.PhoneNumber,
		RoleID:             entity.RoleID,
		RoleName:           entity.RoleName,
		BranchID:           entity.BranchID,
		DepartmentID:       entity.DepartmentID,
		OfficeID:           entity.OfficeID,
		OtdelID:            entity.OtdelID,
		StatusID:           entity.StatusID,
		PhotoURL:           entity.PhotoURL,
		MustChangePassword: entity.MustChangePassword,
	}
}

// CreateUser с правильной логикой поиска статуса
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

	authContext := authz.Context{Actor: actor, Permissions: permissionsMap}
	if !authz.CanDo(authz.UsersCreate, authContext) {
		s.logger.Warn("CreateUser: Отказано в доступе на создание пользователя")
		return nil, apperrors.ErrForbidden
	}

	// ПРАВИЛЬНЫЙ ПУТЬ: Находим ID статуса по его коду.
	activeStatus, err := s.statusRepository.FindByCode(ctx, constants.UserStatusActiveCode)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			s.logger.Error("CreateUser: Критическая ошибка конфигурации. Статус с кодом 'ACTIVE' не найден в базе данных.", zap.Error(err))
			return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации системы: обязательный статус не найден.", err, nil)
		}
		s.logger.Error("CreateUser: Ошибка при поиске статуса 'ACTIVE'", zap.Error(err))
		return nil, err
	}

	// Проверяем, что найденный статус относится к пользователям (type=2)
	if activeStatus.Type != 2 {
		s.logger.Error("CreateUser: Критическая ошибка конфигурации. Статус с кодом 'ACTIVE' не является статусом пользователя (type!=2).", zap.Int("type", activeStatus.Type))
		return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации системы: некорректный тип статуса.", nil, nil)
	}

	// Пароль по умолчанию устанавливается из номера телефона
	hashedPassword, err := utils.HashPassword(payload.PhoneNumber)
	if err != nil {
		s.logger.Error("CreateUser: Не удалось хешировать пароль", zap.Error(err))
		return nil, err
	}

	userEntity := &entities.User{
		Fio:                payload.Fio,
		Email:              payload.Email,
		PhoneNumber:        payload.PhoneNumber,
		Password:           hashedPassword,
		Position:           payload.Position,
		StatusID:           activeStatus.ID, // <-- Используем ID, который мы НАДЕЖНО нашли по коду.
		RoleID:             payload.RoleID,
		BranchID:           payload.BranchID,
		DepartmentID:       payload.DepartmentID,
		OfficeID:           payload.OfficeID,
		OtdelID:            payload.OtdelID,
		PhotoURL:           payload.PhotoURL,
		MustChangePassword: true,
	}

	createdEntity, err := s.userRepository.CreateUser(ctx, userEntity)
	if err != nil {
		s.logger.Error("CreateUser: Репозиторий вернул ошибку", zap.Error(err))
		return nil, err
	}
	return userEntityToUserDTO(createdEntity), nil
}

// Остальные методы без изменений (оставляю, чтобы ты заменил файл целиком)
func (s *UserService) GetUsers(ctx context.Context, filter types.Filter) ([]dto.UserDTO, uint64, error) {
	// ...
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
	// ...
	user, err := s.userRepository.FindUser(ctx, id)
	if err != nil {
		return nil, err
	}
	actorID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, err := s.userRepository.FindUserByID(ctx, actorID)
	if err != nil {
		s.logger.Error("FindUser: Не удалось найти пользователя-актора", zap.Uint64("actorID", actorID), zap.Error(err))
		return nil, apperrors.ErrUserNotFound
	}
	authContext := authz.Context{
		Actor: actor, Permissions: permissionsMap, Target: user,
	}
	if !authz.CanDo(authz.UsersView, authContext) {
		s.logger.Warn("FindUser: Отказано в доступе при просмотре пользователя", zap.Uint64("targetUserID", id), zap.Uint64("actorID", actor.ID))
		return nil, apperrors.ErrForbidden
	}
	return userEntityToUserDTO(user), nil
}

func (s *UserService) UpdateUser(ctx context.Context, payload dto.UpdateUserDTO) (*dto.UserDTO, error) {
	// ...
	actorID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, _ := s.userRepository.FindUserByID(ctx, actorID)
	existingUser, err := s.userRepository.FindUser(ctx, payload.ID)
	if err != nil {
		s.logger.Error("UpdateUser: не удалось найти пользователя для обновления", zap.Uint64("id", payload.ID), zap.Error(err))
		return nil, err
	}
	authContext := authz.Context{
		Actor: actor, Permissions: permissionsMap, Target: existingUser,
	}
	isAdmin := authz.CanDo(authz.UsersUpdate, authContext)
	isOwnProfile := actor.ID == existingUser.ID
	if !isAdmin && !isOwnProfile {
		return nil, apperrors.ErrForbidden
	}
	if isOwnProfile {
		if hasProfileUpdate, ok := permissionsMap[authz.ProfileUpdate]; !ok || !hasProfileUpdate {
			return nil, apperrors.ErrForbidden
		}
	}
	if payload.Password != nil && *payload.Password != "" {
		canUpdatePassword := isAdmin || (isOwnProfile && permissionsMap[authz.PasswordUpdate])
		if !canUpdatePassword {
			return nil, apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав на смену пароля", nil, nil)
		}
		hashedPassword, err := utils.HashPassword(*payload.Password)
		if err != nil {
			s.logger.Error("UpdateUser: ошибка хеширования пароля", zap.Error(err))
			return nil, err
		}
		if err := s.userRepository.UpdatePassword(ctx, payload.ID, hashedPassword); err != nil {
			s.logger.Error("UpdateUser: ошибка обновления пароля в репозитории", zap.Error(err))
			return nil, err
		}
	}
	updatedEntity, err := s.userRepository.UpdateUser(ctx, payload)
	if err != nil {
		s.logger.Error("UpdateUser: ошибка при сохранении изменений в репозитории", zap.Error(err))
		return nil, err
	}
	return userEntityToUserDTO(updatedEntity), nil
}

func (s *UserService) DeleteUser(ctx context.Context, id uint64) error {
	// ...
	actorID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, _ := s.userRepository.FindUserByID(ctx, actorID)
	targetUser, _ := s.userRepository.FindUser(ctx, id)
	authContext := authz.Context{
		Actor: actor, Permissions: permissionsMap, Target: targetUser,
	}
	if !authz.CanDo(authz.UsersDelete, authContext) {
		return apperrors.ErrForbidden
	}
	return s.userRepository.DeleteUser(ctx, id)
}

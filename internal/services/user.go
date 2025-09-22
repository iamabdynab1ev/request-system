package services

import (
	"context"
	"errors"
	"net/http"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"go.uber.org/zap"
)

// --- ИНТЕРФЕЙС И СТРУКТУРА (без изменений) ---

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

// --- КОНВЕРТЕР В DTO (без изменений) ---
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

// --- ИСПРАВЛЕННЫЙ МЕТОД CreateUser ---
func (s *UserService) CreateUser(ctx context.Context, payload dto.CreateUserDTO) (*dto.UserDTO, error) {
	// Нормализуем номер телефона
	normalizedPhone := utils.NormalizeTajikPhoneNumber(payload.PhoneNumber)
	if normalizedPhone == "" {
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат номера телефона.", nil, nil)
	}
	// 1. Проверка прав (используем `buildAuthzContext` для чистоты кода)
	authContext, err := s.buildAuthzContext(ctx, nil) // Цели нет, т.к. создаем нового
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.UsersCreate, *authContext) {
		s.logger.Warn("CreateUser: Отказано в доступе на создание пользователя")
		return nil, apperrors.ErrForbidden
	}

	// 2. ИСПРАВЛЕНИЕ: Надежно получаем ID активного статуса из базы по коду
	activeStatus, err := s.statusRepository.FindByCode(ctx, constants.UserStatusActiveCode)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			s.logger.Error("CreateUser: Критическая ошибка - статус 'ACTIVE' не найден в БД.", zap.Error(err))
			return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации системы: статус 'ACTIVE' не найден.", err, nil)
		}
		s.logger.Error("CreateUser: Ошибка при поиске статуса 'ACTIVE'", zap.Error(err))
		return nil, err
	}

	if activeStatus.Type != 2 { // Убеждаемся, что это статус для пользователя
		s.logger.Error("CreateUser: Критическая ошибка - статус 'ACTIVE' не является статусом пользователя.", zap.Int("type", activeStatus.Type))
		return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации системы: неверный тип статуса 'ACTIVE'.", nil, nil)
	}

	// 3. Хешируем пароль
	hashedPassword, err := utils.HashPassword(payload.PhoneNumber)
	if err != nil {
		s.logger.Error("CreateUser: Не удалось хешировать пароль", zap.Error(err))
		return nil, err
	}

	// 4. Собираем и сохраняем сущность
	userEntity := &entities.User{
		Fio:                payload.Fio,
		Email:              payload.Email,
		PhoneNumber:        payload.PhoneNumber,
		Password:           hashedPassword,
		Position:           payload.Position,
		StatusID:           activeStatus.ID, // <-- Теперь здесь ID из базы, а не константа
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

// --- ИСПРАВЛЕННЫЙ МЕТОД UpdateUser ---
func (s *UserService) UpdateUser(ctx context.Context, payload dto.UpdateUserDTO) (*dto.UserDTO, error) {
	// 1. Получаем пользователя для обновления ("цель")
	targetUser, err := s.userRepository.FindUser(ctx, payload.ID)
	if err != nil {
		return nil, err // Если пользователь не найден, вернется ErrNotFound
	}

	// 2. Строим контекст авторизации с актором и целью
	authContext, err := s.buildAuthzContext(ctx, targetUser)
	if err != nil {
		return nil, err
	}

	// 3. ИСПРАВЛЕНИЕ: Используем ОДНУ простую проверку вместо сложной логики.
	// `CanDo` уже умеет проверять `scope:own` (самого себя), `scope:all` и т.д.
	if !authz.CanDo(authz.UsersUpdate, *authContext) {
		// Дополнительно проверяем право на обновление своего профиля (особый случай)
		isOwnProfile := authContext.Actor.ID == targetUser.ID
		if !(isOwnProfile && authContext.HasPermission(authz.ProfileUpdate)) {
			s.logger.Warn("UpdateUser: отказано в доступе",
				zap.Uint64("actorID", authContext.Actor.ID),
				zap.Uint64("targetUserID", targetUser.ID),
			)
			return nil, apperrors.ErrForbidden
		}
	}

	// 4. Логика для пароля
	if payload.Password != nil && *payload.Password != "" {
		isOwnProfile := authContext.Actor.ID == targetUser.ID

		// Проверяем право на смену пароля (либо общее user:update, либо частное password:update для себя)
		canUpdatePassword := authz.CanDo(authz.UsersUpdate, *authContext) || (isOwnProfile && authContext.HasPermission(authz.PasswordUpdate))

		if !canUpdatePassword {
			return nil, apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав на смену пароля", nil, nil)
		}

		hashedPassword, err := utils.HashPassword(*payload.Password)
		if err != nil {
			return nil, err
		}
		if err := s.userRepository.UpdatePassword(ctx, payload.ID, hashedPassword); err != nil {
			return nil, err
		}
	}

	// 5. Выполнение основного обновления
	updatedEntity, err := s.userRepository.UpdateUser(ctx, payload)
	if err != nil {
		s.logger.Error("UpdateUser: ошибка при сохранении в репозитории", zap.Error(err))
		return nil, err
	}

	return userEntityToUserDTO(updatedEntity), nil
}

// --- ВСПОМОГАТЕЛЬНАЯ ФУНКЦИЯ ---
// Вынесли повторяющийся код в отдельный метод для чистоты
func (s *UserService) buildAuthzContext(ctx context.Context, target *entities.User) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := s.userRepository.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	// `target` может быть `nil`, CanDo это обработает
	return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: target}, nil
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

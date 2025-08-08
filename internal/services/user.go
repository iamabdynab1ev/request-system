package services

import (
	"context"
	"fmt"
	"net/http"                      // Добавлено для http.StatusBadRequest в ErrorResponse, используется в контроллере
	"request-system/internal/authz" // Наш движок авторизации
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils" // Хелперы: GetUserIDFromCtx, GetPermissionsMapFromCtx
	"strings"                  // Добавлен для strings.Contains

	"github.com/jackc/pgx/v5/pgconn" // Для ошибок PostgreSQL (e.g. UniqueConstraintViolation)
	"go.uber.org/zap"                // Для логирования
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

// Хелпер для конвертации статусов
func statusDTOToShortStatusDTO(status *dto.StatusDTO) *dto.ShortStatusDTO {
	if status == nil {
		return nil
	}
	return &dto.ShortStatusDTO{
		ID:   status.ID,
		Name: status.Name,
	}
}

// Хелпер для конвертации UserEntity в UserDTO
func userEntityToDTO(entity *entities.User, status *dto.ShortStatusDTO) *dto.UserDTO {
	if entity == nil {
		return nil
	}
	return &dto.UserDTO{
		ID:          entity.ID,
		Fio:         entity.Fio,
		Email:       entity.Email,
		Position:    entity.Position,
		PhoneNumber: entity.PhoneNumber,
		RoleID:      entity.RoleID,
		RoleName:    entity.RoleName,
		Branch:      entity.BranchID,
		Department:  entity.DepartmentID,
		Office:      entity.OfficeID,
		Otdel:       entity.OtdelID,
		Status:      *status, // Dereference ShortStatusDTO
		PhotoURL:    entity.PhotoURL,
		CreatedAt:   entity.CreatedAt.Format("2006-01-02, 15:04:05"),
		UpdatedAt:   entity.UpdatedAt.Format("2006-01-02, 15:04:05"),
	}
}

// Хелпер для конвертации среза UserEntity в срез UserDTO (используется для GetUsers)
func userEntitiesToDTOs(entities []entities.User, statusRepo repositories.StatusRepositoryInterface, logger *zap.Logger) ([]dto.UserDTO, error) {
	if entities == nil {
		return nil, nil
	}
	dtos := make([]dto.UserDTO, len(entities))
	for i, entity := range entities {
		statusDTO, err := statusRepo.FindStatus(context.Background(), uint64(entity.StatusID))
		if err != nil {
			logger.Error("Не удалось получить статус для пользователя при конвертации в UserDTO", zap.Uint64("userID", entity.ID), zap.Error(err))
			return nil, fmt.Errorf("failed to retrieve status for user %d: %w", entity.ID, err)
		}
		shortStatusDTO := statusDTOToShortStatusDTO(statusDTO)
		dtos[i] = *userEntityToDTO(&entity, shortStatusDTO)
	}
	return dtos, nil
}

// GetUsers - получение списка пользователей с учетом прав доступа
func (s *UserService) GetUsers(ctx context.Context, filter types.Filter) ([]dto.UserDTO, uint64, error) {
	userID, _ := utils.GetUserIDFromCtx(ctx)
	permissionsMap, _ := utils.GetPermissionsMapFromCtx(ctx)
	actor, _ := s.userRepository.FindUserByID(ctx, userID)

	var securityFilter string
	var securityArgs []interface{}

	if !(permissionsMap["superuser"] || permissionsMap["scope:all"]) {
		if permissionsMap["scope:department"] {
			securityFilter = fmt.Sprintf("u.department_id = $%d", len(securityArgs)+1)
			securityArgs = append(securityArgs, actor.DepartmentID)
		} else if permissionsMap["scope:own"] {
			securityFilter = fmt.Sprintf("u.id = $%d", len(securityArgs)+1)
			securityArgs = append(securityArgs, actor.ID)
		} else {
			s.logger.Warn("GetUsers: Отсутствует scope.", zap.Uint64("actorID", actor.ID))
			return []dto.UserDTO{}, 0, nil
		}
	}

	entities, totalCount, err := s.userRepository.GetUsers(ctx, filter, securityFilter, securityArgs)
	if err != nil {
		return nil, 0, err
	}

	// Конвертируем в DTO перед возвратом
	dtos, err := userEntitiesToDTOs(entities, s.statusRepository, s.logger)
	if err != nil {
		return nil, 0, err
	}

	return dtos, totalCount, nil
}

// FindUser - поиск пользователя по ID с учетом прав доступа
func (s *UserService) FindUser(ctx context.Context, id uint64) (*dto.UserDTO, error) {
	user, err := s.userRepository.FindUser(ctx, id)
	if err != nil {
		return nil, err
	}

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
		s.logger.Error("FindUser: Не удалось найти пользователя-актора", zap.Uint64("actorID", actorID), zap.Error(err))
		return nil, apperrors.ErrUserNotFound
	}

	authContext := authz.Context{
		Actor:       actor,
		Permissions: permissionsMap,
		Target:      user, // Цель - пользователь
	}

	if !authz.CanDo("users:view", authContext) {
		s.logger.Warn("FindUser: Отказано в доступе при просмотре пользователя по ID",
			zap.Uint64("targetUserID", id),
			zap.Uint64("actorID", actor.ID),
			zap.Any("permissions", permissionsMap),
		)
		return nil, apperrors.ErrForbidden
	}

	statusDTO, err := s.statusRepository.FindStatus(ctx, uint64(user.StatusID))
	if err != nil {
		s.logger.Error("FindUser: Не удалось получить статус для пользователя", zap.Uint64("userID", user.ID), zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve status for user %d: %w", user.ID, err)
	}
	shortStatusDTO := statusDTOToShortStatusDTO(statusDTO)

	return userEntityToDTO(user, shortStatusDTO), nil
}

// CreateUser - создание нового пользователя
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
		Target:      nil, // Цель не нужна для Create, достаточно базового пермишена
	}

	if !authz.CanDo("users:create", authContext) {
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
		Fio: payload.Fio, Email: payload.Email, PhoneNumber: payload.PhoneNumber, Password: hashedPassword,
		Position: payload.Position, StatusID: payload.StatusID, RoleID: payload.RoleID,
		BranchID: payload.BranchID, DepartmentID: payload.DepartmentID, OfficeID: payload.OfficeID,
		OtdelID: payload.OtdelID, PhotoURL: payload.PhotoURL,
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
					return nil, apperrors.NewHttpError(http.StatusBadRequest, "Пользователь с таким email уже существует.", nil)
				}
				if strings.Contains(pgErr.ConstraintName, "users_phone_number_key") {
					return nil, apperrors.NewHttpError(http.StatusBadRequest, "Пользователь с таким номером телефона уже существует.", nil)
				}
			}
			if pgErr.Code == "23503" { // Foreign key violation
				return nil, apperrors.NewHttpError(http.StatusBadRequest, "Неверные данные: нарушено ограничение внешнего ключа (BranchID/RoleID и т.д.).", nil)
			}
		} else {
			s.logger.Error("CreateUser: Неизвестная ошибка при создании пользователя",
				zap.Uint64("actorID", actor.ID),
				zap.Error(err),
			)
		}
		return nil, apperrors.ErrInternalServer // Общая ошибка для пользователя
	}

	statusDTO, err := s.statusRepository.FindStatus(ctx, uint64(createdEntity.StatusID))
	if err != nil {
		s.logger.Error("CreateUser: Не удалось получить статус для созданного пользователя",
			zap.Uint64("userID", createdEntity.ID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to retrieve status for created user %d: %w", createdEntity.ID, err)
	}
	shortStatusDTO := statusDTOToShortStatusDTO(statusDTO)

	return userEntityToDTO(createdEntity, shortStatusDTO), nil
}

// UpdateUser - обновление пользователя
// internal/services/user_service.go

func (s *UserService) UpdateUser(ctx context.Context, payload dto.UpdateUserDTO) (*dto.UserDTO, error) {
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

	existingUser, err := s.userRepository.FindUser(ctx, payload.ID)
	if err != nil {
		s.logger.Error("UpdateUser: Не удалось найти пользователя для обновления", zap.Uint64("targetUserID", payload.ID), zap.Error(err))
		return nil, err
	}

	authContext := authz.Context{
		Actor: actor, Permissions: permissionsMap, Target: existingUser,
	}

	// Определяем, является ли пользователь админом с правом редактировать ВСЕ
	isAdminWithFullAccess := authz.CanDo("users:update", authContext)

	// Определяем, редактирует ли пользователь свой профиль
	isUpdatingOwnProfile := (actor.ID == existingUser.ID) && permissionsMap["profile:update"]

	if !(isAdminWithFullAccess || isUpdatingOwnProfile) {
		s.logger.Warn("UpdateUser: Отказано в доступе на обновление пользователя",
			zap.Uint64("targetUserID", payload.ID),
			zap.Uint64("actorID", actor.ID),
		)
		return nil, apperrors.ErrForbidden
	}

	// --- ПРИМЕНЯЕМ ИЗМЕНЕНИЯ СОГЛАСНО ПРАВАМ ---

	if isAdminWithFullAccess {
		// АДМИН МОЖЕТ МЕНЯТЬ ВСЁ
		if payload.Fio != "" {
			existingUser.Fio = payload.Fio
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
	} else if isUpdatingOwnProfile {
		// ОБЫЧНЫЙ ПОЛЬЗОВАТЕЛЬ МЕНЯЕТ ТОЛЬКО СВОЙ ПРОФИЛЬ (безопасные поля)
		if payload.Fio != "" {
			existingUser.Fio = payload.Fio
		}
		if payload.PhoneNumber != "" {
			existingUser.PhoneNumber = payload.PhoneNumber
		}
		if payload.Position != "" {
			existingUser.Position = payload.Position
		}
	}

	// Эти поля могут менять оба (если прошли проверку)
	if payload.Password != "" {
		// TODO: Добавить проверку на password:update / users:password:reset
		hashedPassword, _ := utils.HashPassword(payload.Password)
		existingUser.Password = hashedPassword
	}
	if payload.PhotoURL != nil {
		existingUser.PhotoURL = payload.PhotoURL
	}

	existingUser.OfficeID = payload.OfficeID
	existingUser.OtdelID = payload.OtdelID

	// 3. Сохраняем в репозиторий
	updatedEntity, err := s.userRepository.UpdateUser(ctx, existingUser)
	if err != nil {
		// ... (твой код обработки ошибок postgres)
	}

	// 4. Конвертируем в DTO
	statusDTO, _ := s.statusRepository.FindStatus(ctx, updatedEntity.StatusID)
	return userEntityToDTO(updatedEntity, statusDTOToShortStatusDTO(statusDTO)), nil
}

// DeleteUser - мягкое удаление пользователя
func (s *UserService) DeleteUser(ctx context.Context, id uint64) error {
	actorID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return err
	}
	actor, err := s.userRepository.FindUserByID(ctx, actorID)
	if err != nil {
		return apperrors.ErrUserNotFound
	}

	targetUser, err := s.userRepository.FindUser(ctx, id) // Пользователь, которого хотят удалить
	if err != nil {
		s.logger.Error("DeleteUser: Не удалось найти пользователя для удаления", zap.Uint64("targetUserID", id), zap.Error(err))
		return err
	}

	authContext := authz.Context{
		Actor:       actor,
		Permissions: permissionsMap,
		Target:      targetUser, // Цель - удаляемый пользователь
	}

	// 'users:delete' + scope. Например: "users:delete" + "scope:all" (для админа)
	if !authz.CanDo("users:delete", authContext) {
		s.logger.Warn("DeleteUser: Отказано в доступе на удаление пользователя",
			zap.Uint64("targetUserID", id),
			zap.Uint64("actorID", actor.ID),
		)
		return apperrors.ErrForbidden
	}

	s.logger.Info("DeleteUser: Начало операции мягкого удаления пользователя", zap.Uint64("userID", id), zap.Uint64("actorID", actor.ID))
	err = s.userRepository.DeleteUser(ctx, id)
	if err != nil {
		s.logger.Error("DeleteUser: Ошибка при мягком удалении пользователя из репозитория", zap.Uint64("userID", id), zap.Error(err))
	}
	return err
}

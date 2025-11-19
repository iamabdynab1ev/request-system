package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const telegramLinkTokenTTL = 10 * time.Minute

type UserServiceInterface interface {
	GetUsers(ctx context.Context, filter types.Filter) ([]dto.UserDTO, uint64, error)
	FindUser(ctx context.Context, id uint64) (*dto.UserDTO, error)
	CreateUser(ctx context.Context, payload dto.CreateUserDTO) (*dto.UserDTO, error)
	UpdateUser(ctx context.Context, payload dto.UpdateUserDTO, rawRequestBody []byte) (*dto.UserDTO, error)
	DeleteUser(ctx context.Context, id uint64) error
	GetPermissionDetailsForUser(ctx context.Context, userID uint64) (*dto.UIPermissionsResponseDTO, error)
	UpdateUserPermissions(ctx context.Context, userID uint64, payload dto.UpdateUserPermissionsDTO) error

	GenerateTelegramLinkToken(ctx context.Context) (string, error)
	ConfirmTelegramLink(ctx context.Context, token string, chatID int64) error
	FindUserByTelegramChatID(ctx context.Context, chatID int64) (*entities.User, error)
}

type UserService struct {
	txManager             repositories.TxManagerInterface
	userRepository        repositories.UserRepositoryInterface
	roleRepository        repositories.RoleRepositoryInterface
	permissionRepository  repositories.PermissionRepositoryInterface
	statusRepository      repositories.StatusRepositoryInterface
	cacheRepository       repositories.CacheRepositoryInterface
	authPermissionService AuthPermissionServiceInterface
	logger                *zap.Logger
}

func NewUserService(
	txManager repositories.TxManagerInterface,
	userRepository repositories.UserRepositoryInterface,
	roleRepository repositories.RoleRepositoryInterface,
	permissionRepository repositories.PermissionRepositoryInterface,
	statusRepository repositories.StatusRepositoryInterface,
	cacheRepository repositories.CacheRepositoryInterface,
	authPermissionService AuthPermissionServiceInterface,
	logger *zap.Logger,
) UserServiceInterface {
	return &UserService{
		txManager:             txManager,
		userRepository:        userRepository,
		roleRepository:        roleRepository,
		permissionRepository:  permissionRepository,
		statusRepository:      statusRepository,
		cacheRepository:       cacheRepository,
		authPermissionService: authPermissionService,
		logger:                logger,
	}
}

func (s *UserService) GetUsers(ctx context.Context, filter types.Filter) ([]dto.UserDTO, uint64, error) {
	authContext, err := s.buildAuthzContext(ctx, nil)
	if err != nil {
		return nil, 0, err
	}
	if !authz.CanDo(authz.UsersView, *authContext) {
		return nil, 0, apperrors.ErrForbidden
	}

	users, totalCount, err := s.userRepository.GetUsers(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if len(users) == 0 {
		return []dto.UserDTO{}, 0, nil
	}

	userIDs := make([]uint64, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	userRolesMap, err := s.userRepository.GetRolesByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]dto.UserDTO, len(users))
	for i, u := range users {
		dto := userEntityToUserDTO(&u)
		if roles, ok := userRolesMap[u.ID]; ok && len(roles) > 0 {
			roleIDs := make([]uint64, len(roles))
			for j, r := range roles {
				roleIDs[j] = r.ID
			}
			dto.RoleIDs = roleIDs
		} else {
			dto.RoleIDs = []uint64{}
		}
		dtos[i] = *dto
	}

	return dtos, totalCount, nil
}

func (s *UserService) GetPermissionDetailsForUser(ctx context.Context, userID uint64) (*dto.UIPermissionsResponseDTO, error) {
	// Просто "пробрасываем" вызов в репозиторий, где находится вся логика

	return s.permissionRepository.GetDetailedPermissionsForUI(ctx, userID)
}

func (s *UserService) FindUser(ctx context.Context, id uint64) (*dto.UserDTO, error) {
	userEntity, err := s.userRepository.FindUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	authContext, err := s.buildAuthzContext(ctx, userEntity)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.UsersView, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	roles, err := s.userRepository.GetRolesByUserID(ctx, id)
	if err != nil {
		return nil, err
	}
	roleIDs := make([]uint64, len(roles))
	for i, r := range roles {
		roleIDs[i] = r.ID
	}
	userDTO := userEntityToUserDTO(userEntity)
	userDTO.RoleIDs = roleIDs

	return userDTO, nil
}

func (s *UserService) CreateUser(ctx context.Context, payload dto.CreateUserDTO) (*dto.UserDTO, error) {
	authContext, err := s.buildAuthzContext(ctx, nil)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.UsersCreate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	activeStatusID, err := s.statusRepository.FindIDByCode(ctx, constants.UserStatusActiveCode)
	if err != nil {
	}

	hashedPassword, err := utils.HashPassword(payload.PhoneNumber)
	if err != nil {
		return nil, err
	}

	entity := entities.User{
		Fio:                payload.Fio,
		Email:              payload.Email,
		PhoneNumber:        payload.PhoneNumber,
		Password:           hashedPassword,
		PositionID:         &payload.PositionID,
		StatusID:           activeStatusID,
		BranchID:           payload.BranchID,
		DepartmentID:       payload.DepartmentID,
		OfficeID:           payload.OfficeID,
		OtdelID:            payload.OtdelID,
		PhotoURL:           payload.PhotoURL,
		IsHead:             &payload.IsHead,
		MustChangePassword: true,
	}

	var createdID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		var txErr error
		createdID, txErr = s.userRepository.CreateFromSync(ctx, tx, entity)
		if txErr != nil {
			return txErr
		}
		if txErr = s.userRepository.SyncUserRoles(ctx, tx, createdID, payload.RoleIDs); txErr != nil {
			return txErr
		}
		return nil
	})
	if err != nil {
		s.logger.Error("Ошибка в транзакции создания пользователя", zap.Error(err))
		return nil, err
	}

	return s.FindUser(ctx, createdID)
}

// UpdateUser - ИСПРАВЛЕН
func (s *UserService) UpdateUser(ctx context.Context, payload dto.UpdateUserDTO, rawRequestBody []byte) (*dto.UserDTO, error) {
	targetUser, err := s.userRepository.FindUserByID(ctx, payload.ID)
	if err != nil {
		return nil, err
	}

	authContext, err := s.buildAuthzContext(ctx, targetUser)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.UsersUpdate, *authContext) {
		return nil, apperrors.ErrForbidden
	}

	// Применяем изменения к существующей сущности
	// (логику можно вынести в хелпер)
	if payload.Fio != nil {
		targetUser.Fio = *payload.Fio
	}
	if payload.Email != nil {
		targetUser.Email = *payload.Email
	}
	if payload.PhoneNumber != nil {
		targetUser.PhoneNumber = *payload.PhoneNumber
	}
	if payload.PositionID != nil {
		targetUser.PositionID = payload.PositionID
	}
	if payload.StatusID != nil {
		targetUser.StatusID = *payload.StatusID
	}
	if payload.BranchID != nil {
		targetUser.BranchID = payload.BranchID
	}
	if payload.DepartmentID != nil {
		targetUser.DepartmentID = payload.DepartmentID
	}
	if payload.OfficeID != nil {
		targetUser.OfficeID = payload.OfficeID
	}
	if payload.OtdelID != nil {
		targetUser.OtdelID = payload.OtdelID
	}
	if payload.PhotoURL != nil {
		targetUser.PhotoURL = payload.PhotoURL
	}
	if payload.IsHead != nil {
		targetUser.IsHead = payload.IsHead
	}

	// ИСПРАВЛЕНИЕ: Используем TxManager
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		// ИСПОЛЬЗУЕМ НОВЫЙ МЕТОД UpdateFromSync
		if txErr := s.userRepository.UpdateFromSync(ctx, tx, payload.ID, *targetUser); txErr != nil {
			return txErr
		}

		// (Опционально) Обновляем пароль, если он передан (остается без изменений)
		// (Опционально) Синхронизируем роли, если они переданы
		if payload.RoleIDs != nil {
			if txErr := s.userRepository.SyncUserRoles(ctx, tx, payload.ID, *payload.RoleIDs); txErr != nil {
				return txErr
			}
		}

		return nil
	})
	if err != nil {
		s.logger.Error("Ошибка в транзакции обновления пользователя", zap.Error(err))
		return nil, err
	}

	// Инвалидация кеша после успешного коммита
	if payload.RoleIDs != nil {
		s.authPermissionService.InvalidateUserPermissionsCache(ctx, payload.ID)
	}

	return s.FindUser(ctx, payload.ID)
}

func (s *UserService) DeleteUser(ctx context.Context, id uint64) error {
	targetUser, err := s.userRepository.FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil
		}
		return err
	}

	authContext, err := s.buildAuthzContext(ctx, targetUser)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.UsersDelete, *authContext) {
		return apperrors.ErrForbidden
	}

	return s.userRepository.DeleteUser(ctx, id)
}

func userEntityToUserDTO(entity *entities.User) *dto.UserDTO {
	if entity == nil {
		return nil
	}

	var isHead bool
	if entity.IsHead != nil {
		isHead = *entity.IsHead
	}

	return &dto.UserDTO{
		ID:                 entity.ID,
		Fio:                entity.Fio,
		Email:              entity.Email,
		PhoneNumber:        entity.PhoneNumber,
		StatusID:           entity.StatusID,
		DepartmentID:       entity.DepartmentID,
		BranchID:           entity.BranchID,
		PositionID:         entity.PositionID,
		OfficeID:           entity.OfficeID,
		OtdelID:            entity.OtdelID,
		MustChangePassword: entity.MustChangePassword,
		IsHead:             isHead,
		PhotoURL:           entity.PhotoURL,
	}
}

func (s *UserService) UpdateUserPermissions(ctx context.Context, userID uint64, payload dto.UpdateUserPermissionsDTO) error {
	// 1. Проверяем право на само действие
	authContext, err := s.buildAuthzContext(ctx, nil)
	if err != nil {
		return err
	}
	if !authz.CanDo(authz.UsersUpdate, *authContext) {
		return apperrors.ErrForbidden
	}

	// 2. Получаем "фундамент" - все права, которые приходят от ролей пользователя
	rolePermissionIDs, err := s.permissionRepository.GetRolePermissionIDsForUser(ctx, userID)
	if err != nil {
		s.logger.Error("Не удалось получить права ролей для пользователя", zap.Error(err))
		return apperrors.NewHttpError(http.StatusInternalServerError, "Не удалось получить права ролей", err, nil)
	}
	rolePermsMap := make(map[uint64]bool)
	for _, id := range rolePermissionIDs {
		rolePermsMap[id] = true
	}

	// 3. Вычисляем списки для записи в базу, основываясь на "фундаменте"
	finalDirectPermissions := make([]uint64, 0)
	finalDeniedPermissions := make([]uint64, 0)

	// Какие права стали индивидуальными? (Есть в has_access, но нет в правах от роли)
	for _, id := range payload.HasAccessIDs {
		if !rolePermsMap[id] {
			finalDirectPermissions = append(finalDirectPermissions, id)
		}
	}

	// Какие ролевые права были заблокированы? (Есть в no_access и есть в правах от роли)
	for _, id := range payload.NoAccessIDs {
		if rolePermsMap[id] {
			finalDeniedPermissions = append(finalDeniedPermissions, id)
		}
	}

	// 4. В транзакции полностью перезаписываем индивидуальные и заблокированные права
	tx, err := s.userRepository.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.userRepository.SyncUserDirectPermissions(ctx, tx, userID, finalDirectPermissions); err != nil {
		return err
	}
	if err := s.userRepository.SyncUserDeniedPermissions(ctx, tx, userID, finalDeniedPermissions); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	s.logger.Info("Индивидуальные права изменены. Инвалидируем кэш.", zap.Uint64("userID", userID))
	return s.authPermissionService.InvalidateUserPermissionsCache(ctx, userID)
}

func (s *UserService) buildAuthzContext(ctx context.Context, target *entities.User) (*authz.Context, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	// Используем MAP для быстрой проверки прав
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	actor, err := s.userRepository.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	var targetInterface interface{}
	if target != nil {
		targetInterface = target
	}

	return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: targetInterface}, nil
}

func (s *UserService) GenerateTelegramLinkToken(ctx context.Context) (string, error) {
	// 1. Получаем ID текущего пользователя из контекста
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		s.logger.Error("GenerateTelegramLinkToken: не удалось получить userID из контекста", zap.Error(err))
		return "", err
	}

	// 2. Генерируем уникальный, случайный токен
	token := uuid.New().String()

	// 3. Формируем ключ для Redis
	redisKey := fmt.Sprintf("telegram-link-token:%s", token)

	// 4. Сохраняем в Redis связку "токен -> userID" со временем жизни
	err = s.cacheRepository.Set(ctx, redisKey, userID, telegramLinkTokenTTL)
	if err != nil {
		s.logger.Error("GenerateTelegramLinkToken: не удалось сохранить токен в Redis",
			zap.String("redisKey", redisKey),
			zap.Uint64("userID", userID),
			zap.Error(err),
		)
		return "", apperrors.ErrInternalServer
	}

	s.logger.Info("Сгенерирован токен для привязки Telegram", zap.Uint64("userID", userID))

	// 5. Возвращаем токен пользователю (он его отправит боту)
	return token, nil
}

// ConfirmTelegramLink проверяет токен и привязывает chat_id к пользователю
func (s *UserService) ConfirmTelegramLink(ctx context.Context, token string, chatID int64) error {
	// 1. Формируем ключ и ищем его в Redis
	redisKey := fmt.Sprintf("telegram-link-token:%s", token)

	val, err := s.cacheRepository.Get(ctx, redisKey)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			s.logger.Warn("ConfirmTelegramLink: Попытка использовать неверный или просроченный токен", zap.String("token", token))
			return apperrors.NewBadRequestError("Неверный код или истекло время его действия")
		}
		s.logger.Error("ConfirmTelegramLink: Ошибка при получении токена из Redis", zap.Error(err))
		return apperrors.ErrInternalServer
	}
	s.logger.Info("ConfirmTelegramLink: Получен userID из Redis", zap.String("valueFromRedis", val))

	// 2. Конвертируем userID из строки в uint64
	userID, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		s.logger.Error("ConfirmTelegramLink: Не удалось распарсить userID из Redis", zap.String("valueFromRedis", val), zap.Error(err))
		return apperrors.ErrInternalServer
	}
	s.logger.Info("ConfirmTelegramLink: userID успешно распарсен", zap.Uint64("userID", userID))
	// 3. Обновляем chat_id для найденного пользователя в базе данных
	err = s.userRepository.UpdateTelegramChatID(ctx, userID, chatID)
	if err != nil {
		s.logger.Error("ConfirmTelegramLink: Не удалось обновить telegram_chat_id в базе",
			zap.Uint64("userID", userID),
			zap.Int64("chatID", chatID),
			zap.Error(err),
		)
		return apperrors.ErrInternalServer
	}

	// 4. Удаляем токен из Redis, чтобы его нельзя было использовать повторно
	_ = s.cacheRepository.Del(ctx, redisKey)

	s.logger.Info("Telegram-аккаунт успешно привязан", zap.Uint64("userID", userID), zap.Int64("chatID", chatID))
	return nil
}

func (s *UserService) FindUserByTelegramChatID(ctx context.Context, chatID int64) (*entities.User, error) {
	return s.userRepository.FindUserByTelegramChatID(ctx, chatID)
}

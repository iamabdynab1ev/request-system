package services

import (
	"context"
	"errors"
	"net/http"
	"time"

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

type UserServiceInterface interface {
	GetUsers(ctx context.Context, filter types.Filter) ([]dto.UserDTO, uint64, error)
	FindUser(ctx context.Context, id uint64) (*dto.UserDTO, error)
	CreateUser(ctx context.Context, payload dto.CreateUserDTO) (*dto.UserDTO, error)
	UpdateUser(ctx context.Context, payload dto.UpdateUserDTO) (*dto.UserDTO, error)
	DeleteUser(ctx context.Context, id uint64) error
	GetPermissionDetailsForUser(ctx context.Context, userID uint64) (*dto.UIPermissionsResponseDTO, error)
	UpdateUserPermissions(ctx context.Context, userID uint64, payload dto.UpdateUserPermissionsDTO) error
}

type UserService struct {
	userRepository        repositories.UserRepositoryInterface
	roleRepository        repositories.RoleRepositoryInterface
	permissionRepository  repositories.PermissionRepositoryInterface
	statusRepository      repositories.StatusRepositoryInterface
	authPermissionService AuthPermissionServiceInterface
	logger                *zap.Logger
}

func NewUserService(
	userRepository repositories.UserRepositoryInterface,
	roleRepository repositories.RoleRepositoryInterface,
	permissionRepository repositories.PermissionRepositoryInterface,
	statusRepository repositories.StatusRepositoryInterface,
	authPermissionService AuthPermissionServiceInterface,
	logger *zap.Logger,
) UserServiceInterface {
	return &UserService{
		userRepository:        userRepository,
		roleRepository:        roleRepository,
		permissionRepository:  permissionRepository,
		statusRepository:      statusRepository,
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

	if payload.IsHead {
		exists, err := s.userRepository.IsHeadExistsInDepartment(ctx, payload.DepartmentID, 0)
		if err != nil {
			return nil, apperrors.ErrInternalServer
		}
		if exists {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "В этом департаменте уже назначен руководитель.", nil, nil)
		}
	}

	activeStatusID, err := s.statusRepository.FindIDByCode(ctx, constants.UserStatusActiveCode)
	if err != nil {
		return nil, apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка конфигурации: статус 'ACTIVE' не найден.", err, nil)
	}

	hashedPassword, err := utils.HashPassword(payload.PhoneNumber)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	userEntity := &entities.User{
		Fio:                payload.Fio,
		Email:              payload.Email,
		PhoneNumber:        payload.PhoneNumber,
		Password:           hashedPassword,
		PositionID:         &payload.PositionID,
		StatusID:           activeStatusID,
		BranchID:           &payload.BranchID,
		DepartmentID:       payload.DepartmentID,
		OfficeID:           payload.OfficeID,
		OtdelID:            payload.OtdelID,
		PhotoURL:           payload.PhotoURL,
		IsHead:             &payload.IsHead,
		MustChangePassword: true,
		BaseEntity:         types.BaseEntity{CreatedAt: &now, UpdatedAt: &now},
	}

	tx, err := s.userRepository.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	createdID, err := s.userRepository.CreateUser(ctx, tx, userEntity)
	if err != nil {
		return nil, err
	}

	if err = s.userRepository.SyncUserRoles(ctx, tx, createdID, payload.RoleIDs); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.FindUser(ctx, createdID)
}

func (s *UserService) UpdateUser(ctx context.Context, payload dto.UpdateUserDTO) (*dto.UserDTO, error) {
	targetUser, err := s.userRepository.FindUserByID(ctx, payload.ID)
	if err != nil {
		return nil, err // Если пользователь не найден, вернется ошибка
	}

	// 2. Создаем контекст авторизации.
	authContext, err := s.buildAuthzContext(ctx, targetUser)
	if err != nil {
		return nil, err
	}
	if !authz.CanDo(authz.UsersUpdate, *authContext) {
		s.logger.Warn("Отказано в доступе на обновление пользователя",
			zap.Uint64("actorID", authContext.Actor.ID),
			zap.Uint64("targetID", targetUser.ID),
			zap.String("requiredPermission", authz.UsersUpdate),
		)
		return nil, apperrors.ErrForbidden
	}

	tx, err := s.userRepository.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// 4. Обновляем основные поля пользователя
	if err = s.userRepository.UpdateUser(ctx, tx, payload); err != nil {
		return nil, err
	}

	// 5. (Опционально) Обновляем пароль, если он передан
	if payload.Password != nil && *payload.Password != "" {
		if !authz.CanDo(authz.UsersPasswordReset, *authContext) {
			return nil, apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав на сброс/смену пароля", nil, nil)
		}
		hashedPassword, err := utils.HashPassword(*payload.Password)
		if err != nil {
			return nil, err
		}
		if err = s.userRepository.UpdatePassword(ctx, payload.ID, hashedPassword); err != nil {
			return nil, err
		}
	}

	// 6. (Опционально) Синхронизируем роли, если они переданы
	if payload.RoleIDs != nil {
		if err = s.userRepository.SyncUserRoles(ctx, tx, payload.ID, *payload.RoleIDs); err != nil {
			return nil, err
		}

		// 7. САМОЕ ГЛАВНОЕ: Если роли изменились, инвалидируем кэш
		if err := s.authPermissionService.InvalidateUserPermissionsCache(ctx, payload.ID); err != nil {
			s.logger.Warn("Не удалось инвалидировать кеш пользователя после обновления ролей", zap.Uint64("userID", payload.ID), zap.Error(err))
			// Не возвращаем ошибку, так как основная операция прошла успешно
		}
	}

	// 8. Коммитим транзакцию
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	// 9. Возвращаем обновленный профиль
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

	var branchID, positionID uint64
	var isHead bool
	if entity.BranchID != nil {
		branchID = *entity.BranchID
	}
	if entity.PositionID != nil {
		positionID = *entity.PositionID
	}
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
		MustChangePassword: entity.MustChangePassword,
		BranchID:           branchID,
		PositionID:         positionID,
		IsHead:             isHead,
		OfficeID:           entity.OfficeID,
		OtdelID:            entity.OtdelID,
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

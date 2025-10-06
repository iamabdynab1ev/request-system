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
}

type UserService struct {
	userRepository        repositories.UserRepositoryInterface
	roleRepository        repositories.RoleRepositoryInterface // <-- ЭТОТ РЕПОЗИТОРИЙ НУЖЕН
	permissionRepository  repositories.PermissionRepositoryInterface
	statusRepository      repositories.StatusRepositoryInterface
	authPermissionService AuthPermissionServiceInterface
	logger                *zap.Logger
}

func NewUserService(
	userRepository repositories.UserRepositoryInterface,
	roleRepository repositories.RoleRepositoryInterface, // <-- ЭТОТ РЕПОЗИТОРИЙ НУЖЕН
	permissionRepository repositories.PermissionRepositoryInterface,
	statusRepository repositories.StatusRepositoryInterface,
	authPermissionService AuthPermissionServiceInterface,
	logger *zap.Logger,
) UserServiceInterface {
	return &UserService{
		userRepository:        userRepository,
		roleRepository:        roleRepository, // <-- ЭТОТ РЕПОЗИТОРИЙ НУЖЕН
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

	userEntities, totalCount, err := s.userRepository.GetUsers(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if len(userEntities) == 0 {
		return []dto.UserDTO{}, 0, nil
	}

	userIDs := make([]uint64, len(userEntities))
	for i, u := range userEntities {
		userIDs[i] = u.ID
	}

	userRolesMap, err := s.userRepository.GetRolesByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]dto.UserDTO, len(userEntities))
	for i, entity := range userEntities {
		userDTO := userEntityToUserDTO(&entity)
		userDTO.RoleIDs = make([]uint64, 0)
		if roles, ok := userRolesMap[entity.ID]; ok {
			for _, role := range roles {
				userDTO.RoleIDs = append(userDTO.RoleIDs, role.ID)
			}
		}
		dtos[i] = *userDTO
	}
	return dtos, totalCount, nil
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

	permissionsInfo, err := s.calculatePermissionLists(ctx, id)
	if err != nil {
		return nil, err
	}

	userDTO := userEntityToUserDTO(userEntity)
	userDTO.RoleIDs = roleIDs
	userDTO.Permissions = permissionsInfo

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

	activeStatus, err := s.statusRepository.FindByCode(ctx, constants.UserStatusActiveCode)
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
		Position:           payload.Position,
		StatusID:           uint64(activeStatus.ID),
		BranchID:           payload.BranchID,
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
		return nil, err
	}

	authContext, err := s.buildAuthzContext(ctx, targetUser)
	if err != nil {
		return nil, err
	}
	isOwnProfile := authContext.Actor.ID == targetUser.ID
	canUpdateUser := authz.CanDo(authz.UsersUpdate, *authContext)
	canUpdateOwnProfile := isOwnProfile && authContext.HasPermission(authz.ProfileUpdate)

	if !canUpdateUser && !canUpdateOwnProfile {
		return nil, apperrors.ErrForbidden
	}

	if payload.IsHead != nil && *payload.IsHead {
		departmentID := targetUser.DepartmentID
		if payload.DepartmentID != nil {
			departmentID = *payload.DepartmentID
		}
		exists, err := s.userRepository.IsHeadExistsInDepartment(ctx, departmentID, payload.ID)
		if err != nil {
			return nil, apperrors.ErrInternalServer
		}
		if exists {
			return nil, apperrors.NewHttpError(http.StatusBadRequest, "В этом департаменте уже назначен руководитель.", nil, nil)
		}
	}

	tx, err := s.userRepository.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err = s.userRepository.UpdateUser(ctx, tx, payload); err != nil {
		return nil, err
	}

	if payload.Password != nil && *payload.Password != "" {
		canResetPassword := authz.CanDo(authz.UsersPasswordReset, *authContext)
		canChangeOwnPassword := isOwnProfile && authContext.HasPermission(authz.PasswordUpdate)
		if !canResetPassword && !canChangeOwnPassword {
			return nil, apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав на смену пароля", nil, nil)
		}
		hashedPassword, err := utils.HashPassword(*payload.Password)
		if err != nil {
			return nil, err
		}
		if err = s.userRepository.UpdatePassword(ctx, payload.ID, hashedPassword); err != nil {
			return nil, err
		}
	}

	if payload.RoleIDs != nil {
		if !canUpdateUser {
			return nil, apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав на изменение ролей пользователя", nil, nil)
		}
		if err = s.userRepository.SyncUserRoles(ctx, tx, payload.ID, *payload.RoleIDs); err != nil {
			return nil, err
		}
	}

	if payload.DirectPermissionIDs != nil {
		if !canUpdateUser {
			return nil, apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав на изменение индивидуальных прав", nil, nil)
		}
		if err = s.userRepository.SyncUserDirectPermissions(ctx, tx, payload.ID, *payload.DirectPermissionIDs); err != nil {
			return nil, err
		}
	}

	if payload.DeniedPermissionIDs != nil {
		if !canUpdateUser {
			return nil, apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав на изменение индивидуальных прав", nil, nil)
		}
		if err = s.userRepository.SyncUserDeniedPermissions(ctx, tx, payload.ID, *payload.DeniedPermissionIDs); err != nil {
			return nil, err
		}
	}

	if payload.RoleIDs != nil || payload.DirectPermissionIDs != nil || payload.DeniedPermissionIDs != nil {
		if err := s.authPermissionService.InvalidateUserPermissionsCache(ctx, payload.ID); err != nil {
			s.logger.Warn("Не удалось инвалидировать кеш пользователя после обновления", zap.Uint64("userID", payload.ID), zap.Error(err))
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
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

	var targetInterface interface{}
	if target != nil {
		targetInterface = target
	}

	return &authz.Context{Actor: actor, Permissions: permissionsMap, Target: targetInterface}, nil
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
		Position:           entity.Position,
		PhoneNumber:        entity.PhoneNumber,
		BranchID:           entity.BranchID,
		DepartmentID:       entity.DepartmentID,
		OfficeID:           entity.OfficeID,
		OtdelID:            entity.OtdelID,
		StatusID:           entity.StatusID,
		PhotoURL:           entity.PhotoURL,
		MustChangePassword: entity.MustChangePassword,
		IsHead:             isHead,
	}
}

func (s *UserService) calculatePermissionLists(ctx context.Context, userID uint64) (*dto.PermissionsInfoDTO, error) {
	allSystemPermissions, _, err := s.permissionRepository.GetPermissions(ctx, 0, 0, "")
	if err != nil {
		return nil, err
	}

	currentPermissionIDs, err := s.permissionRepository.GetFinalUserPermissionIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	currentPermissionsSet := make(map[uint64]bool, len(currentPermissionIDs))
	for _, id := range currentPermissionIDs {
		currentPermissionsSet[id] = true
	}

	unavailablePermissionIDs := make([]uint64, 0)
	for _, p := range allSystemPermissions {
		if !currentPermissionsSet[p.ID] {
			unavailablePermissionIDs = append(unavailablePermissionIDs, p.ID)
		}
	}

	return &dto.PermissionsInfoDTO{
		CurrentPermissionIDs:     currentPermissionIDs,
		UnavailablePermissionIDs: unavailablePermissionIDs,
	}, nil
}

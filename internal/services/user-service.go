package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/types"
	"request-system/pkg/utils"
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

// ---------------- READING ----------------

func (s *UserService) GetUsers(ctx context.Context, filter types.Filter) ([]dto.UserDTO, uint64, error) {
	// Auth Check
	if _, err := s.checkAccess(ctx, authz.UsersView, nil); err != nil {
		return nil, 0, err
	}

	users, total, err := s.userRepository.GetUsers(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if len(users) == 0 {
		return []dto.UserDTO{}, 0, nil
	}

	// Обогащаем Ролями (Bulk)
	uids := make([]uint64, len(users))
	for i, u := range users {
		uids[i] = u.ID
	}

	rolesMap, _ := s.userRepository.GetRolesByUserIDs(ctx, uids)

	dtos := make([]dto.UserDTO, len(users))
	for i, u := range users {
		d := userEntityToUserDTO(&u)
		if roles, ok := rolesMap[u.ID]; ok {
			for _, r := range roles {
				d.RoleIDs = append(d.RoleIDs, r.ID)
			}
		}
		dtos[i] = *d
	}
	return dtos, total, nil
}

func (s *UserService) FindUser(ctx context.Context, id uint64) (*dto.UserDTO, error) {
	u, err := s.userRepository.FindUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if _, err := s.checkAccess(ctx, authz.UsersView, u); err != nil {
		return nil, err
	}

	d := userEntityToUserDTO(u)
	roles, _ := s.userRepository.GetRolesByUserID(ctx, id)
	for _, r := range roles {
		d.RoleIDs = append(d.RoleIDs, r.ID)
	}

	return d, nil
}

func (s *UserService) GetPermissionDetailsForUser(ctx context.Context, userID uint64) (*dto.UIPermissionsResponseDTO, error) {
	if _, err := s.checkAccess(ctx, authz.UsersView, nil); err != nil {
		return nil, err
	}
	return s.permissionRepository.GetDetailedPermissionsForUI(ctx, userID)
}

// ---------------- WRITING (CREATE/UPDATE) ----------------

func (s *UserService) CreateUser(ctx context.Context, p dto.CreateUserDTO) (*dto.UserDTO, error) {
	if _, err := s.checkAccess(ctx, authz.UsersCreate, nil); err != nil {
		return nil, err
	}

	// Получаем дефолтный статус
	stID, err := s.statusRepository.FindIDByCode(ctx, constants.UserStatusActiveCode)
	if err != nil {
		return nil, apperrors.ErrInternalServer
	} // Если нет статуса - ошибка конфига

	hash, err := utils.HashPassword(p.PhoneNumber) // Пароль = телефон
	if err != nil {
		return nil, err
	}

	entity := &entities.User{
		Fio: p.Fio, Email: p.Email, PhoneNumber: p.PhoneNumber, Password: hash,
		PositionID: &p.PositionID, StatusID: stID,
		BranchID: p.BranchID, DepartmentID: p.DepartmentID,
		OfficeID: p.OfficeID, OtdelID: p.OtdelID,
		PhotoURL: p.PhotoURL, IsHead: &p.IsHead, MustChangePassword: true,
	}

	var newID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		id, err := s.userRepository.CreateUser(ctx, tx, entity) // ВНИМАНИЕ: CreateUser, а не FromSync
		if err != nil {
			return err
		}
		newID = id

		if len(p.RoleIDs) > 0 {
			return s.userRepository.SyncUserRoles(ctx, tx, id, p.RoleIDs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.FindUser(ctx, newID)
}

// UpdateUser — ВОТ ТУТ ГЛАВНАЯ МАГИЯ REFLECTION
func (s *UserService) UpdateUser(ctx context.Context, p dto.UpdateUserDTO, rawBody []byte) (*dto.UserDTO, error) {
	target, err := s.userRepository.FindUserByID(ctx, p.ID)
	if err != nil {
		return nil, err
	}

	if _, err := s.checkAccess(ctx, authz.UsersUpdate, target); err != nil {
		return nil, err
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		// 1. Применяем DTO -> Entity (Automatic Patching)
		// Используем нашу утилиту. Она обновит Fio, Email и т.д.
		utils.ApplyUpdates(target, &p)

		// 2. Особая обработка пароля (если он пришел в DTO)
		if p.Password != nil && len(*p.Password) >= 6 {
			hash, err := utils.HashPassword(*p.Password)
			if err != nil {
				return err
			}
			target.Password = hash
		} else {
			// Пароль пустой (не трогаем), в SQL update не отправим или пустой string
			// Репозиторий должен уметь игнорировать пустой пароль (мы это сделали)
			target.Password = ""
		}

		// 3. Сохраняем поля
		if err := s.userRepository.UpdateUser(ctx, tx, target); err != nil {
			return err
		}

		// 4. Обновляем роли (если список пришел и не nil)
		if p.RoleIDs != nil {
			if err := s.userRepository.SyncUserRoles(ctx, tx, p.ID, *p.RoleIDs); err != nil {
				return err
			}
			// Сброс кэша прав
			s.authPermissionService.InvalidateUserPermissionsCache(ctx, p.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.FindUser(ctx, p.ID)
}

func (s *UserService) DeleteUser(ctx context.Context, id uint64) error {
	u, err := s.userRepository.FindUserByID(ctx, id)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if _, err := s.checkAccess(ctx, authz.UsersDelete, u); err != nil {
		return err
	}

	return s.userRepository.DeleteUser(ctx, id)
}

// ---------------- PERMISSIONS (Granular) ----------------

func (s *UserService) UpdateUserPermissions(ctx context.Context, userID uint64, payload dto.UpdateUserPermissionsDTO) error {
	if _, err := s.checkAccess(ctx, authz.UsersUpdate, nil); err != nil {
		return err
	}

	// Logic remains same (fetching base roles + computing deltas)
	// (Your logic was solid here)
	rolePermIDs, err := s.permissionRepository.GetRolePermissionIDsForUser(ctx, userID)
	if err != nil {
		return apperrors.NewInternalError("Ошибка получения прав ролей")
	}

	basePerms := make(map[uint64]bool)
	for _, id := range rolePermIDs {
		basePerms[id] = true
	}

	add := []uint64{}
	deny := []uint64{}

	for _, id := range payload.HasAccessIDs {
		if !basePerms[id] {
			add = append(add, id)
		}
	}
	for _, id := range payload.NoAccessIDs {
		if basePerms[id] {
			deny = append(deny, id)
		}
	}

	return s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		if err := s.userRepository.SyncUserDirectPermissions(ctx, tx, userID, add); err != nil {
			return err
		}
		if err := s.userRepository.SyncUserDeniedPermissions(ctx, tx, userID, deny); err != nil {
			return err
		}
		return s.authPermissionService.InvalidateUserPermissionsCache(ctx, userID)
	})
}

// ---------------- TELEGRAM LINK ----------------

func (s *UserService) GenerateTelegramLinkToken(ctx context.Context) (string, error) {
	uid, _ := utils.GetUserIDFromCtx(ctx)
	token := uuid.New().String()
	key := fmt.Sprintf("telegram-link-token:%s", token)
	if err := s.cacheRepository.Set(ctx, key, uid, telegramLinkTokenTTL); err != nil {
		return "", apperrors.ErrInternalServer
	}
	return token, nil
}

func (s *UserService) ConfirmTelegramLink(ctx context.Context, token string, chatID int64) error {
	key := fmt.Sprintf("telegram-link-token:%s", token)
	val, err := s.cacheRepository.Get(ctx, key)
	if err != nil {
		return apperrors.NewBadRequestError("Неверный код или истек")
	}

	uid, _ := strconv.ParseUint(val, 10, 64)
	if err := s.userRepository.UpdateTelegramChatID(ctx, uid, chatID); err != nil {
		return err
	}
	_ = s.cacheRepository.Del(ctx, key)
	return nil
}

func (s *UserService) FindUserByTelegramChatID(ctx context.Context, chatID int64) (*entities.User, error) {
	return s.userRepository.FindUserByTelegramChatID(ctx, chatID)
}

// ---------------- HELPERS ----------------

func (s *UserService) checkAccess(ctx context.Context, perm string, target interface{}) (*authz.Context, error) {
	actorID, _ := utils.GetUserIDFromCtx(ctx)
	actor, _ := s.userRepository.FindUserByID(ctx, actorID)
	perms, _ := utils.GetPermissionsMapFromCtx(ctx)

	ac := &authz.Context{Actor: actor, Permissions: perms, Target: target}
	if !authz.CanDo(perm, *ac) {
		return nil, apperrors.ErrForbidden
	}
	return ac, nil
}

func userEntityToUserDTO(e *entities.User) *dto.UserDTO {
	if e == nil {
		return nil
	}
	d := &dto.UserDTO{
		ID: e.ID, Fio: e.Fio, Email: e.Email, PhoneNumber: e.PhoneNumber,
		StatusID: e.StatusID, StatusCode: e.StatusCode,
		BranchID: e.BranchID, DepartmentID: e.DepartmentID,
		PositionID: e.PositionID, OfficeID: e.OfficeID, OtdelID: e.OtdelID,
		PhotoURL: e.PhotoURL, MustChangePassword: e.MustChangePassword,
	}
	if e.IsHead != nil {
		d.IsHead = *e.IsHead
	}
	if e.CreatedAt != nil {
		d.CreatedAt = e.CreatedAt.Format(time.RFC3339)
	} // Используем правильный time.Time
	if e.UpdatedAt != nil {
		d.UpdatedAt = e.UpdatedAt.Format(time.RFC3339)
	}
	return d
}

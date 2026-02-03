package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

	UpdateUser(ctx context.Context, payload dto.UpdateUserDTO, explicitFields map[string]interface{}) (*dto.UserDTO, error)

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
	otdelRepository       repositories.OtdelRepositoryInterface
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
	otdelRepo repositories.OtdelRepositoryInterface,
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
		otdelRepository:       otdelRepo,
		roleRepository:        roleRepository,
		permissionRepository:  permissionRepository,
		statusRepository:      statusRepository,
		cacheRepository:       cacheRepository,
		authPermissionService: authPermissionService,
		logger:                logger,
	}
}

func (s *UserService) GetUsers(ctx context.Context, filter types.Filter) ([]dto.UserDTO, uint64, error) {
	// 1. Проверка прав
	if _, err := s.checkAccess(ctx, authz.UsersView, nil); err != nil {
		return nil, 0, err
	}

	// 2. Получение списка
	users, total, err := s.userRepository.GetUsers(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if len(users) == 0 {
		return []dto.UserDTO{}, 0, nil
	}

	// 3. Сбор ID для batch-загрузки
	uids := make([]uint64, len(users))
	for i, u := range users {
		uids[i] = u.ID
	}

	// 4. Загрузка связей
	rolesMap, _ := s.userRepository.GetRolesByUserIDs(ctx, uids)
	positionsMap, _ := s.userRepository.GetPositionIDsByUserIDs(ctx, uids)

	otdelsMap, _ := s.userRepository.GetOtdelIDsByUserIDs(ctx, uids) 

	// 5. Сборка DTO
	dtos := make([]dto.UserDTO, len(users))
	for i, u := range users {
		d := userEntityToUserDTO(&u)
		
		// Роли
		if roles, ok := rolesMap[u.ID]; ok {
			for _, r := range roles {
				d.RoleIDs = append(d.RoleIDs, r.ID)
			}
		}
		
		// Должности
		if posIDs, ok := positionsMap[u.ID]; ok {
			d.PositionIDs = posIDs
		} else {
            d.PositionIDs = []uint64{}
        }

		if list, ok := otdelsMap[u.ID]; ok {
			d.OtdelIDs = list
		} else {
			d.OtdelIDs = []uint64{}
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

	pids, err := s.userRepository.GetPositionIDsByUserID(ctx, id)
	if err == nil {
		u.PositionIDs = pids 
	}
    otdIDs, _ := s.userRepository.GetOtdelIDsByUserID(ctx, u.ID)
	if err == nil { u.OtdelIDs = otdIDs }

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

func (s *UserService) CreateUser(ctx context.Context, p dto.CreateUserDTO) (*dto.UserDTO, error) {
	if _, err := s.checkAccess(ctx, authz.UsersCreate, nil); err != nil {
		return nil, err
	}
	if len(p.PositionIDs) > 2 {
        return nil, apperrors.NewBadRequestError("Превышен лимит должностей. Максимум можно назначить 2 должности.")
    }
	if len(p.OtdelIDs) > 2 {
        return nil, apperrors.NewBadRequestError("Превышен лимит отделов. Сотрудник может быть привязан максимум к 2 отделам.")
    }
 if err := s.validateHierarchy(ctx, p.DepartmentID, p.OtdelID, p.OtdelIDs); err != nil {
        return nil, err
    }
	stID, err := s.statusRepository.FindIDByCode(ctx, constants.UserStatusActiveCode)
	if err != nil {
		return nil, apperrors.ErrInternalServer
	}

	hash, err := utils.HashPassword(p.PhoneNumber)
	if err != nil {
		return nil, err
	}

	entity := &entities.User{
		Fio: p.Fio, Username: p.Username, Email: p.Email, PhoneNumber: p.PhoneNumber, Password: hash,
		PositionID: &p.PositionID,   PositionIDs: p.PositionIDs,  StatusID: stID,
		BranchID: p.BranchID, DepartmentID: p.DepartmentID,
		OfficeID: p.OfficeID, OtdelID: p.OtdelID, 
		PhotoURL: p.PhotoURL, IsHead: &p.IsHead, MustChangePassword: true,
	}

	var newID uint64
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		id, err := s.userRepository.CreateUser(ctx, tx, entity)
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

// Файл: internal/services/user_service.go

func (s *UserService) UpdateUser(ctx context.Context, p dto.UpdateUserDTO, explicitFields map[string]interface{}) (*dto.UserDTO, error) {
	// 1. Ищем цель (кого редактируем)
	target, err := s.userRepository.FindUserByID(ctx, p.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, err
	}

	// 2. Проверяем права доступа
	actorID, _ := utils.GetUserIDFromCtx(ctx)
	permissions, _ := utils.GetPermissionsMapFromCtx(ctx)

	isSelf := (actorID == target.ID)

	if isSelf {

		canProfile := permissions[authz.ProfileUpdate] 
		canUser    := permissions[authz.UsersUpdate]   
		
		if !canProfile && !canUser {
			s.logger.Warn("Нет прав на редактирование своего профиля", zap.Uint64("userID", actorID))
			return nil, apperrors.ErrForbidden
		}
	} else {
		// Если я редактирую другого, мне СТРОГО нужно user:update
		if !permissions[authz.UsersUpdate] {
			s.logger.Warn("Попытка редактировать чужой профиль без user:update", 
				zap.Uint64("actorID", actorID), 
				zap.Uint64("targetID", target.ID))
			return nil, apperrors.ErrForbidden
		}
	}
	// ======================================================================

	// Валидация лимитов (Админам можно разрешить больше, но пока оставляем как есть)
	if p.PositionIDs != nil && len(*p.PositionIDs) > 2 {
		return nil, apperrors.NewBadRequestError("Превышен лимит должностей. Максимум 2.")
	}
	if p.OtdelIDs != nil && len(*p.OtdelIDs) > 2 {
		return nil, apperrors.NewBadRequestError("Превышен лимит отделов. Максимум 2.")
	}

	// ... [ВЕСЬ ОСТАЛЬНОЙ ВАШ КОД БЕЗ ИЗМЕНЕНИЙ НИЖЕ] ...

	// Логика определения иерархии для валидации
	finalDept := target.DepartmentID
	if p.DepartmentID != nil { finalDept = p.DepartmentID }

	var finalExtras []uint64
	if p.OtdelIDs != nil {
		finalExtras = *p.OtdelIDs
	} else {
		existingOtdels, _ := s.userRepository.GetOtdelIDsByUserID(ctx, target.ID)
		finalExtras = existingOtdels
	}

	finalMainOtdel := target.OtdelID
	if p.OtdelID != nil { 
		finalMainOtdel = p.OtdelID 
	} else if p.OtdelIDs != nil && len(*p.OtdelIDs) > 0 {
		first := (*p.OtdelIDs)[0]
		finalMainOtdel = &first
	} else if p.OtdelIDs != nil && len(*p.OtdelIDs) == 0 {
		finalMainOtdel = nil
	}

	if err := s.validateHierarchy(ctx, finalDept, finalMainOtdel, finalExtras); err != nil {
		return nil, err
	}

	// Проверка прав AD (Оставляем как есть, это важно)
	if _, fieldExists := explicitFields["username"]; fieldExists {
		// Если меняем AD, право manage_ad все равно нужно, даже для себя
		if !permissions[authz.UserManageADLink] {
			return nil, apperrors.NewHttpError(http.StatusForbidden, "У вас нет прав на привязку логина AD", nil, nil)
		}
	}

	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		updatedEntity := *target
		utils.SmartUpdate(&updatedEntity, explicitFields)

		// Должности
		if p.PositionIDs != nil {
			updatedEntity.PositionIDs = *p.PositionIDs 
			if len(updatedEntity.PositionIDs) > 0 {
				first := updatedEntity.PositionIDs[0]
				updatedEntity.PositionID = &first
			} else {
				updatedEntity.PositionID = nil 
			}
		}

		// Отделы
		if p.OtdelIDs != nil {
			updatedEntity.OtdelIDs = *p.OtdelIDs
			if len(updatedEntity.OtdelIDs) > 0 {
				first := updatedEntity.OtdelIDs[0]
				updatedEntity.OtdelID = &first 
			} else {
				updatedEntity.OtdelID = nil 
			}
		}

		// Пароль, Фото, Username
		if p.Password != nil && len(*p.Password) >= 6 {
			hash, _ := utils.HashPassword(*p.Password)
			updatedEntity.Password = hash
			

			if isSelf {
				updatedEntity.MustChangePassword = false
			}
		}
		
		if p.PhotoURL != nil {
			updatedEntity.PhotoURL = p.PhotoURL
		}
		if val, exists := explicitFields["username"]; exists && val == nil {
			updatedEntity.Username = nil
		}

		if err := s.userRepository.UpdateUser(ctx, tx, &updatedEntity); err != nil {
			return err
		}

		if p.RoleIDs != nil {
	
			if !permissions[authz.UsersUpdate] && isSelf {
		
			} else {
				if err := s.userRepository.SyncUserRoles(ctx, tx, p.ID, *p.RoleIDs); err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil { return nil, err }
	s.authPermissionService.InvalidateUserPermissionsCache(ctx, p.ID)
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

func (s *UserService) UpdateUserPermissions(ctx context.Context, userID uint64, payload dto.UpdateUserPermissionsDTO) error {
	if _, err := s.checkAccess(ctx, authz.UsersUpdate, nil); err != nil {
		return err
	}

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
		Username: e.Username,
		StatusID: e.StatusID, StatusCode: e.StatusCode,
		BranchID: e.BranchID, DepartmentID: e.DepartmentID,
		PositionID: e.PositionID, 
       
        PositionIDs: e.PositionIDs, 
 		OtdelIDs: e.OtdelIDs,
        OfficeID: e.OfficeID, OtdelID: e.OtdelID,
		PhotoURL: e.PhotoURL, MustChangePassword: e.MustChangePassword,
		PositionName:   e.PositionName,
		BranchName:     e.BranchName,
		DepartmentName: e.DepartmentName,
		OtdelName:      e.OtdelName,
		OfficeName:     e.OfficeName,
	}
	if e.IsHead != nil {
		d.IsHead = *e.IsHead
	}
	if e.CreatedAt != nil {
		d.CreatedAt = e.CreatedAt.Format(time.RFC3339)
	}
	if e.UpdatedAt != nil {
		d.UpdatedAt = e.UpdatedAt.Format(time.RFC3339)
	}
	return d
}
func (s *UserService) validateHierarchy(ctx context.Context, deptID *uint64, mainOtdelID *uint64, extraOtdelIDs []uint64) error {
	if deptID == nil || *deptID == 0 {
		return nil
	}

	// Собираем все ID и очищаем от дублей
	checkMap := make(map[uint64]bool)
	if mainOtdelID != nil && *mainOtdelID > 0 {
		checkMap[*mainOtdelID] = true
	}
	for _, id := range extraOtdelIDs {
		if id > 0 { checkMap[id] = true }
	}

	if len(checkMap) == 0 {
		return nil
	}

	// Превращаем обратно в чистый слайс без повторов
	cleanList := make([]uint64, 0, len(checkMap))
	for id := range checkMap {
		cleanList = append(cleanList, id)
	}

	isValid, err := s.otdelRepository.ValidateOtdelsInDepartment(ctx, *deptID, cleanList)
	if err != nil {
		return apperrors.ErrInternalServer
	}

	if !isValid {
	
		s.logger.Warn("Hierarchy validation failed", zap.Uint64("deptID", *deptID), zap.Uint64s("checkedIDs", cleanList))
		return apperrors.NewBadRequestError("Отдел(ы) не принадлежат выбранному департаменту.")
	}
	return nil
}

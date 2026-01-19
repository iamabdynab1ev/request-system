package services

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"request-system/internal/authz"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"

	"github.com/google/uuid"
	"go.uber.org/zap"

	ldap "github.com/go-ldap/ldap/v3"

	"request-system/pkg/config"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/filestorage"
	"request-system/pkg/utils"
)

var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)

type AuthServiceInterface interface {
	Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error)
	GetUserByID(ctx context.Context, userID uint64) (*dto.UserProfileDTO, error)
	RequestPasswordReset(ctx context.Context, payload dto.ResetPasswordRequestDTO) error
	VerifyResetCode(ctx context.Context, payload dto.VerifyCodeDTO) (*dto.VerifyCodeResponseDTO, error)
	ResetPassword(ctx context.Context, payload dto.ResetPasswordDTO) error
	UpdateMyProfile(ctx context.Context, payload dto.UpdateMyProfileDTO) (*dto.UserDTO, error)
	FindUser(ctx context.Context, id uint64) (*dto.UserDTO, error)
}

type AuthService struct {
	txManager         repositories.TxManagerInterface
	userRepo          repositories.UserRepositoryInterface
	cacheRepo         repositories.CacheRepositoryInterface
	fileStorage       filestorage.FileStorageInterface
	logger            *zap.Logger
	cfg               *config.AuthConfig
	ldapCfg           *config.LDAPConfig
	notifySvc         NotificationServiceInterface
	positionService   PositionServiceInterface
	branchService     BranchServiceInterface
	departmentService DepartmentServiceInterface
	otdelService      OtdelServiceInterface
	officeService     OfficeServiceInterface
}

func NewAuthService(
	txManager repositories.TxManagerInterface,
	userRepo repositories.UserRepositoryInterface,
	cacheRepo repositories.CacheRepositoryInterface,
	fileStorage filestorage.FileStorageInterface,
	logger *zap.Logger,
	cfg *config.AuthConfig,
	ldapCfg *config.LDAPConfig,
	notifySvc NotificationServiceInterface,
	positionService PositionServiceInterface,
	branchService BranchServiceInterface,
	departmentService DepartmentServiceInterface,
	otdelService OtdelServiceInterface,
	officeService OfficeServiceInterface,
) AuthServiceInterface {
	return &AuthService{
		txManager:         txManager,
		userRepo:          userRepo,
		cacheRepo:         cacheRepo,
		fileStorage:       fileStorage,
		logger:            logger,
		cfg:               cfg,
		ldapCfg:           ldapCfg,
		notifySvc:         notifySvc,
		positionService:   positionService,
		branchService:     branchService,
		departmentService: departmentService,
		otdelService:      otdelService,
		officeService:     officeService,
	}
}

// Приватный метод аутентификации в AD
func (s *AuthService) authenticateInAD(username, password string) error {
	l, err := ldap.DialURL(fmt.Sprintf("ldap://%s:%d", s.ldapCfg.Host, s.ldapCfg.Port))
	if err != nil {
		s.logger.Error("Не удалось подключиться к LDAP-серверу", zap.Error(err), zap.String("host", s.ldapCfg.Host))
		return apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка подключения к сервису аутентификации", err, nil)
	}
	defer l.Close()

	userRDN := fmt.Sprintf(`%s\%s`, s.ldapCfg.Domain, username)
	err = l.Bind(userRDN, password)
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
			return apperrors.ErrInvalidCredentials
		}
		return apperrors.NewHttpError(http.StatusInternalServerError, "Системная ошибка аутентификации", err, nil)
	}
	return nil
}

func (s *AuthService) Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error) {
	loginInput := strings.ToLower(payload.Login)
	systemRootEmail := strings.ToLower(s.cfg.SystemRootLogin)

	user, err := s.userRepo.FindUserByEmailOrLogin(ctx, loginInput)
	if err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}

	if user.StatusCode != constants.UserStatusActiveCode {
		return nil, apperrors.ErrUserDisabled
	}

	authenticated := false
	if systemRootEmail != "" && (loginInput == systemRootEmail || user.Email == systemRootEmail) {
		if err := utils.ComparePasswords(user.Password, payload.Password); err == nil {
			authenticated = true
		}
	} else {
		if s.ldapCfg.Enabled {
			adUsername := loginInput
			if user.Username != nil && *user.Username != "" {
				adUsername = *user.Username
			}
			if err := s.authenticateInAD(adUsername, payload.Password); err == nil {
				authenticated = true
			}
		} else {
			if err := utils.ComparePasswords(user.Password, payload.Password); err == nil {
				authenticated = true
			}
		}
	}

	if !authenticated {
		return nil, apperrors.ErrInvalidCredentials
	}

	if user.MustChangePassword {
		resetToken := uuid.New().String()
		s.cacheRepo.Set(ctx, fmt.Sprintf(constants.CacheKeyForceChangeToken, resetToken), user.ID, 15*time.Minute)
		errDetails := apperrors.ErrChangePasswordWithToken
		errDetails.Details = dto.ChangePasswordRequiredDTO{ResetToken: resetToken, Message: "Первый вход: необходимо сменить временный пароль."}
		return nil, errDetails
	}

	return user, nil
}

func (s *AuthService) UpdateMyProfile(ctx context.Context, payload dto.UpdateMyProfileDTO) (*dto.UserDTO, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil { return nil, err }

	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil { return nil, err }

	if _, hasPermission := permissionsMap[authz.ProfileUpdate]; !hasPermission {
		return nil, apperrors.ErrForbidden
	}

	userEntity, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil { return nil, err }

	// Логика удаления/замены фото
	if payload.PhotoURL != nil {
		// Физически удаляем старое, если оно было
		if userEntity.PhotoURL != nil {
			_ = s.fileStorage.Delete(*userEntity.PhotoURL)
		}

		if *payload.PhotoURL == "SET_NULL" {
			userEntity.PhotoURL = nil 
		} else {
			userEntity.PhotoURL = payload.PhotoURL
		}
	}

	if payload.Fio != nil { userEntity.Fio = *payload.Fio }
	if payload.PhoneNumber != nil { userEntity.PhoneNumber = *payload.PhoneNumber }
	if payload.Email != nil { userEntity.Email = *payload.Email }

	tx, err := s.userRepo.BeginTx(ctx)
	if err != nil { return nil, err }
	defer tx.Rollback(ctx)

	if err := s.userRepo.UpdateUser(ctx, tx, userEntity); err != nil { return nil, err }
	if err := tx.Commit(ctx); err != nil { return nil, err }

	return s.FindUser(ctx, userID)
}

func (s *AuthService) GetUserByID(ctx context.Context, userID uint64) (*dto.UserProfileDTO, error) {
	user, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil { return nil, apperrors.ErrUserNotFound }

	res := &dto.UserProfileDTO{ID: user.ID, Email: user.Email, Phone: user.PhoneNumber, FIO: user.Fio, PhotoURL: user.PhotoURL}

	if user.DepartmentID != nil {
		if dep, err := s.departmentService.FindDepartment(ctx, *user.DepartmentID); err == nil { res.DepartmentName = dep.Name }
	}
	if user.OtdelID != nil {
		if otdel, err := s.otdelService.FindOtdel(ctx, *user.OtdelID); err == nil { res.OtdelName = &otdel.Name }
	}
	if user.PositionID != nil {
		if pos, err := s.positionService.GetByID(ctx, uint64(*user.PositionID)); err == nil { res.PositionName = pos.Name }
	}
	if user.BranchID != nil {
		if br, err := s.branchService.FindBranch(ctx, *user.BranchID); err == nil { res.BranchName = br.Name }
	}

	return res, nil
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, payload dto.ResetPasswordRequestDTO) error {
	loginInput := strings.ToLower(payload.Login)
	user, _ := s.userRepo.FindUserByEmailOrLogin(ctx, loginInput)
	if user == nil { return nil }

	resetCode := fmt.Sprintf("%04d", rand.Intn(10000))
	s.cacheRepo.Set(ctx, fmt.Sprintf(constants.CacheKeyResetPhoneCode, loginInput), resetCode, time.Minute*15)

	if user.TelegramChatID.Valid && user.TelegramChatID.Int64 != 0 {
		message := fmt.Sprintf("Ваш код для сброса пароля: %s", resetCode)
		_ = s.notifySvc.SendPlainMessage(ctx, user.TelegramChatID.Int64, message)
	}
	return nil
}

func (s *AuthService) VerifyResetCode(ctx context.Context, payload dto.VerifyCodeDTO) (*dto.VerifyCodeResponseDTO, error) {
	login := strings.ToLower(payload.Login)
	storedCode, _ := s.cacheRepo.Get(ctx, fmt.Sprintf(constants.CacheKeyResetPhoneCode, login))
	if storedCode == "" || storedCode != payload.Code {
		return nil, apperrors.ErrInvalidCredentials
	}

	user, _ := s.userRepo.FindUserByEmailOrLogin(ctx, login)
	vToken := uuid.New().String()
	s.cacheRepo.Set(ctx, fmt.Sprintf(constants.CacheKeyVerifyPhone, vToken), user.ID, time.Minute*15)
	return &dto.VerifyCodeResponseDTO{VerificationToken: vToken}, nil
}

func (s *AuthService) ResetPassword(ctx context.Context, payload dto.ResetPasswordDTO) error {
	userIDStr, err := s.cacheRepo.Get(ctx, fmt.Sprintf(constants.CacheKeyVerifyPhone, payload.Token))
	if err != nil { return apperrors.ErrInvalidCredentials }

	parsedID, _ := strconv.ParseUint(userIDStr, 10, 64)
	hashedPassword, _ := utils.HashPassword(payload.NewPassword)
	return s.userRepo.UpdatePassword(ctx, parsedID, hashedPassword)
}

func (s *AuthService) FindUser(ctx context.Context, id uint64) (*dto.UserDTO, error) {
	user, err := s.userRepo.FindUserByID(ctx, id)
	if err != nil { return nil, err }
	roles, _ := s.userRepo.GetRolesByUserID(ctx, id)
	
	d := &dto.UserDTO{ID: user.ID, Fio: user.Fio, Email: user.Email, PhoneNumber: user.PhoneNumber, PhotoURL: user.PhotoURL}
	for _, r := range roles { d.RoleIDs = append(d.RoleIDs, r.ID) }
	return d, nil
}

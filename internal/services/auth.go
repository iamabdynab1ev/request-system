package services

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"

	ldap "github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/pkg/config"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/filestorage"
	"request-system/pkg/utils"
)

var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)

type AuthServiceInterface interface {
	Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error)
	// Метод для получения своего профиля (/auth/me)
	GetUserByID(ctx context.Context, userID uint64) (*dto.UserProfileDTO, error)
	RequestPasswordReset(ctx context.Context, payload dto.ResetPasswordRequestDTO) error
	VerifyResetCode(ctx context.Context, payload dto.VerifyCodeDTO) (*dto.VerifyCodeResponseDTO, error)
	ResetPassword(ctx context.Context, payload dto.ResetPasswordDTO) error
	UpdateMyProfile(ctx context.Context, payload dto.UpdateMyProfileDTO) (*dto.UserDTO, error)
}

type AuthService struct {
	txManager   repositories.TxManagerInterface
	userRepo    repositories.UserRepositoryInterface
	cacheRepo   repositories.CacheRepositoryInterface
	fileStorage filestorage.FileStorageInterface
	logger      *zap.Logger
	cfg         *config.AuthConfig
	ldapCfg     *config.LDAPConfig
	notifySvc   NotificationServiceInterface
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

	_ PositionServiceInterface,
	_ BranchServiceInterface,
	_ DepartmentServiceInterface,
	_ OtdelServiceInterface,
	_ OfficeServiceInterface,
) AuthServiceInterface {
	return &AuthService{
		txManager:   txManager,
		userRepo:    userRepo,
		cacheRepo:   cacheRepo,
		fileStorage: fileStorage,
		logger:      logger,
		cfg:         cfg,
		ldapCfg:     ldapCfg,
		notifySvc:   notifySvc,
	}
}

// ... метод authenticateInAD остается без изменений ...
func (s *AuthService) authenticateInAD(username, password string) error {
	dialer := &net.Dialer{Timeout: s.ldapCfg.Timeout}
	l, err := ldap.DialURL(
		fmt.Sprintf("ldap://%s:%d", s.ldapCfg.Host, s.ldapCfg.Port),
		ldap.DialWithDialer(dialer),
	)
	if err != nil {
		s.logger.Error("Не удалось подключиться к LDAP-серверу", zap.Error(err), zap.Duration("timeout", s.ldapCfg.Timeout))
		return apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка подключения к сервису аутентификации", err, nil)
	}
	defer l.Close()

	userRDN := fmt.Sprintf(`%s\%s`, s.ldapCfg.Domain, username)
	err = l.Bind(userRDN, password)
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
			return apperrors.ErrInvalidCredentials
		}
		s.logger.Error("LDAP bind failed", zap.String("username", username), zap.Error(err))
		return apperrors.NewHttpError(http.StatusInternalServerError, "Системная ошибка аутентификации", err, nil)
	}
	return nil
}

func isInvalidCredentialsError(err error) bool {
	var httpErr *apperrors.HttpError
	if errors.As(err, &httpErr) {
		return httpErr.Code == http.StatusUnauthorized && httpErr.Message == apperrors.ErrInvalidCredentials.Message
	}
	return false
}

// ... Login остается без изменений ...
func (s *AuthService) Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error) {
	loginInput := strings.ToLower(payload.Login)
	systemRootEmail := strings.ToLower(s.cfg.SystemRootLogin)

	user, err := s.userRepo.FindUserByEmailOrLogin(ctx, loginInput)
	if err != nil {
		s.logger.Error("Ошибка при поиске пользователя (FindUserByEmailOrLogin)",
			zap.String("login", loginInput),
			zap.Error(err),
		)
		return nil, apperrors.ErrInvalidCredentials
	}

	if user.StatusCode != constants.UserStatusActiveCode {
		s.logger.Warn("Попытка входа заблокированного пользователя", zap.String("login", loginInput))
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
			} else if !isInvalidCredentialsError(err) {
				s.logger.Error("LDAP authentication system error", zap.String("login", loginInput), zap.String("ad_username", adUsername), zap.Error(err))
				return nil, err
			}
		} else {
			if err := utils.ComparePasswords(user.Password, payload.Password); err == nil {
				authenticated = true
			}
		}
	}

	if !authenticated {
		s.logger.Warn("Неверный пароль или ошибка LDAP при входе", zap.String("login", loginInput))
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

// === ОБНОВЛЕННЫЙ МЕТОД GetUserByID ДЛЯ /auth/me ===
func (s *AuthService) GetUserByID(ctx context.Context, userID uint64) (*dto.UserProfileDTO, error) {
	// 1. Базовые данные из User Repo (он джойнит таблицы имен Branch/Otdel)
	user, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	// 2. Получаем доп. списки (Роли, Отделы, Должности)
	roles, err := s.userRepo.GetRolesByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("GetUserByID: Roles failed", zap.Error(err))
	}
	roleIDs := make([]uint64, 0, len(roles))
	for _, r := range roles {
		roleIDs = append(roleIDs, r.ID)
	}

	positionIDs, err := s.userRepo.GetPositionIDsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("GetUserByID: Positions failed", zap.Error(err))
		positionIDs = []uint64{}
	}

	otdelIDs, err := s.userRepo.GetOtdelIDsByUserID(ctx, userID)
	if err != nil {
		s.logger.Error("GetUserByID: Otdels failed", zap.Error(err))
		otdelIDs = []uint64{}
	}

	// 3. Формируем ответ
	res := &dto.UserProfileDTO{
		ID:       user.ID,
		FIO:      user.Fio,
		Email:    user.Email,
		Phone:    user.PhoneNumber,
		Username: user.Username,
		PhotoURL: user.PhotoURL,
		StatusID: user.StatusID,
		IsHead:   safeBool(user.IsHead),

		// Основные ID
		BranchID:     user.BranchID,
		OfficeID:     user.OfficeID,
		DepartmentID: user.DepartmentID,
		OtdelID:      user.OtdelID,
		PositionID:   user.PositionID,

		// Названия (Repo возвращает их, если использовать правильный SELECT)
		// Используем хелперы для безопасного разыменования
		DepartmentName: safeString(user.DepartmentName),
		OtdelName:      user.OtdelName, // уже указатель
		PositionName:   safeString(user.PositionName),
		BranchName:     safeString(user.BranchName),
		OfficeName:     user.OfficeName, // уже указатель

		// Массивы
		RoleIDs:     roleIDs,
		PositionIDs: positionIDs,
		OtdelIDs:    otdelIDs,
	}

	return res, nil
}

func (s *AuthService) UpdateMyProfile(ctx context.Context, payload dto.UpdateMyProfileDTO) (*dto.UserDTO, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	currentUser, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var oldPhotoURL *string
	if currentUser.PhotoURL != nil {
		photoCopy := *currentUser.PhotoURL
		oldPhotoURL = &photoCopy
	}

	updatedUser := currentUser
	err = s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		userEntity, err := s.userRepo.FindUserByIDInTx(ctx, tx, userID)
		if err != nil {
			return err
		}

		if payload.PhotoURL != nil {
			if *payload.PhotoURL == "SET_NULL" {
				userEntity.PhotoURL = nil
			} else {
				userEntity.PhotoURL = payload.PhotoURL
			}
		}
		if payload.Fio != nil {
			userEntity.Fio = *payload.Fio
		}
		if payload.PhoneNumber != nil {
			userEntity.PhoneNumber = *payload.PhoneNumber
		}
		if payload.Email != nil {
			userEntity.Email = *payload.Email
		}

		if err := s.userRepo.UpdateUser(ctx, tx, userEntity); err != nil {
			return err
		}

		updatedUser = userEntity
		return nil
	})
	if err != nil {
		return nil, err
	}

	if shouldDeleteOldPhoto(oldPhotoURL, updatedUser.PhotoURL) {
		if err := s.fileStorage.Delete(*oldPhotoURL); err != nil {
			s.logger.Warn("Не удалось удалить старое фото профиля", zap.Uint64("user_id", userID), zap.String("photo_url", *oldPhotoURL), zap.Error(err))
		}
	}

	return &dto.UserDTO{
		ID:           updatedUser.ID,
		Fio:          updatedUser.Fio,
		Email:        updatedUser.Email,
		PhoneNumber:  updatedUser.PhoneNumber,
		Username:     updatedUser.Username,
		StatusID:     updatedUser.StatusID,
		PositionID:   updatedUser.PositionID,
		BranchID:     updatedUser.BranchID,
		DepartmentID: updatedUser.DepartmentID,
		OfficeID:     updatedUser.OfficeID,
		OtdelID:      updatedUser.OtdelID,
		PhotoURL:     updatedUser.PhotoURL,
		IsHead:       safeBool(updatedUser.IsHead),
	}, nil
}

func shouldDeleteOldPhoto(oldPhotoURL *string, newPhotoURL *string) bool {
	if oldPhotoURL == nil || *oldPhotoURL == "" {
		return false
	}
	if newPhotoURL == nil {
		return true
	}
	return *oldPhotoURL != *newPhotoURL
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, payload dto.ResetPasswordRequestDTO) error {
	loginInput := strings.ToLower(payload.Login)
	user, _ := s.userRepo.FindUserByEmailOrLogin(ctx, loginInput)
	if user == nil {
		return nil
	}

	resetCode := fmt.Sprintf("%04d", rand.Intn(10000))
	s.cacheRepo.Set(ctx, fmt.Sprintf(constants.CacheKeyResetPhoneCode, loginInput), resetCode, time.Minute*15)

	if user.TelegramChatID.Valid && user.TelegramChatID.Int64 != 0 {
		_ = s.notifySvc.SendPlainMessage(ctx, user.TelegramChatID.Int64, "Код: "+resetCode)
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
	var userIDStr string
	var err error
	var isForceChange bool

	userIDStr, err = s.cacheRepo.Get(ctx, fmt.Sprintf(constants.CacheKeyVerifyPhone, payload.Token))
	if err != nil {
		userIDStr, err = s.cacheRepo.Get(ctx, fmt.Sprintf(constants.CacheKeyForceChangeToken, payload.Token))
		if err == nil {
			isForceChange = true
		}
	}
	if err != nil {
		return apperrors.ErrInvalidCredentials
	}

	parsedID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return apperrors.ErrInvalidCredentials
	}
	hashedPassword, err := utils.HashPassword(payload.NewPassword)
	if err != nil {
		return apperrors.ErrInternal
	}

	if isForceChange {
		err = s.userRepo.UpdatePasswordAndClearFlag(ctx, parsedID, hashedPassword)
		s.cacheRepo.Del(ctx, fmt.Sprintf(constants.CacheKeyForceChangeToken, payload.Token))
	} else {
		err = s.userRepo.UpdatePassword(ctx, parsedID, hashedPassword)
		s.cacheRepo.Del(ctx, fmt.Sprintf(constants.CacheKeyVerifyPhone, payload.Token))
	}
	return err
}

func safeString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
func safeBool(ptr *bool) bool {
	if ptr == nil {
		return false
	}
	return *ptr
}

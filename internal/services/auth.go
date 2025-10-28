// Файл: internal/services/auth_service.go
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

	"request-system/pkg/config"
	"request-system/pkg/constants"
	apperrors "request-system/pkg/errors"

	"request-system/pkg/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)

type AuthServiceInterface interface {
	Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error)
	GetUserByID(ctx context.Context, userID uint64) (*dto.UserProfileDTO, error)
	RequestPasswordReset(ctx context.Context, payload dto.ResetPasswordRequestDTO) error
	VerifyResetCode(ctx context.Context, payload dto.VerifyCodeDTO) (*dto.VerifyCodeResponseDTO, error)
	ResetPassword(ctx context.Context, payload dto.ResetPasswordDTO) error
	UpdateMyProfile(ctx context.Context, payload dto.UpdateMyProfileDTO) (*dto.UserDTO, error)
}

type AuthService struct {
	userRepo          repositories.UserRepositoryInterface
	cacheRepo         repositories.CacheRepositoryInterface
	logger            *zap.Logger
	cfg               *config.AuthConfig
	notifySvc         NotificationServiceInterface
	positionService   PositionServiceInterface
	branchService     BranchServiceInterface
	departmentService DepartmentServiceInterface
	otdelService      OtdelServiceInterface
	officeService     OfficeServiceInterface
}

func NewAuthService(
	userRepo repositories.UserRepositoryInterface,
	cacheRepo repositories.CacheRepositoryInterface,
	logger *zap.Logger,
	cfg *config.AuthConfig,
	notifySvc NotificationServiceInterface,
	positionService PositionServiceInterface,
	branchService BranchServiceInterface,
	departmentService DepartmentServiceInterface,
	otdelService OtdelServiceInterface,
	officeService OfficeServiceInterface,
) AuthServiceInterface {
	return &AuthService{
		userRepo:          userRepo,
		cacheRepo:         cacheRepo,
		logger:            logger,
		cfg:               cfg,
		notifySvc:         notifySvc,
		positionService:   positionService,
		branchService:     branchService,
		departmentService: departmentService,
		otdelService:      otdelService,
		officeService:     officeService,
	}
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, payload dto.ResetPasswordRequestDTO) error {
	loginInput := strings.ToLower(payload.Login)
	logger := s.logger.With(zap.String("login_input", loginInput))
	spamProtectionKey := fmt.Sprintf(constants.CacheKeySpamProtect, loginInput)
	if _, err := s.cacheRepo.Get(ctx, spamProtectionKey); err == nil {
		logger.Warn("Слишком частые запросы на сброс пароля")
		return apperrors.NewHttpError(http.StatusTooManyRequests, "Запрашивать код можно не чаще одного раза в минуту", nil, nil)
	}
	s.cacheRepo.Set(ctx, spamProtectionKey, "active", time.Minute)

	var user *entities.User
	var err error

	isEmail := emailRegex.MatchString(loginInput)
	normalizedPhone := utils.NormalizeTajikPhoneNumber(loginInput)

	if isEmail {
		logger.Debug("Ввод распознан как Email, ищем по email.")
		user, err = s.userRepo.FindUserByEmailOrLogin(ctx, loginInput)
	} else if normalizedPhone != "" {
		// Если нормализация удалась, ищем по стандартизированному номеру
		logger.Debug("Ввод распознан как телефон, ищем по нормализованному номеру.", zap.String("normalized_phone", normalizedPhone))
		user, err = s.userRepo.FindUserByPhone(ctx, normalizedPhone)
	} else {
		// Если это ни email, ни валидный телефон, тихо выходим
		logger.Warn("Логин не является ни email, ни телефоном. Тихо выходим.")
		return nil
	}
	// <<<--- КОНЕЦ ГЛАВНЫХ ИЗМЕНЕНИЙ ---

	// Если пользователь не найден, тихо выходим. Блокировка от спама уже стоит.
	if err != nil || user == nil {
		logger.Warn("Попытка сброса пароля для несуществующего логина.")
		return nil
	}

	// Шаг 3: Генерируем код/токен и ОТПРАВЛЯЕМ его
	if isEmail {
		resetToken := uuid.New().String()
		cacheKey := fmt.Sprintf(constants.CacheKeyResetEmail, resetToken)
		s.cacheRepo.Set(ctx, cacheKey, user.ID, s.cfg.ResetTokenTTL)

		if err := s.notifySvc.SendPasswordResetEmail(user.Email, resetToken); err != nil {
			s.logger.Error("Не удалось отправить email для сброса пароля", zap.Error(err))
		}
	} else { // Телефон
		resetCode := fmt.Sprintf("%04d", rand.Intn(10000))
		// Для ключа Redis используем оригинальный ввод, т.к. пользователь будет вводить его же для верификации
		cacheKey := fmt.Sprintf(constants.CacheKeyResetPhoneCode, loginInput)
		s.cacheRepo.Set(ctx, cacheKey, resetCode, s.cfg.VerificationCodeTTL)

		if err := s.notifySvc.SendPasswordResetSMS(user.PhoneNumber, resetCode); err != nil {
			s.logger.Error("Не удалось отправить SMS для сброса пароля", zap.Error(err))
		}
	}
	return nil
}

func (s *AuthService) Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error) {
	loginInput := strings.ToLower(payload.Login)

	var user *entities.User
	var err error

	if emailRegex.MatchString(loginInput) {
		user, err = s.userRepo.FindUserByEmailOrLogin(ctx, loginInput)
	} else {

		normalizedPhone := utils.NormalizeTajikPhoneNumber(loginInput)
		if normalizedPhone != "" {
			user, err = s.userRepo.FindUserByPhone(ctx, normalizedPhone)
		} else {
			user, err = s.userRepo.FindUserByEmailOrLogin(ctx, loginInput)
		}
	}

	// Если пользователь не найден ни одним из способов, возвращаем ошибку.
	if err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}

	// Весь остальной код остается БЕЗ ИЗМЕНЕНИЙ
	if err := s.checkLockout(ctx, user.ID); err != nil {
		return nil, err
	}
	if user.StatusCode != constants.UserStatusActiveCode {
		s.handleFailedLoginAttempt(ctx, user.ID)
		return nil, apperrors.ErrInvalidCredentials
	}
	if err := utils.ComparePasswords(user.Password, payload.Password); err != nil {
		s.handleFailedLoginAttempt(ctx, user.ID)
		return nil, apperrors.ErrInvalidCredentials
	}
	if user.MustChangePassword {
		resetToken := uuid.New().String()
		cacheKey := fmt.Sprintf(constants.CacheKeyForceChangeToken, resetToken)
		if err := s.cacheRepo.Set(ctx, cacheKey, user.ID, 5*time.Minute); err != nil {
			return nil, apperrors.ErrInternalServer
		}
		responseDTO := dto.ChangePasswordRequiredDTO{
			ResetToken: resetToken,
			Message:    "Необходимо установить новый пароль для завершения входа.",
		}
		errWithDetails := apperrors.ErrChangePasswordWithToken
		errWithDetails.Details = responseDTO
		return nil, errWithDetails
	}
	s.resetLoginAttempts(ctx, user.ID)
	return user, nil
}

func (s *AuthService) ResetPassword(ctx context.Context, payload dto.ResetPasswordDTO) error {
	var userIDStr string
	var err error
	var isForcedChange bool

	cacheKeyForce := fmt.Sprintf(constants.CacheKeyForceChangeToken, payload.Token)
	userIDStr, err = s.cacheRepo.Get(ctx, cacheKeyForce)
	if err == nil {
		s.cacheRepo.Del(ctx, cacheKeyForce)
		isForcedChange = true
	} else {
		cacheKeyEmail := fmt.Sprintf(constants.CacheKeyResetEmail, payload.Token)
		userIDStr, err = s.cacheRepo.Get(ctx, cacheKeyEmail)
		if err == nil {
			s.cacheRepo.Del(ctx, cacheKeyEmail)
		} else {
			cacheKeyPhone := fmt.Sprintf(constants.CacheKeyVerifyPhone, payload.Token)
			userIDStr, err = s.cacheRepo.Get(ctx, cacheKeyPhone)
			if err != nil {
				return apperrors.NewHttpError(http.StatusBadRequest, "Неверный или истекший токен", err, nil)
			}
			s.cacheRepo.Del(ctx, cacheKeyPhone)
		}
	}

	parsedID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil || parsedID == 0 {
		return apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка получения ID пользователя из кэша", err, nil)
	}

	hashedPassword, err := utils.HashPassword(payload.NewPassword)
	if err != nil {
		return apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка хэширования нового пароля", err, nil)
	}

	if isForcedChange {
		if err := s.userRepo.UpdatePasswordAndClearFlag(ctx, parsedID, hashedPassword); err != nil {
			return apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка обновления пароля и сброса флага", err, nil)
		}
	} else {
		if err := s.userRepo.UpdatePassword(ctx, parsedID, hashedPassword); err != nil {
			return apperrors.NewHttpError(http.StatusInternalServerError, "Ошибка обновления пароля", err, nil)
		}
	}
	return nil
}

func (s *AuthService) GetUserByID(ctx context.Context, userID uint64) (*dto.UserProfileDTO, error) {
	logger := s.logger.With(zap.Uint64("userID", userID))

	// Шаг 1: Получаем основную информацию о пользователе
	user, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		logger.Error("Не удалось найти пользователя по ID", zap.Error(err))
		return nil, apperrors.ErrUserNotFound
	}

	// Шаг 2: Создаем базовую структуру ответа
	response := &dto.UserProfileDTO{
		ID:           user.ID,
		Email:        user.Email,
		Phone:        user.PhoneNumber,
		FIO:          user.Fio,
		PhotoURL:     user.PhotoURL,
		DepartmentID: user.DepartmentID, // Сразу присваиваем ID департамента
	}

	// Шаг 3: Получаем имена для связанных сущностей

	// --- Получаем Департамент ---
	if user.DepartmentID > 0 {
		if depDTO, err := s.departmentService.FindDepartment(ctx, user.DepartmentID); err == nil {
			response.DepartmentName = depDTO.Name
		} else {
			logger.Warn("Не удалось получить имя департамента", zap.Error(err))
		}
	}

	// --- Получаем Отдел (если есть) ---
	if user.OtdelID != nil && *user.OtdelID > 0 {
		if otdelDTO, err := s.otdelService.FindOtdel(ctx, *user.OtdelID); err == nil {
			response.OtdelName = &otdelDTO.Name // Присваиваем указатель на имя
		} else {
			logger.Warn("Не удалось получить имя отдела", zap.Error(err))
		}
	}

	// --- Получаем Должность ---
	if user.PositionID != nil {
		if posDTO, err := s.positionService.GetByID(ctx, uint64(*user.PositionID)); err == nil {
			response.PositionName = posDTO.Name
		} else {
			logger.Warn("Не удалось получить имя должности", zap.Error(err))
		}
	}

	if user.BranchID != nil {
		if branchDTO, err := s.branchService.FindBranch(ctx, *user.BranchID); err == nil {
			response.BranchName = branchDTO.Name
		} else {
			logger.Warn("Не удалось получить имя филиала", zap.Error(err))
		}
	}

	// --- Получаем Офис (если есть) ---
	if user.OfficeID != nil && *user.OfficeID > 0 {
		if officeDTO, err := s.officeService.FindOffice(ctx, *user.OfficeID); err == nil {
			response.OfficeName = &officeDTO.Name // Присваиваем указатель на имя
		} else {
			logger.Warn("Не удалось получить имя офиса", zap.Error(err))
		}
	}

	return response, nil
}

func (s *AuthService) VerifyResetCode(ctx context.Context, payload dto.VerifyCodeDTO) (*dto.VerifyCodeResponseDTO, error) {
	loginInput := payload.Login
	logger := s.logger.With(zap.String("login_input", loginInput))

	attemptsKey := fmt.Sprintf(constants.CacheKeyResetAttempts, loginInput)
	if attemptsStr, err := s.cacheRepo.Get(ctx, attemptsKey); err == nil {
		if attempts, _ := strconv.Atoi(attemptsStr); attempts >= s.cfg.MaxResetAttempts {
			return nil, apperrors.NewHttpError(http.StatusTooManyRequests, fmt.Sprintf("Слишком много попыток. Попробуйте через %.0f минут.", s.cfg.LockoutDuration.Minutes()), nil, nil)
		}
	}

	cacheKeyCode := fmt.Sprintf(constants.CacheKeyResetPhoneCode, loginInput)
	storedCode, err := s.cacheRepo.Get(ctx, cacheKeyCode)
	if err != nil || storedCode != payload.Code {
		s.cacheRepo.Incr(ctx, attemptsKey)
		s.cacheRepo.Expire(ctx, attemptsKey, s.cfg.LockoutDuration)
		logger.Warn("Неверный или истекший код верификации", zap.Error(err))
		return nil, apperrors.NewHttpError(http.StatusBadRequest, "Неверный или истекший код верификации", err, nil)
	}

	// <<<--- НАЧАЛО ИСПРАВЛЕНИЙ ---
	// Теперь мы нормализуем номер ПЕРЕД ПОИСКОМ пользователя в базе.
	// Это решает проблему "404 User Not Found".
	normalizedPhone := utils.NormalizeTajikPhoneNumber(loginInput)
	if normalizedPhone == "" {
		// Если ввод нельзя нормализовать, значит, это невалидный номер
		logger.Warn("Не удалось нормализовать номер телефона при верификации")
		return nil, apperrors.ErrUserNotFound
	}

	user, err := s.userRepo.FindUserByPhone(ctx, normalizedPhone)
	if err != nil {
		logger.Error("Не удалось найти пользователя по нормализованному номеру", zap.String("normalized_phone", normalizedPhone), zap.Error(err))
		return nil, apperrors.ErrUserNotFound
	}
	// <<<--- КОНЕЦ ИСПРАВЛЕНИЙ ---

	// Создание верификационного токена для шага /reset (остается без изменений)
	verificationToken := uuid.New().String()
	cacheKeyVerify := fmt.Sprintf(constants.CacheKeyVerifyPhone, verificationToken)
	if err := s.cacheRepo.Set(ctx, cacheKeyVerify, user.ID, s.cfg.VerificationCodeTTL); err != nil {
		return nil, apperrors.ErrInternalServer
	}

	// Удаляем использованный код и счетчик попыток
	s.cacheRepo.Del(ctx, cacheKeyCode, attemptsKey)

	return &dto.VerifyCodeResponseDTO{VerificationToken: verificationToken}, nil
}

func (s *AuthService) checkLockout(ctx context.Context, userID uint64) error {
	lockoutKey := fmt.Sprintf(constants.CacheKeyLockout, userID)
	if _, err := s.cacheRepo.Get(ctx, lockoutKey); err == nil {
		return apperrors.ErrAccountLocked
	}
	return nil
}

func (s *AuthService) handleFailedLoginAttempt(ctx context.Context, userID uint64) {
	attemptsKey := fmt.Sprintf(constants.CacheKeyLoginAttempts, userID)
	attempts, _ := s.cacheRepo.Incr(ctx, attemptsKey)
	if attempts >= int64(s.cfg.MaxLoginAttempts) {
		lockoutKey := fmt.Sprintf(constants.CacheKeyLockout, userID)
		s.cacheRepo.Set(ctx, lockoutKey, "locked", s.cfg.LockoutDuration)
		s.cacheRepo.Del(ctx, attemptsKey)
	}
}

func (s *AuthService) resetLoginAttempts(ctx context.Context, userID uint64) {
	attemptsKey := fmt.Sprintf(constants.CacheKeyLoginAttempts, userID)
	lockoutKey := fmt.Sprintf(constants.CacheKeyLockout, userID)
	s.cacheRepo.Del(ctx, attemptsKey, lockoutKey)
}

func (s *AuthService) UpdateMyProfile(ctx context.Context, payload dto.UpdateMyProfileDTO) (*dto.UserDTO, error) {
	userID, err := utils.GetUserIDFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	permissionsMap, err := utils.GetPermissionsMapFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if _, hasPermission := permissionsMap[authz.ProfileUpdate]; !hasPermission {
		return nil, apperrors.ErrForbidden
	}

	// Шаг 2: Выполняем обновление через репозиторий
	updatePayload := dto.UpdateUserDTO{
		ID:          userID,
		Fio:         payload.Fio,
		PhoneNumber: payload.PhoneNumber,
		Email:       payload.Email,
		PhotoURL:    payload.PhotoURL,
	}

	tx, err := s.userRepo.BeginTx(ctx)
	if err != nil {
		s.logger.Error("UpdateMyProfile: не удалось начать транзакцию", zap.Error(err))
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := s.userRepo.UpdateUser(ctx, tx, updatePayload); err != nil {
		s.logger.Error("UpdateMyProfile: ошибка при вызове userRepo.UpdateUser", zap.Uint64("userID", userID), zap.Error(err))
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("UpdateMyProfile: не удалось закоммитить транзакцию", zap.Error(err))
		return nil, err
	}

	// Шаг 3: Возвращаем обновленные данные
	updatedUser, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	roles, _ := s.userRepo.GetRolesByUserID(ctx, userID)
	roleIDs := make([]uint64, len(roles))
	for i, r := range roles {
		roleIDs[i] = r.ID
	}

	userDTO := userEntityToUserDTO(updatedUser)
	userDTO.RoleIDs = roleIDs

	return userDTO, nil
}

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

// <<<--- ИНТЕРФЕЙС ОСТАЕТСЯ ПРЕЖНИМ ---
type AuthServiceInterface interface {
	Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error)
	GetUserByID(ctx context.Context, userID uint64) (*entities.User, error)
	RequestPasswordReset(ctx context.Context, payload dto.ResetPasswordRequestDTO) error
	VerifyResetCode(ctx context.Context, payload dto.VerifyCodeDTO) (*dto.VerifyCodeResponseDTO, error)
	ResetPassword(ctx context.Context, payload dto.ResetPasswordDTO) error
}

// <<<--- СТРУКТУРА ТЕПЕРЬ ВКЛЮЧАЕТ СЕРВИС УВЕДОМЛЕНИЙ ---
type AuthService struct {
	userRepo  repositories.UserRepositoryInterface
	cacheRepo repositories.CacheRepositoryInterface
	logger    *zap.Logger
	cfg       *config.AuthConfig
	notifySvc NotificationServiceInterface // <--- ДОБАВЛЕНО
}

// <<<--- КОНСТРУКТОР ТЕПЕРЬ ПРИНИМАЕТ СЕРВИС УВЕДОМЛЕНИЙ ---
func NewAuthService(
	userRepo repositories.UserRepositoryInterface,
	cacheRepo repositories.CacheRepositoryInterface,
	logger *zap.Logger,
	cfg *config.AuthConfig,
	notifySvc NotificationServiceInterface, // <--- ДОБАВЛЕНО
) AuthServiceInterface {
	return &AuthService{
		userRepo:  userRepo,
		cacheRepo: cacheRepo,
		logger:    logger,
		cfg:       cfg,
		notifySvc: notifySvc, // <--- ДОБАВЛЕНО
	}
}

// <<<--- ЭТО ПОЛНОСТЬЮ ИСПРАВЛЕННАЯ ФУНКЦИЯ ---
func (s *AuthService) RequestPasswordReset(ctx context.Context, payload dto.ResetPasswordRequestDTO) error {
	loginInput := strings.ToLower(payload.Login)
	logger := s.logger.With(zap.String("login_input", loginInput))

	// Шаг 1: СРАЗУ ставим защиту от спама на основе оригинального ввода
	spamProtectionKey := fmt.Sprintf(constants.CacheKeySpamProtect, loginInput)
	if _, err := s.cacheRepo.Get(ctx, spamProtectionKey); err == nil {
		logger.Warn("Слишком частые запросы на сброс пароля")
		return apperrors.NewHttpError(http.StatusTooManyRequests, "Запрашивать код можно не чаще одного раза в минуту", nil, nil)
	}
	s.cacheRepo.Set(ctx, spamProtectionKey, "active", time.Minute)

	// <<<--- НАЧАЛО ГЛАВНЫХ ИЗМЕНЕНИЙ ---
	var user *entities.User
	var err error

	isEmail := emailRegex.MatchString(loginInput)
	// Пытаемся нормализовать ввод как телефонный номер
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

// --- ОСТАЛЬНЫЕ ФУНКЦИИ (Login, VerifyResetCode, ResetPassword и т.д.) ОСТАЮТСЯ БЕЗ ИЗМЕНЕНИЙ ---
// Здесь я их привожу, чтобы вы могли просто заменить весь файл целиком.

func (s *AuthService) Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error) {
	loginInput := strings.ToLower(payload.Login)

	// <<<--- НАЧАЛО НОВОЙ ЛОГИКИ ПОИСКА ---
	var user *entities.User
	var err error

	// Проверяем, это email, или что-то, что может быть телефоном.
	if emailRegex.MatchString(loginInput) {
		// Если это email, ищем как и раньше.
		user, err = s.userRepo.FindUserByEmailOrLogin(ctx, loginInput)
	} else {
		// А если это не email, ПРИНУДИТЕЛЬНО нормализуем ввод как телефонный номер.
		normalizedPhone := utils.NormalizeTajikPhoneNumber(loginInput)
		if normalizedPhone != "" {
			// Если нормализация удалась, ищем пользователя в базе по ЧИСТОМУ номеру.
			user, err = s.userRepo.FindUserByPhone(ctx, normalizedPhone)
		} else {
			// Если это и не email, и не распознаваемый телефон, тогда это просто логин.
			// Пытаемся найти по нему как есть (для обратной совместимости).
			user, err = s.userRepo.FindUserByEmailOrLogin(ctx, loginInput)
		}
	}
	// <<<--- КОНЕЦ НОВОЙ ЛОГИКИ ПОИСКА ---

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

func (s *AuthService) GetUserByID(ctx context.Context, userID uint64) (*entities.User, error) {
	user, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}
	return user, nil
}

func (s *AuthService) VerifyResetCode(ctx context.Context, payload dto.VerifyCodeDTO) (*dto.VerifyCodeResponseDTO, error) {
	// Для подсчета попыток и поиска ключа в Redis используем ОРИГИНАЛЬНЫЙ ВВОД,
	// так как именно он использовался при создании ключа на шаге /request.
	loginInput := payload.Login
	logger := s.logger.With(zap.String("login_input", loginInput))

	// Проверка на блокировку из-за частых неудачных попыток (остается без изменений)
	attemptsKey := fmt.Sprintf(constants.CacheKeyResetAttempts, loginInput)
	if attemptsStr, err := s.cacheRepo.Get(ctx, attemptsKey); err == nil {
		if attempts, _ := strconv.Atoi(attemptsStr); attempts >= s.cfg.MaxResetAttempts {
			return nil, apperrors.NewHttpError(http.StatusTooManyRequests, fmt.Sprintf("Слишком много попыток. Попробуйте через %.0f минут.", s.cfg.LockoutDuration.Minutes()), nil, nil)
		}
	}

	// Проверяем код в Redis (остается без изменений)
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

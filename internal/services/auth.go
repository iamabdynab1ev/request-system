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
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)

type AuthServiceInterface interface {
	Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error)
	GetUserByID(ctx context.Context, userID uint64) (*entities.User, error)
	RequestPasswordReset(ctx context.Context, payload dto.ResetPasswordRequestDTO) error
	VerifyResetCode(ctx context.Context, payload dto.VerifyCodeDTO) (*dto.VerifyCodeResponseDTO, error)
	ResetPassword(ctx context.Context, payload dto.ResetPasswordDTO) error
}

type AuthService struct {
	userRepo  repositories.UserRepositoryInterface
	cacheRepo repositories.CacheRepositoryInterface
	logger    *zap.Logger
	cfg       *config.AuthConfig
}

func NewAuthService(
	userRepo repositories.UserRepositoryInterface,
	cacheRepo repositories.CacheRepositoryInterface,
	logger *zap.Logger,
	cfg *config.AuthConfig,
) AuthServiceInterface {
	return &AuthService{
		userRepo:  userRepo,
		cacheRepo: cacheRepo,
		logger:    logger,
		cfg:       cfg,
	}
}

// Файл: internal/services/auth_service.go

func (s *AuthService) RequestPasswordReset(ctx context.Context, payload dto.ResetPasswordRequestDTO) error {
	logger := s.logger.With(zap.String("login", payload.Login))

	// 1. ЗАЩИТА: Проверяем, не заблокирован ли пользователь за многочисленные попытки ввода кода.
	lockoutKey := fmt.Sprintf("reset_attempts:%s", payload.Login)
	attemptsStr, _ := s.cacheRepo.Get(ctx, lockoutKey)
	if attempts, _ := strconv.Atoi(attemptsStr); attempts >= 5 { // 5 можно вынести в конфиг
		logger.Warn("Попытка запросить код для заблокированного логина (слишком много попыток ввода)", zap.String("login", payload.Login))
		return apperrors.NewHttpError(http.StatusTooManyRequests, "Слишком много неверных попыток. Пожалуйста, подождите 15 минут.", nil)
	}

	// 2. ЗАЩИТА: Проверяем, не запрашивал ли пользователь код в последнюю минуту (защита от SMS/email спама).
	spamProtectionKey := fmt.Sprintf("reset_spam_protect:%s", payload.Login)
	if _, err := s.cacheRepo.Get(ctx, spamProtectionKey); err == nil {
		logger.Warn("Слишком частые запросы на сброс пароля (спам)", zap.String("login", payload.Login))
		return apperrors.NewHttpError(http.StatusTooManyRequests, "Запрашивать код можно не чаще одного раза в минуту.", nil)
	}

	// 3. ИЩЕМ ПОЛЬЗОВАТЕЛЯ: Проверяем формат и ищем в базе.
	var user *entities.User
	var err error
	isEmail := emailRegex.MatchString(payload.Login)
	var isPhone bool
	var normalizedPhone string

	phone := payload.Login
	if strings.HasPrefix(phone, "+992") && len(phone) == 13 {
		isPhone = true
		normalizedPhone = strings.TrimPrefix(phone, "+992")
	} else if !strings.HasPrefix(phone, "+") && len(phone) == 9 {
		if _, err := strconv.Atoi(phone); err == nil {
			isPhone = true
			normalizedPhone = phone
		}
	}

	if isEmail {
		user, err = s.userRepo.FindUserByEmailOrLogin(ctx, payload.Login)
	} else if isPhone {
		user, err = s.userRepo.FindUserByPhone(ctx, normalizedPhone)
	} else {
		logger.Warn("Некорректный формат email или  номера телефона")
		return nil // Тихо выходим, не сообщая, что формат неверный
	}

	if err != nil {
		logger.Warn("Попытка сброса пароля для несуществующего пользователя")
		return nil // Тихо выходим, не сообщая, что пользователь не найден
	}

	// 4. ГЕНЕРИРУЕМ И ОТПРАВЛЯЕМ КОД/ТОКЕН
	// Ставим "метку" для защиты от спама на 1 минуту ПОСЛЕ всех проверок
	s.cacheRepo.Set(ctx, spamProtectionKey, "active", time.Minute*1)

	if isEmail {
		resetToken := uuid.New().String()
		cacheKey := fmt.Sprintf("reset_email:%s", resetToken)
		s.cacheRepo.Set(ctx, cacheKey, user.ID, s.cfg.ResetTokenTTL)
		logger.Warn("!!! ВРЕМЕННОЕ РЕШЕНИЕ: Токен для сброса по EMAIL",
			zap.Uint64("userID", user.ID),
			zap.String("reset_token", resetToken),
		)
		// TODO: Отправить email
	} else { // isPhone
		resetCode := fmt.Sprintf("%04d", rand.Intn(10000))
		cacheKeyCode := fmt.Sprintf("reset_phone_code:%s", payload.Login)
		s.cacheRepo.Set(ctx, cacheKeyCode, resetCode, s.cfg.VerificationCodeTTL)
		logger.Warn("!!! ВРЕМЕННОЕ РЕШЕНИЕ: Код для сброса по ТЕЛЕФОНУ",
			zap.Uint64("userID", user.ID),
			zap.String("verification_code", resetCode),
		)
		// TODO: Отправить SMS
	}

	return nil
}

// Версия с защитой от перебора
func (s *AuthService) VerifyResetCode(ctx context.Context, payload dto.VerifyCodeDTO) (*dto.VerifyCodeResponseDTO, error) {
	logger := s.logger.With(zap.String("login", payload.Login))

	// 1. Проверяем, не заблокирован ли пользователь по попыткам
	attemptsKey := fmt.Sprintf("reset_attempts:%s", payload.Login)
	attemptsStr, _ := s.cacheRepo.Get(ctx, attemptsKey)
	attempts, _ := strconv.Atoi(attemptsStr)

	if attempts >= 5 { // Эту "5" можно вынести в config.Auth
		logger.Warn("Превышено количество попыток ввода кода сброса")
		return nil, apperrors.NewHttpError(http.StatusTooManyRequests, "Слишком много попыток. Попробуйте запросить новый код через 15 минут.", nil)
	}

	// 2. Проверяем сам код
	cacheKeyCode := fmt.Sprintf("reset_phone_code:%s", payload.Login)
	storedCode, err := s.cacheRepo.Get(ctx, cacheKeyCode)

	if err != nil || storedCode != payload.Code {
		logger.Warn("Неверный или истекший код верификации телефона")

		// Увеличиваем счетчик неудачных попыток и ставим TTL
		s.cacheRepo.Incr(ctx, attemptsKey)
		s.cacheRepo.Expire(ctx, attemptsKey, time.Minute*15)

		return nil, apperrors.ErrInvalidVerificationCode
	}

	// 3. Код верный - находим пользователя
	normalizedPhone := strings.TrimPrefix(payload.Login, "+992")
	user, err := s.userRepo.FindUserByPhone(ctx, normalizedPhone)
	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	// 4. Генерируем токен-"пропуск"
	verificationToken := uuid.New().String()
	cacheKeyVerify := fmt.Sprintf("verify_token_phone:%s", verificationToken)
	if err := s.cacheRepo.Set(ctx, cacheKeyVerify, user.ID, s.cfg.VerificationCodeTTL); err != nil {
		return nil, apperrors.ErrInternalServer
	}

	// 5. Удаляем использованный код и счетчик попыток
	s.cacheRepo.Del(ctx, cacheKeyCode, attemptsKey)

	logger.Info("Код верификации телефона подтвержден, выдан токен-пропуск")
	return &dto.VerifyCodeResponseDTO{VerificationToken: verificationToken}, nil
}

func (s *AuthService) ResetPassword(ctx context.Context, payload dto.ResetPasswordDTO) error {
	var userID uint64
	var userIDStr string
	var err error

	cacheKeyEmail := fmt.Sprintf("reset_email:%s", payload.Token)
	userIDStr, err = s.cacheRepo.Get(ctx, cacheKeyEmail)
	if err == nil {
		s.cacheRepo.Del(ctx, cacheKeyEmail)
	} else {
		cacheKeyPhone := fmt.Sprintf("verify_token_phone:%s", payload.Token)
		userIDStr, err = s.cacheRepo.Get(ctx, cacheKeyPhone)
		if err != nil {
			return apperrors.ErrInvalidResetToken
		}
		s.cacheRepo.Del(ctx, cacheKeyPhone)
	}

	parsedID, _ := strconv.ParseUint(userIDStr, 10, 64)
	if parsedID == 0 {
		return apperrors.ErrInternalServer
	}
	userID = parsedID

	hashedPassword, err := utils.HashPassword(payload.NewPassword)
	if err != nil {
		return apperrors.ErrInternalServer
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, hashedPassword); err != nil {
		return apperrors.ErrInternalServer
	}

	s.logger.Info("Пароль для пользователя успешно сброшен", zap.Uint64("userID", userID))
	return nil
}

func (s *AuthService) Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error) {
	user, err := s.userRepo.FindUserByEmailOrLogin(ctx, payload.Login)
	if err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}
	if err := s.checkLockout(ctx, user.ID); err != nil {
		return nil, err
	}
	if err := utils.ComparePasswords(user.Password, payload.Password); err != nil {
		s.handleFailedLoginAttempt(ctx, user.ID)
		return nil, apperrors.ErrInvalidCredentials
	}
	s.resetLoginAttempts(ctx, user.ID)
	return user, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, userID uint64) (*entities.User, error) {
	user, err := s.userRepo.FindUserByID(ctx, userID)
	if err != nil {
		s.logger.Warn("GetUserByID: не удалось найти пользователя", zap.Uint64("userID", userID), zap.Error(err))
		return nil, apperrors.ErrUserNotFound
	}
	return user, nil
}

func (s *AuthService) checkLockout(ctx context.Context, userID uint64) error {
	lockoutKey := fmt.Sprintf("lockout:%d", userID)
	_, err := s.cacheRepo.Get(ctx, lockoutKey)
	if err == nil {
		return apperrors.ErrAccountLocked
	}
	return nil
}

func (s *AuthService) handleFailedLoginAttempt(ctx context.Context, userID uint64) {
	attemptsKey := fmt.Sprintf("login_attempts:%d", userID)
	attempts, _ := s.cacheRepo.Incr(ctx, attemptsKey)
	if attempts >= int64(s.cfg.MaxLoginAttempts) {
		lockoutKey := fmt.Sprintf("lockout:%d", userID)
		s.cacheRepo.Set(ctx, lockoutKey, "locked", s.cfg.LockoutDuration)
		s.cacheRepo.Del(ctx, attemptsKey)
	}
}

func (s *AuthService) resetLoginAttempts(ctx context.Context, userID uint64) {
	attemptsKey := fmt.Sprintf("login_attempts:%d", userID)
	lockoutKey := fmt.Sprintf("lockout:%d", userID)
	s.cacheRepo.Del(ctx, attemptsKey, lockoutKey)
}

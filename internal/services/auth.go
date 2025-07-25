// Package services реализует бизнес-логику приложения.
package services

import (
	"context"
	"fmt"
	"math/rand"
	"request-system/internal/dto"
	"request-system/internal/entities"
	"request-system/internal/repositories"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	maxLoginAttempts      = 100
	lockoutDuration       = 100 * time.Minute
	verificationCodeTTL   = 100 * time.Minute
	resetPasswordTokenTTL = 100 * time.Minute
	resetCodeAttemptsTTL  = 100 * time.Minute
	maxResetCodeAttempts  = 100
)

type AuthServiceInterface interface {
	Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error)
	SendVerificationCode(ctx context.Context, payload dto.SendCodeDTO) error
	LoginWithCode(ctx context.Context, payload dto.VerifyCodeDTO) (*entities.User, error)
	GetUserByID(ctx context.Context, userID uint64) (*entities.User, error)
	CheckRecoveryOptions(ctx context.Context, payload dto.ForgotPasswordInitDTO) (*dto.ForgotPasswordOptionsDTO, error)
	SendRecoveryInstructions(ctx context.Context, payload dto.ForgotPasswordSendDTO) error
	ResetPasswordWithEmail(ctx context.Context, payload dto.ResetPasswordEmailDTO) error
	ResetPasswordWithPhone(ctx context.Context, payload dto.ResetPasswordPhoneDTO) error
}

type AuthService struct {
	userRepo  repositories.UserRepositoryInterface
	cacheRepo repositories.CacheRepositoryInterface
	logger    *zap.Logger
}

func NewAuthService(
	userRepo repositories.UserRepositoryInterface,
	cacheRepo repositories.CacheRepositoryInterface,
	logger *zap.Logger,
) AuthServiceInterface {
	return &AuthService{
		userRepo:  userRepo,
		cacheRepo: cacheRepo,
		logger:    logger,
	}
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

func (s *AuthService) SendVerificationCode(ctx context.Context, payload dto.SendCodeDTO) error {
	var user *entities.User
	var err error
	var contact string

	if payload.Phone != "" {
		contact = payload.Phone
		user, err = s.userRepo.FindUserByPhone(ctx, payload.Phone)
	} else {
		contact = payload.Email
		user, err = s.userRepo.FindUserByEmailOrLogin(ctx, payload.Email)
	}

	if err != nil {
		s.logger.Warn("Попытка запроса кода для несуществующего пользователя", zap.String("contact", contact))
		return nil
	}

	code := fmt.Sprintf("%04d", rand.Intn(10000))
	cacheKey := fmt.Sprintf("verify_code:%d", user.ID)
	if err := s.cacheRepo.Set(ctx, cacheKey, code, verificationCodeTTL); err != nil {
		s.logger.Error("Не удалось сохранить код верификации в кеш", zap.Error(err))
		return apperrors.ErrInternalServer
	}

	s.logger.Info("Код верификации сгенерирован (для теста)", zap.Uint64("userID", user.ID), zap.String("code", code))

	return nil
}

func (s *AuthService) LoginWithCode(ctx context.Context, payload dto.VerifyCodeDTO) (*entities.User, error) {
	var user *entities.User
	var err error

	if payload.Phone != "" {
		user, err = s.userRepo.FindUserByPhone(ctx, payload.Phone)
	} else {
		user, err = s.userRepo.FindUserByEmailOrLogin(ctx, payload.Email)
	}

	if err != nil {
		return nil, apperrors.ErrUserNotFound
	}

	if err = s.checkLockout(ctx, user.ID); err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("verify_code:%d", user.ID)
	storedCode, err := s.cacheRepo.Get(ctx, cacheKey)
	if err != nil || storedCode != payload.Code {
		s.handleFailedLoginAttempt(ctx, user.ID)
		return nil, apperrors.ErrInvalidVerificationCode
	}

	s.resetLoginAttempts(ctx, user.ID)
	s.cacheRepo.Del(ctx, cacheKey)

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

	if attempts >= maxLoginAttempts {
		lockoutKey := fmt.Sprintf("lockout:%d", userID)
		s.cacheRepo.Set(ctx, lockoutKey, "locked", lockoutDuration)
		s.cacheRepo.Del(ctx, attemptsKey)
	}
}

func (s *AuthService) resetLoginAttempts(ctx context.Context, userID uint64) {
	attemptsKey := fmt.Sprintf("login_attempts:%d", userID)
	lockoutKey := fmt.Sprintf("lockout:%d", userID)
	s.cacheRepo.Del(ctx, attemptsKey, lockoutKey)
}
func (s *AuthService) CheckRecoveryOptions(ctx context.Context, payload dto.ForgotPasswordInitDTO) (*dto.ForgotPasswordOptionsDTO, error) {
	logger := s.logger.With(zap.String("email", payload.Email))
	logger.Info("Проверка опций восстановления пароля")

	user, err := s.userRepo.FindUserByEmailOrLogin(ctx, payload.Email)
	if err != nil {
		logger.Warn("Попытка проверки опций для несуществующего пользователя")
		return &dto.ForgotPasswordOptionsDTO{Options: []string{}}, nil
	}

	options := []string{"email"}
	if user.PhoneNumber != "" {
		options = append(options, "phone")
	}

	logger.Info("Опции восстановления найдены", zap.Strings("options", options))
	return &dto.ForgotPasswordOptionsDTO{Options: options}, nil
}

func (s *AuthService) SendRecoveryInstructions(ctx context.Context, payload dto.ForgotPasswordSendDTO) error {
	logger := s.logger.With(zap.String("email", payload.Email), zap.String("method", payload.Method))
	logger.Info("Отправка инструкций для восстановления")

	user, err := s.userRepo.FindUserByEmailOrLogin(ctx, payload.Email)
	if err != nil {
		logger.Warn("Попытка отправки инструкций для несуществующего пользователя")
		return nil // Тихо выходим
	}

	if payload.Method == "email" {
		return s.sendEmailRecovery(ctx, user)
	}

	if payload.Method == "phone" {
		return s.sendPhoneRecovery(ctx, user)
	}

	return apperrors.ErrBadRequest // На случай, если метод не "email" и не "phone"
}

func (s *AuthService) ResetPasswordWithEmail(ctx context.Context, payload dto.ResetPasswordEmailDTO) error {
	logger := s.logger.With(zap.String("token_suffix", payload.Token[len(payload.Token)-4:]))
	logger.Info("Сброс пароля по email токену")

	// Получаем ID пользователя из кеша
	cacheKey := fmt.Sprintf("reset_token:%s", payload.Token)
	userIDStr, err := s.cacheRepo.Get(ctx, cacheKey)
	if err != nil {
		logger.Warn("Невалидный или истекший токен сброса")
		return apperrors.ErrInvalidResetToken
	}

	s.cacheRepo.Del(ctx, cacheKey)

	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		logger.Error("не удалось преобразовать userID из Redis в uint64", zap.Error(err), zap.String("userIDStr", userIDStr))
		return apperrors.ErrInternalServer
	}
	return s.updateUserPassword(ctx, logger, userID, payload.NewPassword)
}

func (s *AuthService) ResetPasswordWithPhone(ctx context.Context, payload dto.ResetPasswordPhoneDTO) error {
	logger := s.logger.With(zap.String("email", payload.Email))
	logger.Info("Сброс пароля по коду с телефона")

	user, err := s.userRepo.FindUserByEmailOrLogin(ctx, payload.Email)
	if err != nil {
		logger.Warn("Пользователь не найден при попытке сброса по коду")
		return apperrors.ErrInvalidCredentials
	}

	if err := s.checkResetCodeAttempts(ctx, user.ID); err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("reset_code:%d", user.ID)
	storedCode, err := s.cacheRepo.Get(ctx, cacheKey)
	if err != nil || storedCode != payload.Code {
		logger.Warn("Неверный код сброса")
		s.handleFailedResetCodeAttempt(ctx, user.ID)
		return apperrors.ErrInvalidVerificationCode
	}

	s.cacheRepo.Del(ctx, cacheKey)

	return s.updateUserPassword(ctx, logger, user.ID, payload.NewPassword)
}

func (s *AuthService) sendEmailRecovery(ctx context.Context, user *entities.User) error {
	resetToken := uuid.New().String()
	cacheKey := fmt.Sprintf("reset_token:%s", resetToken)

	if err := s.cacheRepo.Set(ctx, cacheKey, user.ID, resetPasswordTokenTTL); err != nil {
		s.logger.Error("Не удалось сохранить токен сброса в кеш", zap.Error(err), zap.Uint64("userID", user.ID))
		return apperrors.ErrInternalServer
	}
	s.logger.Info("Сгенерирован токен для сброса по email", zap.String("token", resetToken), zap.Uint64("userID", user.ID))
	// TODO: Реальная отправка email со ссылкой, содержащей resetToken
	return nil
}

func (s *AuthService) sendPhoneRecovery(ctx context.Context, user *entities.User) error {
	if user.PhoneNumber == "" {
		s.logger.Warn("Попытка восстановления по телефону для пользователя без номера", zap.Uint64("userID", user.ID))
		return apperrors.ErrBadRequest
	}
	resetCode := fmt.Sprintf("%04d", rand.Intn(10000))
	cacheKey := fmt.Sprintf("reset_code:%d", user.ID)

	if err := s.cacheRepo.Set(ctx, cacheKey, resetCode, verificationCodeTTL); err != nil {
		s.logger.Error("не удалось сохранить код сброса в кеш", zap.Error(err), zap.Uint64("userID", user.ID))
		return apperrors.ErrInternalServer
	}
	s.logger.Info("Сгенерирован код для сброса по телефону", zap.String("code", resetCode), zap.Uint64("userID", user.ID))
	// TODO: Реальная отправка SMS с кодом `resetCode` на `user.PhoneNumber`
	return nil
}

func (s *AuthService) updateUserPassword(ctx context.Context, logger *zap.Logger, userID uint64, newPassword string) error {
	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		logger.Error("Не удалось хешировать новый пароль", zap.Error(err))
		return apperrors.ErrInternalServer
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, hashedPassword); err != nil {
		logger.Error("Не удалось обновить пароль в БД", zap.Error(err))
		return apperrors.ErrInternalServer
	}

	logger.Info("Пароль для пользователя успешно сброшен")
	return nil
}

func (s *AuthService) checkResetCodeAttempts(ctx context.Context, userID uint64) error {
	attemptsKey := fmt.Sprintf("reset_attempts:%d", userID)
	attemptsStr, _ := s.cacheRepo.Get(ctx, attemptsKey)
	attempts, _ := strconv.Atoi(attemptsStr)

	if attempts >= maxResetCodeAttempts {
		s.logger.Warn("Превышено количество попыток ввода кода  сброса", zap.Uint64("userID", userID))
		return apperrors.NewHttpError(429, "Слишком много попыток. Попробуйте запросить новый код.", nil)
	}
	return nil
}

func (s *AuthService) handleFailedResetCodeAttempt(ctx context.Context, userID uint64) {
	attemptsKey := fmt.Sprintf("reset_attempts:%d", userID)
	// Увеличиваем счетчик и устанавливаем ему время жизни.
	// Если счетчика не было, он создастся со значением 1.
	s.cacheRepo.Incr(ctx, attemptsKey)
	s.cacheRepo.Expire(ctx, attemptsKey, resetCodeAttemptsTTL)
}

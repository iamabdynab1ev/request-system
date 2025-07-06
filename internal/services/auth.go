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
	"time"

	"go.uber.org/zap"
)

const (
	maxLoginAttempts    = 3
	lockoutDuration     = 5 * time.Minute
	verificationCodeTTL = 5 * time.Minute
)

type AuthServiceInterface interface {
	Login(ctx context.Context, payload dto.LoginDTO) (*entities.User, error)
	SendVerificationCode(ctx context.Context, payload dto.SendCodeDTO) error
	LoginWithCode(ctx context.Context, payload dto.VerifyCodeDTO) (*entities.User, error)
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

	s.logger.Info("Код верификации сгенерирован (для теста)", zap.Int("userID", user.ID), zap.String("code", code))

	// TODO: Реализовать отправку SMS или Email
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

	if err := s.checkLockout(ctx, user.ID); err != nil {
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

func (s *AuthService) checkLockout(ctx context.Context, userID int) error {
	lockoutKey := fmt.Sprintf("lockout:%d", userID)
	_, err := s.cacheRepo.Get(ctx, lockoutKey)
	if err == nil {
		return apperrors.ErrAccountLocked
	}
	return nil
}

func (s *AuthService) handleFailedLoginAttempt(ctx context.Context, userID int) {
	attemptsKey := fmt.Sprintf("login_attempts:%d", userID)
	attempts, _ := s.cacheRepo.Incr(ctx, attemptsKey)

	if attempts >= maxLoginAttempts {
		lockoutKey := fmt.Sprintf("lockout:%d", userID)
		s.cacheRepo.Set(ctx, lockoutKey, "locked", lockoutDuration)
		s.cacheRepo.Del(ctx, attemptsKey)
	}
}

func (s *AuthService) resetLoginAttempts(ctx context.Context, userID int) {
	attemptsKey := fmt.Sprintf("login_attempts:%d", userID)
	lockoutKey := fmt.Sprintf("lockout:%d", userID)
	s.cacheRepo.Del(ctx, attemptsKey, lockoutKey)
}

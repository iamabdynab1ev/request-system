package services

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/entities"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
)

const telegramLinkTokenTTL = 10 * time.Minute
const telegramLinkShortCodeLength = 6
const telegramLinkShortCodeAttempts = 10

type telegramLinkCachePayload struct {
	UserID    uint64 `json:"user_id"`
	Token     string `json:"token,omitempty"`
	ShortCode string `json:"short_code,omitempty"`
}

func telegramLinkTokenCacheKey(token string) string {
	return fmt.Sprintf("telegram-link-token:%s", token)
}

func telegramLinkShortCodeCacheKey(code string) string {
	return fmt.Sprintf("telegram-link-short:%s", code)
}

func parseTelegramLinkCachePayload(raw string) (*telegramLinkCachePayload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty telegram link payload")
	}

	if strings.HasPrefix(raw, "{") {
		var payload telegramLinkCachePayload
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return nil, err
		}
		if payload.UserID == 0 {
			return nil, fmt.Errorf("telegram link payload user_id is empty")
		}
		return &payload, nil
	}

	uid, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || uid == 0 {
		return nil, fmt.Errorf("invalid telegram link payload")
	}

	return &telegramLinkCachePayload{UserID: uid}, nil
}

func isTelegramShortCodeValue(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != telegramLinkShortCodeLength {
		return false
	}

	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}

	return true
}

func generateTelegramShortCode() (string, error) {
	max := big.NewInt(900000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", telegramLinkShortCodeLength, 100000+n.Int64()), nil
}

func (s *UserService) allocateTelegramShortCode(ctx context.Context) (string, error) {
	for attempt := 0; attempt < telegramLinkShortCodeAttempts; attempt++ {
		code, err := generateTelegramShortCode()
		if err != nil {
			return "", err
		}

		if existing, err := s.cacheRepository.Get(ctx, telegramLinkShortCodeCacheKey(code)); err == nil && strings.TrimSpace(existing) != "" {
			continue
		}

		return code, nil
	}

	return "", fmt.Errorf("failed to allocate unique telegram short code")
}

func (s *UserService) GenerateTelegramLinkToken(ctx context.Context) (*dto.TelegramLinkTokenDTO, error) {
	uid, err := utils.GetUserIDFromCtx(ctx)
	if err != nil || uid == 0 {
		s.logger.Warn("Telegram link token generation rejected: missing user in context", zap.Error(err))
		return nil, apperrors.ErrUnauthorized
	}

	token := uuid.New().String()
	shortCode, err := s.allocateTelegramShortCode(ctx)
	if err != nil {
		s.logger.Error("Failed to allocate telegram short code", zap.Uint64("user_id", uid), zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}

	payload := telegramLinkCachePayload{
		UserID:    uid,
		Token:     token,
		ShortCode: shortCode,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("Failed to marshal telegram link payload", zap.Uint64("user_id", uid), zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}

	if err := s.cacheRepository.Set(ctx, telegramLinkTokenCacheKey(token), string(payloadJSON), telegramLinkTokenTTL); err != nil {
		s.logger.Error("Failed to store telegram link token", zap.Uint64("user_id", uid), zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}
	if err := s.cacheRepository.Set(ctx, telegramLinkShortCodeCacheKey(shortCode), string(payloadJSON), telegramLinkTokenTTL); err != nil {
		_ = s.cacheRepository.Del(ctx, telegramLinkTokenCacheKey(token))
		s.logger.Error("Failed to store telegram short code", zap.Uint64("user_id", uid), zap.Error(err))
		return nil, apperrors.ErrInternalServer
	}

	s.logger.Info("Telegram link token generated", zap.Uint64("user_id", uid), zap.String("short_code", shortCode))
	return &dto.TelegramLinkTokenDTO{
		Token:            token,
		ShortCode:        shortCode,
		ExpiresInSeconds: int(telegramLinkTokenTTL / time.Second),
	}, nil
}

func (s *UserService) GetTelegramLinkStatus(ctx context.Context) (*dto.TelegramLinkStatusDTO, error) {
	uid, err := utils.GetUserIDFromCtx(ctx)
	if err != nil || uid == 0 {
		return nil, apperrors.ErrUnauthorized
	}

	user, err := s.userRepository.FindUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || apperrors.IsNotFound(err) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, err
	}

	result := &dto.TelegramLinkStatusDTO{
		Linked: user.TelegramChatID.Valid,
	}
	if user.TelegramChatID.Valid {
		chatID := user.TelegramChatID.Int64
		result.TelegramChatID = &chatID
	}

	return result, nil
}

func (s *UserService) UnlinkTelegram(ctx context.Context) error {
	uid, err := utils.GetUserIDFromCtx(ctx)
	if err != nil || uid == 0 {
		return apperrors.ErrUnauthorized
	}

	user, err := s.userRepository.FindUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || apperrors.IsNotFound(err) {
			return apperrors.ErrUserNotFound
		}
		return err
	}
	if !user.TelegramChatID.Valid {
		return nil
	}

	return s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
		return s.userRepository.ClearTelegramChatID(ctx, tx, uid)
	})
}

func (s *UserService) FindUserByTelegramChatID(ctx context.Context, chatID int64) (*entities.User, error) {
	return s.userRepository.FindUserByTelegramChatID(ctx, chatID)
}

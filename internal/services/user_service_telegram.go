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
	"github.com/jackc/pgx/v5/pgconn"
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

func (s *UserService) ConfirmTelegramLink(ctx context.Context, token string, chatID int64) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return apperrors.NewBadRequestError("Код привязки не указан")
	}

	lookupKeys := []string{telegramLinkTokenCacheKey(token)}
	if isTelegramShortCodeValue(token) {
		lookupKeys = append([]string{telegramLinkShortCodeCacheKey(token)}, lookupKeys...)
	}

	var (
		cacheKey string
		val      string
		err      error
	)
	for _, key := range lookupKeys {
		val, err = s.cacheRepository.Get(ctx, key)
		if err == nil && strings.TrimSpace(val) != "" {
			cacheKey = key
			break
		}
	}
	if cacheKey == "" {
		s.logger.Warn("Telegram link token not found or expired", zap.String("token_or_code", token))
		return apperrors.NewBadRequestError("Неверный код или срок его действия истёк")
	}

	payload, err := parseTelegramLinkCachePayload(val)
	if err != nil {
		s.logger.Error("Telegram link token contains invalid payload",
			zap.String("token_or_code", token),
			zap.String("cache_key", cacheKey),
			zap.String("cached_value", val),
			zap.Error(err))
		return apperrors.NewBadRequestError("Код привязки повреждён. Получите новый код на сайте")
	}
	uid := payload.UserID

	cleanupKeys := map[string]struct{}{
		cacheKey: {},
	}
	if payload.Token != "" {
		cleanupKeys[telegramLinkTokenCacheKey(payload.Token)] = struct{}{}
	}
	if payload.ShortCode != "" {
		cleanupKeys[telegramLinkShortCodeCacheKey(payload.ShortCode)] = struct{}{}
	}
	cleanup := func() {
		for key := range cleanupKeys {
			_ = s.cacheRepository.Del(ctx, key)
		}
	}

	existingUser, err := s.userRepository.FindUserByTelegramChatID(ctx, chatID)
	if err == nil && existingUser != nil {
		if existingUser.ID == uid {
			cleanup()
			s.logger.Info("Telegram already linked to the same user",
				zap.Uint64("user_id", uid),
				zap.Int64("chat_id", chatID))
			return nil
		}

		if err := s.txManager.RunInTransaction(ctx, func(tx pgx.Tx) error {
			if err := s.userRepository.ClearTelegramChatID(ctx, tx, existingUser.ID); err != nil {
				return err
			}
			return s.userRepository.UpdateTelegramChatIDTx(ctx, tx, uid, chatID)
		}); err != nil {
			s.logger.Error("Failed to reassign telegram chat id",
				zap.Int64("chat_id", chatID),
				zap.Uint64("from_user_id", existingUser.ID),
				zap.Uint64("to_user_id", uid),
				zap.Error(err))
			return err
		}

		cleanup()
		s.logger.Warn("Telegram chat reassigned to another user",
			zap.Int64("chat_id", chatID),
			zap.Uint64("from_user_id", existingUser.ID),
			zap.Uint64("to_user_id", uid))
		return nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		s.logger.Error("Failed to check existing telegram link",
			zap.Int64("chat_id", chatID),
			zap.Error(err))
		return apperrors.ErrInternalServer
	}
	if err := s.userRepository.UpdateTelegramChatID(ctx, uid, chatID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperrors.NewBadRequestError("Этот Telegram аккаунт уже привязан к другому пользователю")
		}
		if errors.Is(err, apperrors.ErrNotFound) {
			s.logger.Warn("Telegram link target user not found",
				zap.Uint64("user_id", uid),
				zap.Int64("chat_id", chatID))
			return apperrors.NewBadRequestError("Пользователь для этого кода не найден. Получите новый код на сайте")
		}
		s.logger.Error("Failed to update telegram chat id",
			zap.Uint64("user_id", uid),
			zap.Int64("chat_id", chatID),
			zap.Error(err))
		return err
	}
	cleanup()
	s.logger.Info("Telegram account linked successfully",
		zap.Uint64("user_id", uid),
		zap.Int64("chat_id", chatID))
	return nil
}

func (s *UserService) FindUserByTelegramChatID(ctx context.Context, chatID int64) (*entities.User, error) {
	return s.userRepository.FindUserByTelegramChatID(ctx, chatID)
}

package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgapi "request-system/pkg/telegram"
)

const telegramScreenMessageKey = "tg_screen_message:%d"
const telegramScreenTTL = 30 * 24 * time.Hour

func (c *TelegramController) getScreenMessageID(ctx context.Context, chatID int64) int {
	value, err := c.cacheRepo.Get(ctx, fmt.Sprintf(telegramScreenMessageKey, chatID))
	if err != nil {
		return 0
	}

	messageID, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || messageID <= 0 {
		return 0
	}

	return messageID
}

func (c *TelegramController) setScreenMessageID(ctx context.Context, chatID int64, messageID int) {
	if messageID <= 0 {
		return
	}

	_ = c.cacheRepo.Set(ctx, fmt.Sprintf(telegramScreenMessageKey, chatID), strconv.Itoa(messageID), telegramScreenTTL)
}

func (c *TelegramController) clearScreenMessageID(ctx context.Context, chatID int64) {
	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramScreenMessageKey, chatID))
}

func (c *TelegramController) renderScreenWithID(ctx context.Context, chatID int64, messageID int, text string, options ...tgapi.MessageOption) (int, error) {
	targetMessageID := messageID
	if targetMessageID == 0 {
		targetMessageID = c.getScreenMessageID(ctx, chatID)
	}

	forceNewMessage := tgapi.HasReplyKeyboard(options...)

	if targetMessageID > 0 && !forceNewMessage {
		if err := c.tgService.EditMessageText(ctx, chatID, targetMessageID, text, options...); err == nil {
			c.setScreenMessageID(ctx, chatID, targetMessageID)
			return targetMessageID, nil
		}
	}

	if targetMessageID > 0 {
		_ = c.tgService.DeleteMessage(ctx, chatID, targetMessageID)
	}

	newMessageID, err := c.tgService.SendMessageWithID(ctx, chatID, text, options...)
	if err != nil {
		return 0, err
	}

	c.setScreenMessageID(ctx, chatID, newMessageID)
	return newMessageID, nil
}

func (c *TelegramController) renderScreen(ctx context.Context, chatID int64, messageID int, text string, options ...tgapi.MessageOption) error {
	_, err := c.renderScreenWithID(ctx, chatID, messageID, text, options...)
	return err
}

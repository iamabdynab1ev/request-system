package telegram

import (
	"context"
	"fmt"

	tgapi "request-system/pkg/telegram"
)

func (c *TelegramController) statusScreenOptions() []tgapi.MessageOption {
	keyboard := make([][]tgapi.InlineKeyboardButton, 0, len(c.mainMenuKeyboard())+1)
	keyboard = append(keyboard, []tgapi.InlineKeyboardButton{{
		Text:         unlinkButton,
		CallbackData: `{"action":"unlink_prompt"}`,
	}})
	keyboard = append(keyboard, c.mainMenuKeyboard()...)

	return []tgapi.MessageOption{
		tgapi.WithKeyboard(keyboard),
		tgapi.WithMarkdownV2(),
	}
}

func (c *TelegramController) unlinkConfirmationOptions() []tgapi.MessageOption {
	return []tgapi.MessageOption{
		tgapi.WithKeyboard([][]tgapi.InlineKeyboardButton{{
			{Text: confirmUnlinkButton, CallbackData: `{"action":"unlink_confirm"}`},
			{Text: cancelButton, CallbackData: `{"action":"main_status"}`},
		}}),
		tgapi.WithMarkdownV2(),
	}
}

func (c *TelegramController) handleLinkStatusCommand(ctx context.Context, chatID int64) error {
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	status, err := c.userService.GetTelegramLinkStatus(userCtx)
	if err != nil {
		return err
	}

	if !status.Linked {
		return c.renderScreen(
			ctx,
			chatID,
			0,
			"ℹ️ *Telegram сейчас не привязан*\n\nОткройте профиль на сайте, получите новый код и отправьте его сюда командой `/start <код>`\\.",
			tgapi.WithMarkdownV2(),
		)
	}

	text := fmt.Sprintf(
		"🔐 *Статус Telegram*\n\n"+
			"👤 *Пользователь:* %s\n"+
			"🆔 *Chat ID:* `%d`\n\n"+
			"Если нужно отключить этот Telegram от аккаунта, нажмите кнопку ниже или используйте /unlink\\.",
		tgapi.EscapeTextForMarkdownV2(user.Fio),
		chatID,
	)

	return c.renderScreen(ctx, chatID, 0, text, c.statusScreenOptions()...)
}

func (c *TelegramController) handleUnlinkCommand(ctx context.Context, chatID int64) error {
	user, _, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	text := fmt.Sprintf(
		"⚠️ *Подтвердите отвязку*\n\n"+
			"Вы действительно хотите отвязать этот Telegram от аккаунта *%s*?\n\n"+
			"После отвязки:\n"+
			"• уведомления по заявкам перестанут приходить\n"+
			"• работа с заявками в боте станет недоступна\n"+
			"• для повторной привязки понадобится новый код с сайта",
		tgapi.EscapeTextForMarkdownV2(user.Fio),
	)

	return c.renderScreen(ctx, chatID, 0, text, c.unlinkConfirmationOptions()...)
}

func (c *TelegramController) handleConfirmUnlinkAction(ctx context.Context, chatID int64) error {
	user, userCtx, err := c.prepareUserContext(ctx, chatID)
	if err != nil {
		return err
	}

	if err := c.userService.UnlinkTelegram(userCtx); err != nil {
		return err
	}

	_ = c.cacheRepo.Del(ctx, fmt.Sprintf(telegramStateKey, chatID))

	screenMessageID := c.getScreenMessageID(ctx, chatID)
	if screenMessageID > 0 {
		_ = c.tgService.DeleteMessage(ctx, chatID, screenMessageID)
		c.clearScreenMessageID(ctx, chatID)
	}

	text := fmt.Sprintf(
		"✅ *Telegram отвязан*\n\n"+
			"Аккаунт *%s* больше не связан с этим Telegram\\.\n\n"+
			"Чтобы привязать его снова или подключить другой аккаунт, получите новый код на сайте и отправьте `/start <код>`\\.",
		tgapi.EscapeTextForMarkdownV2(user.Fio),
	)

	return c.renderScreen(ctx, chatID, 0, text, tgapi.WithMarkdownV2())
}

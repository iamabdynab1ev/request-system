package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"request-system/internal/dto"
	"request-system/internal/entities"
	tgapi "request-system/pkg/telegram"
)

func (c *TelegramController) orderBackKeyboard(orderID uint64) [][]tgapi.InlineKeyboardButton {
	return [][]tgapi.InlineKeyboardButton{
		{{Text: menuBackButton, CallbackData: fmt.Sprintf(`{"action":"select_order","order_id":%d}`, orderID)}},
	}
}

func (c *TelegramController) renderStateScreen(ctx context.Context, chatID int64, state *dto.TelegramState, text string, options ...tgapi.MessageOption) error {
	messageID := 0
	if state != nil {
		messageID = state.MessageID
	}

	renderedMessageID, err := c.renderScreenWithID(ctx, chatID, messageID, text, options...)
	if err != nil {
		return err
	}

	if state != nil && renderedMessageID > 0 && state.MessageID != renderedMessageID {
		state.MessageID = renderedMessageID
		return c.setUserState(ctx, chatID, state)
	}

	return nil
}

func (c *TelegramController) syncStateMessageID(ctx context.Context, chatID int64, state *dto.TelegramState) error {
	if state == nil {
		return nil
	}

	currentMessageID := c.getScreenMessageID(ctx, chatID)
	if currentMessageID <= 0 || state.MessageID == currentMessageID {
		return nil
	}

	state.MessageID = currentMessageID
	return c.setUserState(ctx, chatID, state)
}

func (c *TelegramController) showEditMenuForState(ctx context.Context, chatID int64, state *dto.TelegramState, order *entities.Order) error {
	messageID := 0
	if state != nil {
		messageID = state.MessageID
	}

	if err := c.sendEditMenu(ctx, chatID, messageID, order); err != nil {
		return err
	}

	return c.syncStateMessageID(ctx, chatID, state)
}

func (c *TelegramController) renderCommentPrompt(ctx context.Context, chatID int64, state *dto.TelegramState, notice string) error {
	text := "💬 *Введите комментарий:*\n\n_Макс\\. 500 символов_"
	if strings.TrimSpace(notice) != "" {
		text = notice + "\n\n" + text
	}
	return c.renderStateScreen(
		ctx,
		chatID,
		state,
		text,
		tgapi.WithKeyboard(c.orderBackKeyboard(state.OrderID)),
		tgapi.WithMarkdownV2(),
	)
}

func (c *TelegramController) renderDurationPrompt(ctx context.Context, chatID int64, state *dto.TelegramState, notice string) error {
	quickDurations := []struct {
		Label    string
		Duration time.Duration
	}{
		{"Через 3 часа", 3 * time.Hour},
		{"Завтра", 24 * time.Hour},
		{"Через 3 дня", 72 * time.Hour},
		{"Через неделю", 7 * 24 * time.Hour},
	}

	var keyboard [][]tgapi.InlineKeyboardButton
	row := []tgapi.InlineKeyboardButton{}
	now := time.Now().In(c.loc)
	for _, qd := range quickDurations {
		futureTime := now.Add(qd.Duration).Round(30 * time.Minute)
		callbackValue := futureTime.Format("02.01.2006 15:04")
		buttonText := fmt.Sprintf("%s (%s)", qd.Label, futureTime.Format("02.01 15:04"))
		row = append(row, tgapi.InlineKeyboardButton{
			Text:         buttonText,
			CallbackData: fmt.Sprintf(`{"action":"set_duration","value":"%s"}`, callbackValue),
		})
		if len(row) == 2 {
			keyboard = append(keyboard, row)
			row = []tgapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		keyboard = append(keyboard, row)
	}
	keyboard = append(keyboard, c.orderBackKeyboard(state.OrderID)...)

	text := "Выберите срок или отправьте его текстом в формате `ДД.ММ.ГГГГ ЧЧ:ММ`"
	if strings.TrimSpace(notice) != "" {
		text = notice + "\n\n" + text
	}

	return c.renderStateScreen(ctx, chatID, state, text,
		tgapi.WithKeyboard(keyboard),
		tgapi.WithMarkdownV2())
}

func (c *TelegramController) renderExecutorSelection(ctx context.Context, chatID int64, state *dto.TelegramState, text string, rows [][]tgapi.InlineKeyboardButton) error {
	keyboard := make([][]tgapi.InlineKeyboardButton, 0, len(rows)+1)
	keyboard = append(keyboard, rows...)
	keyboard = append(keyboard, c.orderBackKeyboard(state.OrderID)...)

	return c.renderStateScreen(ctx, chatID, state, text,
		tgapi.WithKeyboard(keyboard),
		tgapi.WithMarkdownV2())
}

func (c *TelegramController) renderSearchPrompt(ctx context.Context, chatID int64, messageID int, notice string) error {
	text := "🔍 *Поиск заявки*\n\n" +
		"Введите:\n" +
		"• номер заявки \\(например: `123`\\)\n" +
		"• или текст из описания"
	if strings.TrimSpace(notice) != "" {
		text = notice + "\n\n" + text
	}

	keyboard := [][]tgapi.InlineKeyboardButton{
		{{Text: menuMainButton, CallbackData: `{"action":"main_menu"}`}},
	}

	return c.renderScreen(ctx, chatID, messageID, text,
		tgapi.WithKeyboard(keyboard),
		tgapi.WithMarkdownV2())
}

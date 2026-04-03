package telegram

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"request-system/pkg/utils"
)

func (c *TelegramController) HandleTelegramLinkStatus(ctx echo.Context) error {
	status, err := c.userService.GetTelegramLinkStatus(ctx.Request().Context())
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	response := map[string]interface{}{
		"enabled":             c.integrationService.Enabled(),
		"linked":              status.Linked,
		"bot_username":        c.cfg.BotUsername,
		"deep_link_available": c.cfg.BotUsername != "",
	}
	if status.TelegramChatID != nil {
		response["telegram_chat_id"] = *status.TelegramChatID
	}

	return utils.SuccessResponse(ctx, response, "Статус Telegram получен", http.StatusOK)
}

func (c *TelegramController) HandleUnlinkTelegram(ctx echo.Context) error {
	if err := c.userService.UnlinkTelegram(ctx.Request().Context()); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	response := map[string]interface{}{
		"enabled":             c.integrationService.Enabled(),
		"linked":              false,
		"bot_username":        c.cfg.BotUsername,
		"deep_link_available": c.cfg.BotUsername != "",
	}

	return utils.SuccessResponse(ctx, response, "Telegram отвязан", http.StatusOK)
}

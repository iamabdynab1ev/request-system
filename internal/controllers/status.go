package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"
    "strings"
	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type StatusController struct {
	statusService services.StatusServiceInterface
	logger        *zap.Logger
}

func NewStatusController(statusService services.StatusServiceInterface, logger *zap.Logger) *StatusController {
	return &StatusController{statusService: statusService, logger: logger}
}

func (c *StatusController) GetStatuses(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	res, err := c.statusService.GetStatuses(reqCtx, filter)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(
		ctx,
		res.List,
		"Список статусов успешно получен",
		http.StatusOK,
		res.Pagination.TotalCount,
	)
}

func (c *StatusController) FindStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest, // код для HTTP
				"Неверный ID",         // сообщение для пользователя
				err,                   // внутренняя ошибка для логов
				map[string]interface{}{"param": ctx.Param("id")}, // контекст для логов
			),
			c.logger, // передаём логер
		)
	}

	res, err := c.statusService.FindStatus(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, res, "Статус успешно найден", http.StatusOK)
}

func (c *StatusController) FindByCode(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	code := ctx.Param("code")

	id, err := c.statusService.FindIDByCode(reqCtx, code)
	if err != nil {
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(http.StatusBadRequest, "Неверный код статуса", err, nil),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, map[string]any{"id": id}, "Статус найден", http.StatusOK)
}

func (c *StatusController) CreateStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	contentType := ctx.Request().Header.Get("Content-Type")

	var dto dto.CreateStatusDTO

	// ЛОГИКА ИСПРАВЛЕНИЯ: Поддержка JSON и Multipart
	if strings.HasPrefix(contentType, "application/json") {
		// Если пришел чистый JSON
		if err := ctx.Bind(&dto); err != nil {
			return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Некорректный JSON в теле запроса"), c.logger)
		}
	} else {
		// Если пришла форма (с файлами или data)
		dataString := ctx.FormValue("data")
		if dataString == "" {
			return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Поле 'data' в form-data обязательно"), c.logger)
		}
		if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
			return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный JSON в 'data'", err, nil), c.logger)
		}
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	// Файлы получаем только если они есть (в JSON их не будет, это норм)
	iconSmall, _ := ctx.FormFile("icon_small")
	iconBig, _ := ctx.FormFile("icon_big")

	createdStatus, err := c.statusService.CreateStatus(reqCtx, dto, iconSmall, iconBig)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, createdStatus, "Статус успешно создан", http.StatusCreated)
}

func (c *StatusController) UpdateStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный ID"), c.logger)
	}

	contentType := ctx.Request().Header.Get("Content-Type")
	var dto dto.UpdateStatusDTO

	// ЛОГИКА ИСПРАВЛЕНИЯ
	if strings.HasPrefix(contentType, "application/json") {
		if err := ctx.Bind(&dto); err != nil {
			return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Некорректный JSON"), c.logger)
		}
	} else {
		dataString := ctx.FormValue("data")
		if dataString != "" {
			if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
				return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный JSON в 'data'", err, nil), c.logger)
			}
		}
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	iconSmall, _ := ctx.FormFile("icon_small")
	iconBig, _ := ctx.FormFile("icon_big")

	updatedStatus, err := c.statusService.UpdateStatus(reqCtx, id, dto, iconSmall, iconBig)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, updatedStatus, "Статус успешно обновлен", http.StatusOK)
}

func (c *StatusController) DeleteStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Warn("DeleteStatus: Неверный ID", zap.String("param", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный ID",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	if err := c.statusService.DeleteStatus(reqCtx, id); err != nil {
		c.logger.Error("DeleteStatus: Ошибка при удалении статуса", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Статус успешно удален", http.StatusOK)
}

package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

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
	c.logger.Debug("CreateStatus: Начало обработки запроса")

	dataString := ctx.FormValue("data")
	if dataString == "" {
		c.logger.Warn("CreateStatus: Поле 'data' отсутствует в form-data")
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Поле 'data' с JSON обязательно",
				apperrors.ErrBadRequest,
				nil,
			),
			c.logger,
		)
	}
	c.logger.Debug("CreateStatus: Поле 'data' получено", zap.String("data", dataString))

	var dto dto.CreateStatusDTO
	if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
		c.logger.Error("CreateStatus: Ошибка парсинга JSON", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный JSON в 'data'",
				err,
				map[string]interface{}{"data": dataString},
			),
			c.logger,
		)
	}
	c.logger.Debug("CreateStatus: JSON успешно распарсен", zap.Any("dto", dto))

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreateStatus: Ошибка валидации DTO", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	c.logger.Debug("CreateStatus: DTO прошел валидацию")

	iconSmall, errSmall := ctx.FormFile("icon_small")
	if errSmall != nil && errSmall != http.ErrMissingFile {
		c.logger.Error("CreateStatus: Критическая ошибка при получении icon_small", zap.Error(errSmall))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Ошибка при получении файла icon_small",
				errSmall,
				nil,
			),
			c.logger,
		)
	}
	if errSmall == http.ErrMissingFile {
		c.logger.Debug("CreateStatus: Файл 'icon_small' не был предоставлен")
	} else {
		c.logger.Debug("CreateStatus: Файл 'icon_small' получен", zap.String("filename", iconSmall.Filename))
	}

	iconBig, errBig := ctx.FormFile("icon_big")
	if errBig != nil && errBig != http.ErrMissingFile {
		c.logger.Error("CreateStatus: Критическая ошибка при получении icon_big", zap.Error(errBig))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Ошибка при получении файла icon_big",
				errBig,
				nil,
			),
			c.logger,
		)
	}
	if errBig == http.ErrMissingFile {
		c.logger.Debug("CreateStatus: Файл 'icon_big' не был предоставлен")
	} else {
		c.logger.Debug("CreateStatus: Файл 'icon_big' получен", zap.String("filename", iconBig.Filename))
	}

	c.logger.Debug("CreateStatus: Вызов statusService.CreateStatus")
	createdStatus, err := c.statusService.CreateStatus(reqCtx, dto, iconSmall, iconBig)
	if err != nil {
		c.logger.Error("CreateStatus: ПОЙМАНА ОШИБКА ИЗ СЕРВИСА", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	c.logger.Info("CreateStatus: Статус успешно создан", zap.Any("result", createdStatus))
	return utils.SuccessResponse(ctx, createdStatus, "Статус успешно создан", http.StatusCreated)
}

func (c *StatusController) UpdateStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Warn("UpdateStatus: Неверный ID", zap.String("param", ctx.Param("id")), zap.Error(err))
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

	dataString := ctx.FormValue("data")
	var dto dto.UpdateStatusDTO
	if dataString != "" {
		if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
			c.logger.Error("UpdateStatus: Неверный JSON в 'data'", zap.String("data", dataString), zap.Error(err))
			return utils.ErrorResponse(ctx,
				apperrors.NewHttpError(
					http.StatusBadRequest,
					"Неверный JSON в 'data'",
					err,
					map[string]interface{}{"data": dataString},
				),
				c.logger,
			)
		}
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("UpdateStatus: Ошибка валидации DTO", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	iconSmall, errSmall := ctx.FormFile("icon_small")
	if errSmall != nil && errSmall != http.ErrMissingFile {
		c.logger.Error("UpdateStatus: Ошибка при получении icon_small", zap.Error(errSmall))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Ошибка при получении файла icon_small",
				errSmall,
				nil,
			),
			c.logger,
		)
	}

	iconBig, errBig := ctx.FormFile("icon_big")
	if errBig != nil && errBig != http.ErrMissingFile {
		c.logger.Error("UpdateStatus: Ошибка при получении icon_big", zap.Error(errBig))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Ошибка при получении файла icon_big",
				errBig,
				nil,
			),
			c.logger,
		)
	}

	updatedStatus, err := c.statusService.UpdateStatus(reqCtx, id, dto, iconSmall, iconBig)
	if err != nil {
		c.logger.Error("UpdateStatus: Ошибка при обновлении статуса", zap.Uint64("id", id), zap.Error(err))
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

package controllers

import (
	"encoding/json"
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"strconv"

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

	res, err := c.statusService.GetStatuses(reqCtx, uint64(filter.Limit), uint64(filter.Offset), filter.Search)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res.List, "Список статусов успешно получен", http.StatusOK, res.Pagination.TotalCount)
}

func (c *StatusController) FindStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID", err))
	}

	res, err := c.statusService.FindStatus(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Статус успешно найден", http.StatusOK)
}

func (c *StatusController) FindByCode(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	code := ctx.Param("code")
	res, err := c.statusService.FindByCode(reqCtx, code)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Статус успешно найден", http.StatusOK)
}

func (c *StatusController) CreateStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	c.logger.Debug("CreateStatus: Начало обработки запроса")

	dataString := ctx.FormValue("data")
	if dataString == "" {
		c.logger.Warn("CreateStatus: Поле 'data' отсутствует в form-data")
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Поле 'data' с JSON обязательно", nil))
	}
	c.logger.Debug("CreateStatus: Поле 'data' получено", zap.String("data", dataString))

	var dto dto.CreateStatusDTO
	if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
		c.logger.Error("CreateStatus: Ошибка парсинга JSON", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный JSON в 'data'", err))
	}
	c.logger.Debug("CreateStatus: JSON успешно распарсен", zap.Any("dto", dto))

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreateStatus: Ошибка валидации DTO", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	c.logger.Debug("CreateStatus: DTO прошел валидацию")

	iconSmall, errSmall := ctx.FormFile("icon_small")
	if errSmall != nil && errSmall != http.ErrMissingFile {
		c.logger.Error("CreateStatus: Критическая ошибка при получении icon_small", zap.Error(errSmall))
		return utils.ErrorResponse(ctx, errSmall)
	}
	if errSmall == http.ErrMissingFile {
		c.logger.Debug("CreateStatus: Файл 'icon_small' не был предоставлен")
	} else {
		c.logger.Debug("CreateStatus: Файл 'icon_small' получен", zap.String("filename", iconSmall.Filename))
	}

	iconBig, errBig := ctx.FormFile("icon_big")
	if errBig != nil && errBig != http.ErrMissingFile {
		c.logger.Error("CreateStatus: Критическая ошибка при получении icon_big", zap.Error(errBig))
		return utils.ErrorResponse(ctx, errBig)
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
		return utils.ErrorResponse(ctx, err)
	}

	c.logger.Info("CreateStatus: Статус успешно создан", zap.Any("result", createdStatus))
	return utils.SuccessResponse(ctx, createdStatus, "Статус успешно создан", http.StatusCreated)
}
func (c *StatusController) UpdateStatus(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID", err))
	}

	dataString := ctx.FormValue("data")
	var dto dto.UpdateStatusDTO
	if dataString != "" {
		if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
			return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный JSON в 'data'", err))
		}
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	iconSmall, errSmall := ctx.FormFile("icon_small")
	if errSmall != nil && errSmall != http.ErrMissingFile {
		return utils.ErrorResponse(ctx, errSmall)
	}
	iconBig, errBig := ctx.FormFile("icon_big")
	if errBig != nil && errBig != http.ErrMissingFile {
		return utils.ErrorResponse(ctx, errBig)
	}

	updatedStatus, err := c.statusService.UpdateStatus(ctx.Request().Context(), id, dto, iconSmall, iconBig)
	if err != nil {
		c.logger.Error("Ошибка при обновлении статуса", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, updatedStatus, "Статус успешно обновлен", http.StatusOK)
}

func (c *StatusController) DeleteStatus(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID", err))
	}

	err = c.statusService.DeleteStatus(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Статус успешно удален", http.StatusOK)
}

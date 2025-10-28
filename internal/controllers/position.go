package controllers

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
)

type PositionController struct {
	service services.PositionServiceInterface
	logger  *zap.Logger
}

func NewPositionController(service services.PositionServiceInterface, logger *zap.Logger) *PositionController {
	return &PositionController{service: service, logger: logger}
}

func (c *PositionController) Create(ctx echo.Context) error {
	var d dto.CreatePositionDTO
	if err := ctx.Bind(&d); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверные данные", err, nil), c.logger)
	}
	if err := ctx.Validate(&d); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	result, err := c.service.Create(ctx.Request().Context(), d)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, result, "Должность создана", http.StatusCreated)
}

func (c *PositionController) Update(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Update: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат ID", err, nil), c.logger)
	}

	rawBody, err := ioutil.ReadAll(ctx.Request().Body)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Не удалось прочитать тело запроса", err, nil), c.logger)
	}
	ctx.Request().Body = ioutil.NopCloser(bytes.NewBuffer(rawBody))

	var d dto.UpdatePositionDTO
	if err := ctx.Bind(&d); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверные данные", err, nil), c.logger)
	}
	if err := ctx.Validate(&d); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	result, err := c.service.Update(ctx.Request().Context(), id, d, rawBody)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, result, "Должность обновлена", http.StatusOK)
}

func (c *PositionController) Delete(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Delete: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат ID", err, nil), c.logger)
	}

	if err := c.service.Delete(ctx.Request().Context(), id); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Должность удалена", http.StatusOK)
}

func (c *PositionController) GetByID(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("GetByID: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный формат ID", err, nil), c.logger)
	}

	result, err := c.service.GetByID(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, result, "Должность найдена", http.StatusOK)
}

func (c *PositionController) GetAll(ctx echo.Context) error {
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	paginatedResult, err := c.service.GetAll(ctx.Request().Context(), filter)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(
		ctx,
		paginatedResult.List,
		"Список должностей получен",
		http.StatusOK,
		paginatedResult.Pagination.TotalCount,
	)
}

func (c *PositionController) GetTypes(ctx echo.Context) error {
	result, err := c.service.GetTypes(ctx.Request().Context())
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, result, "Список типов должностей получен", http.StatusOK)
}

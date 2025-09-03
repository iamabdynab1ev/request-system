package controllers

import (
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type PermissionController struct {
	permService services.PermissionServiceInterface
	logger      *zap.Logger
}

func NewPermissionController(permService services.PermissionServiceInterface, logger *zap.Logger) *PermissionController {
	return &PermissionController{permService: permService, logger: logger}
}

func (c *PermissionController) GetPermissions(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	search := ctx.QueryParam("search")

	res, err := c.permService.GetPermissions(reqCtx, uint64(filter.Limit), uint64(filter.Offset), search)
	if err != nil {
		c.logger.Error("GetPermissions: ошибка получения списка привилегий", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось получить список привилегий",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res.List,
		"Список привилегий успешно получен",
		http.StatusOK,
		res.Pagination.TotalCount,
	)
}

func (c *PermissionController) FindPermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("FindPermission: некорректный ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID привилегии",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	res, err := c.permService.FindPermissionByID(reqCtx, id)
	if err != nil {
		c.logger.Error("FindPermission: ошибка поиска привилегии", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось найти привилегию",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Привилегия успешно найдена",
		http.StatusOK,
	)
}

func (c *PermissionController) CreatePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreatePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("CreatePermission: неверный формат запроса", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат запроса",
				err,
				nil,
			),
			c.logger,
		)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreatePermission: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.permService.CreatePermission(reqCtx, dto)
	if err != nil {
		c.logger.Error("CreatePermission: ошибка при создании привилегии", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось создать привилегию",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Привилегия успешно создана",
		http.StatusCreated,
	)
}

func (c *PermissionController) UpdatePermission(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("UpdatePermission: неверный ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID привилегии",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	var dto dto.UpdatePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("UpdatePermission: неверный формат запроса", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат запроса",
				err,
				nil,
			),
			c.logger,
		)
	}
	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("UpdatePermission: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.permService.UpdatePermission(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("UpdatePermission: ошибка при обновлении привилегии", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось обновить привилегию",
				err,
				nil,
			),
			c.logger,
		)
	}
	return utils.SuccessResponse(ctx, res, "Привилегия успешно обновлена", http.StatusOK)
}

func (c *PermissionController) DeletePermission(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("DeletePermission: неверный ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID привилегии",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	err = c.permService.DeletePermission(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("DeletePermission: ошибка при удалении привилегии", zap.Error(err))
		return utils.ErrorResponse(ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось удалить привилегию",
				err,
				nil,
			),
			c.logger,
		)
	}
	return utils.SuccessResponse(ctx, struct{}{}, "Привилегия успешно удалена", http.StatusOK)
}

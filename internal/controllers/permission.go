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

func NewPermissionController(
	permService services.PermissionServiceInterface,
	logger *zap.Logger,
) *PermissionController {
	return &PermissionController{
		permService: permService,
		logger:      logger,
	}
}

func (c *PermissionController) GetPermissions(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	limit, _ := strconv.ParseUint(ctx.QueryParam("limit"), 10, 64)
	offset, _ := strconv.ParseUint(ctx.QueryParam("offset"), 10, 64)
	search := ctx.QueryParam("search")

	if limit == 0 {
		limit = 100 // Устанавливаем лимит по умолчанию
	}

	// ----- ИСПРАВЛЕНО ЗДЕСЬ -----
	permissions, total, err := c.permService.GetPermissions(reqCtx, limit, offset, search)
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

	// Собираем PaginatedResponse DTO для ответа
	var currentPage uint64 = 1
	if limit > 0 {
		currentPage = (offset / limit) + 1
	}

	response := &dto.PaginatedResponse[dto.PermissionDTO]{
		List:       permissions,
		Pagination: dto.PaginationObject{TotalCount: total, Page: currentPage, Limit: limit},
	}

	return utils.SuccessResponse(ctx, response, "Список привилегий успешно получен", http.StatusOK)
}

func (c *PermissionController) FindPermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID"), nil)
	}
	res, err := c.permService.FindPermissionByID(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, res, "Привилегия успешно найдена", http.StatusOK)
}

func (c *PermissionController) CreatePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	var dto dto.CreatePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат запроса"), nil)
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	res, err := c.permService.CreatePermission(reqCtx, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, res, "Привилегия успешно создана", http.StatusCreated)
}

func (c *PermissionController) UpdatePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID"), nil)
	}
	var dto dto.UpdatePermissionDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат запроса"), nil)
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	res, err := c.permService.UpdatePermission(reqCtx, id, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, res, "Привилегия успешно обновлена", http.StatusOK)
}

func (c *PermissionController) DeletePermission(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID"), nil)
	}
	if err := c.permService.DeletePermission(reqCtx, id); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}
	return utils.SuccessResponse(ctx, nil, "Привилегия успешно удалена", http.StatusOK)
}

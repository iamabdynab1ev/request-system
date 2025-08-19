// Файл: internal/controllers/branch.go
// СКОПИРУЙТЕ И ПОЛНОСТЬЮ ЗАМЕНИТЕ СОДЕРЖИМОЕ

package controllers

import (
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type BranchController struct {
	branchService *services.BranchService
	logger        *zap.Logger
}

func NewBranchController(
	branchService *services.BranchService,
	logger *zap.Logger,
) *BranchController {
	return &BranchController{
		branchService: branchService,
		logger:        logger,
	}
}

func (c *BranchController) GetBranches(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.QueryParams())

	branches, total, err := c.branchService.GetBranches(reqCtx, uint64(filter.Limit), uint64(filter.Offset))
	if err != nil {
		c.logger.Error("Ошибка при получении списка филиалов", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, branches, "Список филиалов успешно получен", http.StatusOK, total)
}

func (c *BranchController) FindBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		// ИСПРАВЛЕНО: используем функцию NewBadRequestError
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Некорректный ID филиала"))
	}

	res, err := c.branchService.FindBranch(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при поиске филиала", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Филиал успешно найден", http.StatusOK)
}

func (c *BranchController) CreateBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	var dto dto.CreateBranchDTO
	if err := ctx.Bind(&dto); err != nil {
		// ИСПРАВЛЕНО: используем функцию NewBadRequestError
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных"))
	}
	if err := ctx.Validate(&dto); err != nil {
		// ИСПРАВЛЕНО: используем функцию NewBadRequestError
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError(err.Error()))
	}

	res, err := c.branchService.CreateBranch(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ошибка при создании филиала", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Филиал успешно создан", http.StatusCreated)
}

func (c *BranchController) UpdateBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		// ИСПРАВЛЕНО: используем функцию NewBadRequestError
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Некорректный ID филиала"))
	}

	var dto dto.UpdateBranchDTO
	if err := ctx.Bind(&dto); err != nil {
		// ИСПРАВЛЕНО: используем функцию NewBadRequestError
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных"))
	}
	if err := ctx.Validate(&dto); err != nil {
		// ИСПРАВЛЕНО: используем функцию NewBadRequestError
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError(err.Error()))
	}

	res, err := c.branchService.UpdateBranch(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("Ошибка при обновлении филиала", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Филиал успешно обновлен", http.StatusOK)
}

func (c *BranchController) DeleteBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		// ИСПРАВЛЕНО: используем функцию NewBadRequestError
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Некорректный ID филиала"))
	}

	err = c.branchService.DeleteBranch(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при удалении филиала", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}

	return ctx.NoContent(http.StatusNoContent)
}

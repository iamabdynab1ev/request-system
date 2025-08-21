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

type BranchController struct {
	branchService services.BranchServiceInterface
	logger        *zap.Logger
}

func NewBranchController(service services.BranchServiceInterface, logger *zap.Logger) *BranchController {
	return &BranchController{branchService: service, logger: logger}
}

func (c *BranchController) GetBranches(ctx echo.Context) error {
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	branches, total, err := c.branchService.GetBranches(ctx.Request().Context(), filter)
	if err != nil {
		c.logger.Error("Ошибка при получении списка филиалов", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, branches, "Список филиалов успешно получен", http.StatusOK, total)
}

func (c *BranchController) FindBranch(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID филиала"))
	}
	res, err := c.branchService.FindBranch(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("Ошибка при поиске филиала", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Филиал успешно найден", http.StatusOK)
}

func (c *BranchController) CreateBranch(ctx echo.Context) error {
	var dto dto.CreateBranchDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных в теле запроса"))
	}
	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.branchService.CreateBranch(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("Ошибка при создании филиала", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Филиал успешно создан", http.StatusCreated)
}

func (c *BranchController) UpdateBranch(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID филиала"))
	}
	var dto dto.UpdateBranchDTO
	if err = ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных в теле запроса"))
	}
	if err = ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}
	res, err := c.branchService.UpdateBranch(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("Ошибка при обновлении филиала", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, res, "Филиал успешно обновлен", http.StatusOK)
}

func (c *BranchController) DeleteBranch(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID филиала"))
	}
	if err := c.branchService.DeleteBranch(ctx.Request().Context(), id); err != nil {
		c.logger.Error("Ошибка при удалении филиала", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}
	return utils.SuccessResponse(ctx, nil, "Филиал успешно удален", http.StatusOK)
}

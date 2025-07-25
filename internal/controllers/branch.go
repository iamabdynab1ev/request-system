package controllers

import (
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"

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

	return utils.SuccessResponse(
		ctx,
		branches,
		"Список филиалов успешно получен",
		http.StatusOK,
		total, // передаём общее количество
	)
}
func (c *BranchController) FindBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Некорректный ID филиала", zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID филиала"))
	}

	res, err := c.branchService.FindBranch(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при поиске филиала", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Филиал успешно найден",
		http.StatusOK,
	)
}

func (c *BranchController) CreateBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateBranchDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("Неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Неверный формат данных"))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка при валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, err.Error()))
	}

	res, err := c.branchService.CreateBranch(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ошибка при создании филиала", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Филиал успешно создан",
		http.StatusCreated,
	)
}

func (c *BranchController) UpdateBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Некорректный ID филиала", zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID филиала"))
	}

	if _, err := c.branchService.FindBranch(reqCtx, id); err != nil {
		c.logger.Warn("Филиал не найден", zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusNotFound, "Филиал не найден"))
	}

	var dto dto.UpdateBranchDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("Неверный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Неверный формат данных"))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка при валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, err.Error()))
	}

	err = c.branchService.UpdateBranch(reqCtx, id, dto)
	if err != nil {
		c.logger.Error("Ошибка при обновлении филиала", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(
		ctx,
		nil,
		"Филиал успешно обновлен",
		http.StatusOK,
	)
}

func (c *BranchController) DeleteBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Некорректный ID филиала", zap.Error(err))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusBadRequest, "Некорректный ID филиала"))
	}

	if _, err := c.branchService.FindBranch(reqCtx, id); err != nil {
		c.logger.Warn("Филиал не найден", zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, echo.NewHTTPError(http.StatusNotFound, "Филиал не найден"))
	}

	err = c.branchService.DeleteBranch(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при удалении филиала", zap.Error(err), zap.Uint64("id", id))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(
		ctx,
		map[string]uint64{"deleted_id": id},
		"Филиал успешно удален",
		http.StatusOK,
	)
}

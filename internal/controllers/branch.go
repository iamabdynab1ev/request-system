package controllers

import (
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
)

type BranchController struct {
	branchService *services.BranchService
}

func NewBranchController(
		branchService *services.BranchService,
) *BranchController {
	return &BranchController{
		branchService: branchService,
	}
}

func (c *BranchController) GetBranches(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	res, err := c.branchService.GetBranches(reqCtx, 6, 10)
	if err != nil {
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}


func (c *BranchController) FindBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.branchService.FindBranch(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}

func (c *BranchController) CreateBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateBranchDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.branchService.CreateBranch(reqCtx, dto)
	if err != nil {
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}

func (c *BranchController) UpdateBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	var dto dto.UpdateBranchDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.branchService.UpdateBranch(reqCtx, id, dto)
	if err != nil {
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		res,
		"Successfully",
		http.StatusOK,
	)
}

func (c *BranchController) DeleteBranch(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	err = c.branchService.DeleteBranch(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(
			ctx,
			err,
		)
	}

	return utils.SuccessResponse(
		ctx,
		struct{}{},
		"Successfully",
		http.StatusOK,
	)
}
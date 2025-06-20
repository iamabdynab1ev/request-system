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

type DepartmentController struct {
	departmentService *services.DepartmentService
	logger 			  *zap.Logger
}

func NewDepartmentController(
		departmentService *services.DepartmentService,
		logger *zap.Logger,
) *DepartmentController { 
	return &DepartmentController{
		departmentService: departmentService,
		logger: 		   logger,
	}
}

func (c *DepartmentController) GetDepartments(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	res, err := c.departmentService.GetDepartments(reqCtx, 6, 10)
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


func (c *DepartmentController) FindDepartment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.departmentService.FindDepartment(reqCtx, id)
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

func (c *DepartmentController) CreateDepartment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateDepartmentDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("неправильный запрос", zap.Error(err))
		return utils.ErrorResponse(ctx, err) 
	}


	if err := ctx.Validate(&dto); err != nil {
	c.logger.Error("Ощибка при валидации данных департамента: ", zap.Error(err))
		return utils.ErrorResponse(ctx, err) 
	}


	res, err := c.departmentService.CreateDepartment(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ощибка при создание департамента: ", zap.Error(err))
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

func (c *DepartmentController) UpdateDepartment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	var dto dto.UpdateDepartmentDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.departmentService.UpdateDepartment(reqCtx, id, dto)
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

func (c *DepartmentController) DeleteDepartment(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	err = c.departmentService.DeleteDepartment(reqCtx, id)
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
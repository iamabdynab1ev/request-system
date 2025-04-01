package controllers

import (
	"net/http"

	"request-system/internal/services"
	"request-system/pkg/utils"
	"github.com/labstack/echo/v4"
)

type BranchController struct {
	service *services.BranchService
}

func NewBranchController(service *services.BranchService) *BranchController {
	return &BranchController{service: service}
}

// GetBranches возвращает все филиалы
func (c *BranchController) GetBranches(ctx echo.Context) error {
	branches, err := c.service.GetAll()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}
	return ctx.JSON(http.StatusOK, utils.SuccessResponse(branches))
}

// GetBranch возвращает филиал по ID
func (c *BranchController) FindBranch(ctx echo.Context) error {
	id := ctx.Param("id")
	branch, err := c.service.GetByID(id)
	if err != nil {
		return ctx.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}
	return ctx.JSON(http.StatusOK, utils.SuccessResponse(branch))
}

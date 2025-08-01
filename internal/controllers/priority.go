package controllers

import (
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"

	"go.uber.org/zap"

	"github.com/labstack/echo/v4"
)

type PriorityController struct {
	priorityService *services.PriorityService
	logger          *zap.Logger
}

func NewPriorityController(priorityService *services.PriorityService,
	logger *zap.Logger,
) *PriorityController {
	return &PriorityController{
		priorityService: priorityService,
		logger:          logger,
	}
}

func (c *PriorityController) GetPriorities(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	// Parse pagination from query parameters.
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	// Service now returns the total count.
	priorities, total, err := c.priorityService.GetPriorities(reqCtx, uint64(filter.Limit), uint64(filter.Offset))
	if err != nil {
		// The service layer already logs the error.
		return utils.ErrorResponse(ctx, err)
	}

	// Ensure we return an empty array `[]` instead of `null` if no results.
	if priorities == nil {
		priorities = make([]dto.PriorityDTO, 0)
	}

	// Pass the total count to the success response for the client.
	return utils.SuccessResponse(ctx, priorities, "Successfully", http.StatusOK, total)
}

func (c *PriorityController) FindPriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Invalid priority ID format", err))
	}

	res, err := c.priorityService.FindPriority(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusOK)
}

func (c *PriorityController) CreatePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreatePriorityDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Invalid request body", err))
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Validation failed", err))
	}

	// Service now returns the created object.
	createdPriority, err := c.priorityService.CreatePriority(reqCtx, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	// Return the created object in the response body with a 201 status.
	return utils.SuccessResponse(ctx, createdPriority, "Successfully created", http.StatusCreated)
}

func (c *PriorityController) UpdatePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Invalid priority ID format", err))
	}

	var dto dto.UpdatePriorityDTO
	if err := ctx.Bind(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Invalid request body", err))
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Validation failed", err))
	}

	// Service now returns the updated object.
	updatedPriority, err := c.priorityService.UpdatePriority(reqCtx, id, dto)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, updatedPriority, "Successfully updated", http.StatusOK)
}

func (c *PriorityController) DeletePriority(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Invalid priority ID format", err))
	}

	if err = c.priorityService.DeletePriority(reqCtx, id); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, nil, "Successfully deleted", http.StatusOK)
}

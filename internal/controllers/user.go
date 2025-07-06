package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/utils"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type UserController struct {
	userService *services.UserService
	logger      *zap.Logger
}

func NewUserController(userService *services.UserService, logger *zap.Logger) *UserController {
	if logger == nil {
		logger = zap.New(zapcore.NewNopCore()) // безопасный пустой логгер
	}
	return &UserController{
		userService: userService,
		logger:      logger,
	}
}

func (c *UserController) GetUsers(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	limit := uint64(10)
	limitStr := ctx.QueryParam("limit")
	if limitStr != "" {
		parsedLimit, err := strconv.ParseUint(limitStr, 20, 64)
		if err == nil && parsedLimit > 0 {
			limit = parsedLimit
		} else {
			c.logger.Error("Ошибка парсинга limit", zap.Error(err))
			return utils.ErrorResponse(ctx, fmt.Errorf("invalid limit parameter: %w", utils.ErrorBadRequest))
		}
	}

	offset := uint64(0)
	offsetStr := ctx.QueryParam("offset")
	if offsetStr != "" {
		parsedOffset, err := strconv.ParseUint(offsetStr, 10, 64)
		if err == nil {
			offset = parsedOffset
		} else {
			c.logger.Error("Ошибка парсинга offset", zap.Error(err))
			return utils.ErrorResponse(ctx, fmt.Errorf("invalid offset parameter: %w", utils.ErrorBadRequest))
		}
	}

	res, err := c.userService.GetUsers(reqCtx, limit, offset)
	if err != nil {
		c.logger.Error("Ошибка при получении списка пользователей", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusOK)
}

func (c *UserController) FindUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Ошибка парсинга ID пользователя из URL", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid user ID format: %w", utils.ErrorBadRequest))
	}

	res, err := c.userService.FindUser(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при поиске пользователя по ID", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusOK)
}

func (c *UserController) CreateUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	var dto dto.CreateUserDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("Ошибка при связывании запроса для создания пользователя", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("request binding failed: %w", utils.ErrorBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка при валидации данных для создания пользователя", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("validation failed: %w", utils.ErrorBadRequest))
	}

	res, err := c.userService.CreateUser(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ошибка при создании пользователя в сервисе", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully created", http.StatusCreated)
}

func (c *UserController) UpdateUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	idFromURL, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Ошибка парсинга ID пользователя из URL для обновления", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid user ID format in URL: %w", utils.ErrorBadRequest))
	}

	var dto dto.UpdateUserDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("Ошибка при связывании запроса для обновления пользователя", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("request binding failed: %w", utils.ErrorBadRequest))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка при валидации данных для обновления пользователя", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("validation failed: %w", utils.ErrorBadRequest))
	}

	dto.ID = int(idFromURL)

	res, err := c.userService.UpdateUser(reqCtx, dto)
	if err != nil {
		c.logger.Error("Ошибка при обновлении пользователя в сервисе", zap.Uint64("id", idFromURL), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully updated", http.StatusOK)
}

func (c *UserController) DeleteUser(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("Ошибка парсинга ID пользователя из URL для удаления", zap.Error(err))
		return utils.ErrorResponse(ctx, fmt.Errorf("invalid user ID format: %w", utils.ErrorBadRequest))
	}

	err = c.userService.DeleteUser(reqCtx, id)
	if err != nil {
		c.logger.Error("Ошибка при мягком удалении пользователя в сервисе", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Successfully deleted", http.StatusOK)
}

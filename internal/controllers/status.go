package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"request-system/internal/dto"
	"request-system/internal/services"
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type StatusController struct {
	statusService *services.StatusService
	logger        *zap.Logger
}

func NewStatusController(
	statusService *services.StatusService,
	logger *zap.Logger,
) *StatusController {
	return &StatusController{
		statusService: statusService,
		logger:        logger,
	}
}

func (c *StatusController) GetStatuses(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	statuses, total, err := c.statusService.GetStatuses(reqCtx, uint64(filter.Limit), uint64(filter.Offset))
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	if statuses == nil {
		statuses = make([]dto.StatusDTO, 0)
	}

	return utils.SuccessResponse(ctx, statuses, "Successfully", http.StatusOK, total)
}

func (c *StatusController) FindStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный ID", err))
	}

	res, err := c.statusService.FindStatus(reqCtx, id)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusOK)
}

func (c *StatusController) FindByCode(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	code := ctx.Param("code")

	res, err := c.statusService.FindByCode(reqCtx, code)
	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Successfully", http.StatusOK)
}

func (c *StatusController) CreateStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	// 1. Получаем текстовые данные. Вариант для работы и с JSON, и с form-data
	var dto dto.CreateStatusDTO
	dataString := ctx.FormValue("data")
	if dataString != "" {
		if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
			return utils.ErrorResponse(ctx, fmt.Errorf("некорректный JSON в 'data': %w", err))
		}
	} else {
		if err := ctx.Bind(&dto); err != nil {
			return utils.ErrorResponse(ctx, fmt.Errorf("не удалось прочитать данные: %w", err))
		}
	}

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	// 2. Получаем заголовки файлов. Если их нет - это ошибка.
	iconSmallHeader, err := ctx.FormFile("icon_small")
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("файл 'icon_small' обязателен: %w", err))
	}
	iconBigHeader, err := ctx.FormFile("icon_big")
	if err != nil {
		return utils.ErrorResponse(ctx, fmt.Errorf("файл 'icon_big' обязателен: %w", err))
	}

	// 3. Открываем файлы. `defer` гарантирует, что они закроются.
	smallIconFile, err := iconSmallHeader.Open()
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrInternalServer)
	}
	defer smallIconFile.Close()

	bigIconFile, err := iconBigHeader.Open()
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.ErrInternalServer)
	}
	defer bigIconFile.Close()
	createdStatus, err := c.statusService.CreateStatus(
		reqCtx,
		dto,
		iconSmallHeader,
		iconBigHeader,
		smallIconFile,
		bigIconFile,
	)
	if err != nil {

		if errors.Is(err, apperrors.ErrConflict) {
			c.logger.Warn(
				"Попытка создания статуса с уже существующим кодом",
				zap.String("code", dto.Code),
				zap.String("name", dto.Name),
				zap.Error(err),
			)
		} else {

			c.logger.Error(
				"Не удалось создать статус",
				zap.Error(err),
			)
		}

		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, createdStatus, "Статус успешно создан", http.StatusCreated)
}

func (c *StatusController) UpdateStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный ID", err))
	}

	var dto dto.UpdateStatusDTO
	dataString := ctx.FormValue("data")
	if dataString != "" {
		if err := json.Unmarshal([]byte(dataString), &dto); err != nil {
			return utils.ErrorResponse(ctx, fmt.Errorf("некорректный JSON в 'data'"))
		}
	}
	dto.ID = id

	if err := ctx.Validate(&dto); err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	iconSmallHeader, err := ctx.FormFile("icon_small")
	if err != nil && err != http.ErrMissingFile {
		return utils.ErrorResponse(ctx, err)
	}
	iconBigHeader, err := ctx.FormFile("icon_big")
	if err != nil && err != http.ErrMissingFile {
		return utils.ErrorResponse(ctx, err)
	}

	updatedStatus, err := c.statusService.UpdateStatus(reqCtx, id, dto, iconSmallHeader, iconBigHeader)

	if err != nil {
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, updatedStatus, "Статус успешно обновлен", http.StatusOK)
}

func (c *StatusController) DeleteStatus(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Некорректный ID", err))
	}

	err = c.statusService.DeleteStatus(reqCtx, id)
	if err != nil {
		var httpErr *apperrors.HttpError
		if errors.As(err, &httpErr) {
			return utils.ErrorResponse(ctx, httpErr)
		}
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, nil, "Статус успешно удален", http.StatusOK)
}

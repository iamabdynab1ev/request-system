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

type EquipmentController struct {
	equipmentService services.EquipmentServiceInterface // Работаем с интерфейсом
	logger           *zap.Logger
}

func NewEquipmentController(
	service services.EquipmentServiceInterface, // Принимаем интерфейс
	logger *zap.Logger,
) *EquipmentController {
	return &EquipmentController{
		equipmentService: service,
		logger:           logger,
	}
}

// ----- РАБОЧИЕ МЕТОДЫ КОНТРОЛЛЕРА -----

func (c *EquipmentController) GetEquipments(ctx echo.Context) error {
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	res, total, err := c.equipmentService.GetEquipments(ctx.Request().Context(), filter)
	if err != nil {
		c.logger.Error("Ошибка при получении списка оборудования", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Оборудование успешно получено", http.StatusOK, total)
}

func (c *EquipmentController) FindEquipment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID"))
	}

	res, err := c.equipmentService.FindEquipment(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("Ошибка при поиске оборудования", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Оборудование успешно найдено", http.StatusOK)
}

func (c *EquipmentController) CreateEquipment(ctx echo.Context) error {
	var dto dto.CreateEquipmentDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("Ошибка привязки данных для создания оборудования", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных в теле запроса"))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка валидации данных для создания оборудования", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.equipmentService.CreateEquipment(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("Ошибка при создании оборудования", zap.Any("payload", dto), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Оборудование успешно создано", http.StatusCreated)
}

func (c *EquipmentController) UpdateEquipment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID"))
	}

	var dto dto.UpdateEquipmentDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("Ошибка привязки данных для обновления оборудования", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат данных в теле запроса"))
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("Ошибка валидации данных для обновления оборудования", zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	res, err := c.equipmentService.UpdateEquipment(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("Ошибка при обновлении оборудования", zap.Uint64("id", id), zap.Any("payload", dto), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, res, "Оборудование успешно обновлено", http.StatusOK)
}

func (c *EquipmentController) DeleteEquipment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный формат ID"))
	}

	err = c.equipmentService.DeleteEquipment(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("Ошибка при удалении оборудования", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(ctx, err)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Оборудование успешно удалено", http.StatusOK)
}

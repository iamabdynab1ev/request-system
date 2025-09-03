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
		c.logger.Error("GetEquipments: ошибка при получении списка оборудования", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось получить список оборудования",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, res, "Список оборудования успешно получен", http.StatusOK, total)
}

func (c *EquipmentController) FindEquipment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("FindEquipment: некорректный ID оборудования", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID оборудования",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	res, err := c.equipmentService.FindEquipment(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("FindEquipment: ошибка при поиске оборудования", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось найти оборудование",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, res, "Оборудование успешно найдено", http.StatusOK)
}

func (c *EquipmentController) CreateEquipment(ctx echo.Context) error {
	var dto dto.CreateEquipmentDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("CreateEquipment: ошибка привязки данных", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат данных в теле запроса",
				err,
				nil,
			),
			c.logger,
		)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("CreateEquipment: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.equipmentService.CreateEquipment(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("CreateEquipment: ошибка при создании оборудования", zap.Any("payload", dto), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось создать оборудование",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, res, "Оборудование успешно создано", http.StatusCreated)
}

func (c *EquipmentController) UpdateEquipment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("UpdateEquipment: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID оборудования",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	var dto dto.UpdateEquipmentDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("UpdateEquipment: ошибка привязки данных", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат данных в теле запроса",
				err,
				nil,
			),
			c.logger,
		)
	}

	if err := ctx.Validate(&dto); err != nil {
		c.logger.Error("UpdateEquipment: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.equipmentService.UpdateEquipment(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("UpdateEquipment: ошибка при обновлении оборудования", zap.Uint64("id", id), zap.Any("payload", dto), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось обновить оборудование",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, res, "Оборудование успешно обновлено", http.StatusOK)
}

func (c *EquipmentController) DeleteEquipment(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("DeleteEquipment: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID оборудования",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	if err := c.equipmentService.DeleteEquipment(ctx.Request().Context(), id); err != nil {
		c.logger.Error("DeleteEquipment: ошибка при удалении оборудования", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось удалить оборудование",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, struct{}{}, "Оборудование успешно удалено", http.StatusOK)
}

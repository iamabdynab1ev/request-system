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

type EquipmentTypeController struct {
	equipmentTypeService services.EquipmentTypeServiceInterface
	logger               *zap.Logger
}

func NewEquipmentTypeController(service services.EquipmentTypeServiceInterface, logger *zap.Logger) *EquipmentTypeController {
	return &EquipmentTypeController{equipmentTypeService: service, logger: logger}
}

func (c *EquipmentTypeController) GetEquipmentTypes(ctx echo.Context) error {
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())
	res, total, err := c.equipmentTypeService.GetEquipmentTypes(ctx.Request().Context(), filter)
	if err != nil {
		c.logger.Error("Ошибка получения списка типов оборудования", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось получить список типов оборудования",
				err,
				nil,
			),
			c.logger,
		)
	}
	return utils.SuccessResponse(ctx, res, "Успешно", http.StatusOK, total)
}

func (c *EquipmentTypeController) FindEquipmentType(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("FindEquipmentType: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID типа оборудования",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	res, err := c.equipmentTypeService.FindEquipmentType(ctx.Request().Context(), id)
	if err != nil {
		c.logger.Error("Ошибка поиска типа оборудования", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось найти тип оборудования",
				err,
				nil,
			),
			c.logger,
		)
	}

	return utils.SuccessResponse(ctx, res, "Тип оборудования успешно найден", http.StatusOK)
}

func (c *EquipmentTypeController) CreateEquipmentType(ctx echo.Context) error {
	var dto dto.CreateEquipmentTypeDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("CreateEquipmentType: ошибка привязки данных", zap.Error(err))
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
		c.logger.Error("CreateEquipmentType: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.equipmentTypeService.CreateEquipmentType(ctx.Request().Context(), dto)
	if err != nil {
		c.logger.Error("Ошибка создания типа оборудования", zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось создать тип оборудования",
				err,
				nil,
			),
			c.logger,
		)
	}
	return utils.SuccessResponse(ctx, res, "Успешно создан", http.StatusCreated)
}

func (c *EquipmentTypeController) UpdateEquipmentType(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("UpdateEquipmentType: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID типа оборудования",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}
	var dto dto.UpdateEquipmentTypeDTO
	if err := ctx.Bind(&dto); err != nil {
		c.logger.Error("UpdateEquipmentType: ошибка привязки данных", zap.Error(err))
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
		c.logger.Error("UpdateEquipmentType: ошибка валидации данных", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	res, err := c.equipmentTypeService.UpdateEquipmentType(ctx.Request().Context(), id, dto)
	if err != nil {
		c.logger.Error("Ошибка обновления типа оборудования", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось обновить тип оборудования",
				err,
				nil,
			),
			c.logger,
		)
	}
	return utils.SuccessResponse(ctx, res, "Успешно обновлен", http.StatusOK)
}

func (c *EquipmentTypeController) DeleteEquipmentType(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		c.logger.Error("DeleteEquipmentType: неверный формат ID", zap.String("id", ctx.Param("id")), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusBadRequest,
				"Неверный формат ID типа оборудования",
				err,
				map[string]interface{}{"param": ctx.Param("id")},
			),
			c.logger,
		)
	}

	if err := c.equipmentTypeService.DeleteEquipmentType(ctx.Request().Context(), id); err != nil {
		c.logger.Error("Ошибка удаления типа оборудования", zap.Uint64("id", id), zap.Error(err))
		return utils.ErrorResponse(
			ctx,
			apperrors.NewHttpError(
				http.StatusInternalServerError,
				"Не удалось удалить тип оборудования",
				err,
				nil,
			),
			c.logger,
		)
	}
	return utils.SuccessResponse(ctx, nil, "Успешно удален", http.StatusOK)
}

// Файл: internal/controllers/order_type.go

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

// OrderTypeController инкапсулирует логику обработки HTTP-запросов для типов заявок.
type OrderTypeController struct {
	service services.OrderTypeServiceInterface
	logger  *zap.Logger
}

// NewOrderTypeController создает новый экземпляр контроллера.
func NewOrderTypeController(service services.OrderTypeServiceInterface, logger *zap.Logger) *OrderTypeController {
	return &OrderTypeController{service: service, logger: logger}
}

// Create обрабатывает запрос на создание нового типа заявки (POST /order-types).
func (c *OrderTypeController) Create(ctx echo.Context) error {
	var createDTO dto.CreateOrderTypeDTO
	// ctx.Bind() автоматически распарсит JSON из тела запроса в нашу структуру DTO.
	if err := ctx.Bind(&createDTO); err != nil {
		c.logger.Warn("Ошибка привязки данных для создания типа заявки", zap.Error(err))
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверные данные в теле запроса", err, nil), c.logger)
	}

	// ctx.Validate() вызовет наш кастомный валидатор для проверки полей DTO (e.g., "required").
	if err := ctx.Validate(&createDTO); err != nil {
		c.logger.Warn("Ошибка валидации данных для создания типа заявки", zap.Error(err))
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	// Вызываем сервис для выполнения основной бизнес-логики.
	result, err := c.service.Create(ctx.Request().Context(), createDTO)
	if err != nil {
		// Ошибка уже залогирована внутри сервиса, просто передаем ее дальше.
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, result, "Тип заявки успешно создан", http.StatusCreated)
}

// Update обрабатывает запрос на обновление типа заявки (PUT /order-types/:id).
func (c *OrderTypeController) Update(ctx echo.Context) error {
	// Считываем ID из параметра URL.
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID в URL", err, nil), c.logger)
	}

	var updateDTO dto.UpdateOrderTypeDTO
	if err := ctx.Bind(&updateDTO); err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверные данные в теле запроса", err, nil), c.logger)
	}
	if err := ctx.Validate(&updateDTO); err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	result, err := c.service.Update(ctx.Request().Context(), id, updateDTO)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, result, "Тип заявки успешно обновлен", http.StatusOK)
}

// Delete обрабатывает запрос на удаление типа заявки (DELETE /order-types/:id).
func (c *OrderTypeController) Delete(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID в URL", err, nil), c.logger)
	}

	err = c.service.Delete(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	// При успешном удалении тело ответа обычно пустое.
	return utils.SuccessResponse(ctx, struct{}{}, "Тип заявки успешно удален", http.StatusOK)
}

// GetByID обрабатывает запрос на получение одного типа заявки по ID (GET /order-types/:id).
func (c *OrderTypeController) GetByID(ctx echo.Context) error {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID в URL", err, nil), c.logger)
	}

	result, err := c.service.GetByID(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, result, "Тип заявки успешно найден", http.StatusOK)
}

// GetAll обрабатывает запрос на получение списка типов заявок (GET /order-types).
func (c *OrderTypeController) GetAll(ctx echo.Context) error {
	// Парсим параметры для пагинации и поиска (?limit=10&offset=0&search=...) из URL.
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	result, err := c.service.GetAll(ctx.Request().Context(), uint64(filter.Limit), uint64(filter.Offset), filter.Search)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, result.List, "Список типов заявок успешно получен", http.StatusOK, result.Pagination.TotalCount)
}

func (c *OrderTypeController) GetConfig(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return utils.ErrorResponse(ctx, apperrors.NewHttpError(http.StatusBadRequest, "Неверный ID", err, nil), c.logger)
	}

	result, err := c.service.GetConfig(ctx.Request().Context(), id)
	if err != nil {
		return utils.ErrorResponse(ctx, err, c.logger)
	}

	return utils.SuccessResponse(ctx, result, "Конфигурация получена", http.StatusOK)
}

package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"request-system/internal/dto"
	"request-system/internal/services"
	"request-system/pkg/api" // Наш новый пакет
	apperrors "request-system/pkg/errors"
	"request-system/pkg/utils" // Для ParseFilter
)

type OrderController struct {
	orderService services.OrderServiceInterface
	logger       *zap.Logger
}

func NewOrderController(service services.OrderServiceInterface, logger *zap.Logger) *OrderController {
	return &OrderController{
		orderService: service,
		logger:       logger,
	}
}

// GetOrders - Получение списка
func (c *OrderController) GetOrders(ctx echo.Context) error {
	reqCtx := ctx.Request().Context()

	// 1. Используем старую утилиту для парсинга, или можно перейти на новую, если она есть.
	// Оставим твою utils.ParseFilterFromQuery, она делает правильную Filter структуру
	filter := utils.ParseFilterFromQuery(ctx.Request().URL.Query())

	onlyParticipant := ctx.QueryParam("participant") == "me"

	result, err := c.orderService.GetOrders(reqCtx, filter, onlyParticipant)
	if err != nil {
		c.logger.Error("GetOrders failed", zap.Error(err))
		return api.ErrorResponse(ctx, err) // Предположим, у тебя есть обертка или используем apperrors
	}

	// 2. Используем наш универсальный SuccessList из Этапа 1
	return api.SuccessList(ctx, "Список заявок успешно получен", result.List, result.TotalCount, filter.Page, filter.Limit)
}

// FindOrder - Получение одной заявки
func (c *OrderController) FindOrder(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return api.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный ID"))
	}

	order, err := c.orderService.FindOrderByID(ctx.Request().Context(), id)
	if err != nil {
		return api.ErrorResponse(ctx, err)
	}

	return api.SuccessOne(ctx, http.StatusOK, "Заявка найдена", order)
}

// CreateOrder - Создание
func (c *OrderController) CreateOrder(ctx echo.Context) error {
	dataStr := ctx.FormValue("data")
	if dataStr == "" {
		return api.ErrorResponse(ctx, apperrors.NewBadRequestError("Нет данных (field 'data')"))
	}

	var d dto.CreateOrderDTO
	if err := json.Unmarshal([]byte(dataStr), &d); err != nil {
		return api.ErrorResponse(ctx, apperrors.NewBadRequestError("Некорректный JSON"))
	}

	if err := ctx.Validate(&d); err != nil {
		return api.ErrorResponse(ctx, apperrors.NewBadRequestError(err.Error()))
	}

	// Получаем файл
	file, err := ctx.FormFile("file")
	if err != nil && err == http.ErrMissingFile {
		file, _ = ctx.FormFile("comment_attachment")
	}

	res, err := c.orderService.CreateOrder(ctx.Request().Context(), d, file)
	if err != nil {
		return api.ErrorResponse(ctx, err)
	}

	return api.SuccessOne(ctx, http.StatusCreated, "Заявка создана", res)
}

// UpdateOrder - Обновление
func (c *OrderController) UpdateOrder(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return api.ErrorResponse(ctx, apperrors.NewBadRequestError("Неверный ID"))
	}

	var d dto.UpdateOrderDTO

	// Поддержка и Multipart (с файлом), и JSON Body
	if ctx.Request().Header.Get("Content-Type") == "application/json" {
		if err := ctx.Bind(&d); err != nil {
			return api.ErrorResponse(ctx, apperrors.NewBadRequestError("Invalid JSON body"))
		}
	} else {
		dataStr := ctx.FormValue("data")
		if dataStr != "" {
			if err := json.Unmarshal([]byte(dataStr), &d); err != nil {
				return api.ErrorResponse(ctx, apperrors.NewBadRequestError("Invalid JSON in 'data'"))
			}
		}
	}

	if err := ctx.Validate(&d); err != nil {
		return api.ErrorResponse(ctx, apperrors.NewBadRequestError(err.Error()))
	}

	file, err := ctx.FormFile("file")
	if err != nil && err == http.ErrMissingFile {
		file, _ = ctx.FormFile("comment_attachment")
	}

	res, err := c.orderService.UpdateOrder(ctx.Request().Context(), id, d, file)
	if err != nil {
		return api.ErrorResponse(ctx, err)
	}

	return api.SuccessOne(ctx, http.StatusOK, "Заявка обновлена", res)
}

// DeleteOrder - Удаление
func (c *OrderController) DeleteOrder(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		return api.ErrorResponse(ctx, apperrors.NewBadRequestError("Invalid ID"))
	}

	if err := c.orderService.DeleteOrder(ctx.Request().Context(), id); err != nil {
		return api.ErrorResponse(ctx, err)
	}

	return api.SuccessOne[any](ctx, http.StatusOK, "Заявка удалена", nil)
}
